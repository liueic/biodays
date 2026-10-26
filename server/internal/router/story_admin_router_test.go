package router_test

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"biodays/server/internal/config"
	"biodays/server/internal/router"
	"biodays/server/internal/service"
	"biodays/server/internal/store"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

const storyAPIToken = "story-admin-token-0123456789abcdef"

type storyAdminHarness struct {
	engine *gin.Engine
	story  *store.Store
}

func newStoryAdminHarness(t *testing.T, apiToken string) *storyAdminHarness {
	t.Helper()
	gin.SetMode(gin.TestMode)
	name := strings.ReplaceAll(t.Name(), "/", "_")
	storyStore, err := store.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", name))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, storyStore.Close()) })

	feedbackStore, err := store.OpenFeedback(filepath.Join(t.TempDir(), "feedback.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, feedbackStore.Close()) })

	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	secret := []byte("0123456789abcdef0123456789abcdef")
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-horse-123"), bcrypt.MinCost)
	require.NoError(t, err)
	adminService := service.NewAdminService(feedbackStore, service.AdminSettings{
		Username: "editor", PasswordHash: string(hash), SessionTTL: 8 * time.Hour, Secret: secret,
	}, func() time.Time { return now })

	cfg := config.Config{
		Port: 8080, DBPath: ":memory:", CORSOrigins: []string{adminOrigin},
		RateLimit: 10000, LogLevel: "error", SiteURL: adminOrigin,
		AdminAPIToken: apiToken,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	engine, err := router.NewWithDeps(cfg, storyStore, logger, func() time.Time { return now }, router.Deps{
		AdminService: adminService,
	})
	require.NoError(t, err)
	return &storyAdminHarness{engine: engine, story: storyStore}
}

func (h *storyAdminHarness) request(t *testing.T, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()
	h.engine.ServeHTTP(rec, req)
	return rec
}

func (h *storyAdminHarness) login(t *testing.T) (cookie, csrf string) {
	t.Helper()
	rec := h.request(t, http.MethodPost, "/api/v1/admin/login",
		`{"username":"editor","password":"correct-horse-123"}`,
		map[string]string{"Origin": adminOrigin})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var response struct {
		Data struct {
			CSRFToken string `json:"csrf_token"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	return strings.Split(rec.Header().Get("Set-Cookie"), ";")[0], response.Data.CSRFToken
}

var storyBearer = map[string]string{"Authorization": "Bearer " + storyAPIToken}

func validStoryJSON(id string) string {
	return fmt.Sprintf(`{"id":%q,"month":1,"day":5,"year":1961,"title":"移液器","content":"长正文","geek_quote":"quote","tags":["仪器极客"],"summary":"摘要","lab_item":"移液枪","source":{"type":"patent","citation":"DE 1090449","url":"https://example.com/1"}}`, id)
}

func TestStoryAdminBearerFlow(t *testing.T) {
	h := newStoryAdminHarness(t, storyAPIToken)

	// Publish without Origin or CSRF headers: the token is the credential.
	rec := h.request(t, http.MethodPost, "/api/v1/admin/stories", validStoryJSON("T-01"), storyBearer)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"created":true`)

	// Same id again is an update, not a create.
	rec = h.request(t, http.MethodPost, "/api/v1/admin/stories", validStoryJSON("T-01"), storyBearer)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"created":false`)

	rec = h.request(t, http.MethodGet, "/api/v1/admin/stories?q=移液", "", storyBearer)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "T-01")

	rec = h.request(t, http.MethodGet, "/api/v1/admin/stories/T-01", "", storyBearer)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "长正文")

	rec = h.request(t, http.MethodPatch, "/api/v1/admin/stories/T-01", `{"title":"新标题"}`, storyBearer)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "新标题")

	rec = h.request(t, http.MethodDelete, "/api/v1/admin/stories/T-01", "", storyBearer)
	require.Equal(t, http.StatusNoContent, rec.Code)

	rec = h.request(t, http.MethodGet, "/api/v1/admin/stories/T-01", "", storyBearer)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestStoryAdminBatchUpsert(t *testing.T) {
	h := newStoryAdminHarness(t, storyAPIToken)
	body := fmt.Sprintf(`{"stories":[%s,%s]}`,
		validStoryJSON("B-01"),
		strings.Replace(validStoryJSON("B-02"), `"month":1`, `"month":13`, 1))
	rec := h.request(t, http.MethodPost, "/api/v1/admin/stories", body, storyBearer)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var response struct {
		Data struct {
			Accepted int `json:"accepted"`
			Rejected int `json:"rejected"`
			Items    []struct {
				ID       string   `json:"id"`
				Problems []string `json:"problems"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	require.Equal(t, 1, response.Data.Accepted)
	require.Equal(t, 1, response.Data.Rejected)
	require.NotEmpty(t, response.Data.Items[1].Problems)
}

func TestStoryAdminValidationAndLimits(t *testing.T) {
	h := newStoryAdminHarness(t, storyAPIToken)

	invalid := strings.Replace(validStoryJSON("V-01"), `"content":"长正文"`, `"content":" "`, 1)
	rec := h.request(t, http.MethodPost, "/api/v1/admin/stories", invalid, storyBearer)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "content is empty")

	rec = h.request(t, http.MethodPost, "/api/v1/admin/stories", `not json`, storyBearer)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	oversize := fmt.Sprintf(`{"stories":[%s]}`, strings.Join(repeatStoryJSON("X-", 201), ","))
	rec = h.request(t, http.MethodPost, "/api/v1/admin/stories", oversize, storyBearer)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "batch limit")

	rec = h.request(t, http.MethodPost, "/api/v1/admin/stories", validStoryJSON("C-01"),
		map[string]string{"Authorization": "Bearer " + storyAPIToken, "Content-Type": "text/plain"})
	require.Equal(t, http.StatusUnsupportedMediaType, rec.Code)
}

func repeatStoryJSON(prefix string, count int) []string {
	out := make([]string, 0, count)
	for i := range count {
		out = append(out, validStoryJSON(fmt.Sprintf("%s%03d", prefix, i)))
	}
	return out
}

func TestStoryAdminTokenRejection(t *testing.T) {
	h := newStoryAdminHarness(t, storyAPIToken)

	rec := h.request(t, http.MethodGet, "/api/v1/admin/stories", "",
		map[string]string{"Authorization": "Bearer wrong-token-value"})
	require.Equal(t, http.StatusUnauthorized, rec.Code)

	// A Bearer header must not fall through to the (absent) cookie session.
	rec = h.request(t, http.MethodPost, "/api/v1/admin/stories", validStoryJSON("R-01"),
		map[string]string{"Authorization": "Bearer wrong-token-value"})
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestStoryAdminWithoutConfiguredToken(t *testing.T) {
	h := newStoryAdminHarness(t, "")
	rec := h.request(t, http.MethodGet, "/api/v1/admin/stories", "",
		map[string]string{"Authorization": "Bearer " + storyAPIToken})
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestStoryAdminSessionFlow(t *testing.T) {
	h := newStoryAdminHarness(t, storyAPIToken)
	cookie, csrf := h.login(t)

	rec := h.request(t, http.MethodGet, "/api/v1/admin/stories", "", map[string]string{"Cookie": cookie})
	require.Equal(t, http.StatusOK, rec.Code)

	// Session writes need CSRF and Origin, like the rest of the admin surface.
	rec = h.request(t, http.MethodPost, "/api/v1/admin/stories", validStoryJSON("S-01"),
		map[string]string{"Cookie": cookie, "Origin": adminOrigin})
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), "CSRF_INVALID")

	rec = h.request(t, http.MethodPost, "/api/v1/admin/stories", validStoryJSON("S-01"),
		map[string]string{"Cookie": cookie, "X-CSRF-Token": csrf})
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), "origin")

	rec = h.request(t, http.MethodPost, "/api/v1/admin/stories", validStoryJSON("S-01"),
		map[string]string{"Cookie": cookie, "X-CSRF-Token": csrf, "Origin": adminOrigin})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	// Anonymous access is rejected.
	rec = h.request(t, http.MethodGet, "/api/v1/admin/stories", "", nil)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}
