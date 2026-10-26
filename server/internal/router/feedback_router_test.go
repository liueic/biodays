package router_test

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"biodays/server/internal/antispam"
	"biodays/server/internal/config"
	"biodays/server/internal/mailer"
	"biodays/server/internal/model"
	"biodays/server/internal/router"
	"biodays/server/internal/service"
	"biodays/server/internal/store"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

const adminOrigin = "https://biodays.test"

type feedbackHarness struct {
	engine *gin.Engine
	store  *store.FeedbackStore
	now    time.Time
	secret []byte
}

func newFeedbackHarness(t *testing.T) *feedbackHarness {
	return newFeedbackHarnessWithSettings(t, nil)
}

func newFeedbackHarnessWithSettings(t *testing.T, tweak func(*service.FeedbackSettings)) *feedbackHarness {
	t.Helper()
	gin.SetMode(gin.TestMode)
	name := strings.ReplaceAll(t.Name(), "/", "_")
	storyStore, err := store.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", name))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, storyStore.Close()) })

	feedbackStore, err := store.OpenFeedback(filepath.Join(t.TempDir(), "feedback.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, feedbackStore.Close()) })

	h := &feedbackHarness{store: feedbackStore, now: time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)}
	h.secret = []byte("0123456789abcdef0123456789abcdef")
	settings := service.FeedbackSettings{
		Enabled: true, SubscriptionsEnabled: true, SMTPEnabled: true,
		SiteURL: adminOrigin, NotifyTo: "admin@biodays.test",
		SubmitHourly: 1000, SubmitDaily: 1000, ResendHourly: 1000, ResendDaily: 1000,
	}
	if tweak != nil {
		tweak(&settings)
	}
	feedbackService := service.NewFeedbackService(feedbackStore, settings, h.secret, func() time.Time { return h.now })
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-horse-123"), bcrypt.MinCost)
	require.NoError(t, err)
	adminService := service.NewAdminService(feedbackStore, service.AdminSettings{
		Username: "editor", PasswordHash: string(hash), SessionTTL: 8 * time.Hour, Secret: h.secret,
	}, func() time.Time { return h.now })

	cfg := config.Config{
		Port: 8080, DBPath: ":memory:", CORSOrigins: []string{adminOrigin},
		RateLimit: 10000, LogLevel: "error", SiteURL: adminOrigin,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	engine, err := router.NewWithDeps(cfg, storyStore, logger, func() time.Time { return h.now }, router.Deps{
		Feedback: feedbackStore, FeedbackService: feedbackService, AdminService: adminService,
	})
	require.NoError(t, err)
	h.engine = engine
	return h
}

func (h *feedbackHarness) request(t *testing.T, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
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

func (h *feedbackHarness) login(t *testing.T) (cookie, csrf string) {
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
	require.NotEmpty(t, response.Data.CSRFToken)
	setCookie := rec.Header().Get("Set-Cookie")
	require.Contains(t, setCookie, "HttpOnly")
	require.Contains(t, setCookie, "SameSite=Strict")
	require.Contains(t, setCookie, "Secure")
	return strings.Split(setCookie, ";")[0], response.Data.CSRFToken
}

func TestFeedbackHTTPContract(t *testing.T) {
	h := newFeedbackHarness(t)

	rec := h.request(t, http.MethodGet, "/api/v1/feedback/options", "", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	require.Contains(t, rec.Body.String(), `"enabled":true`)

	body := `{"request_id":"req-http-0001","kind":"topic","title":"想了解 GFP","content":"希望讲讲荧光蛋白怎样从水母研究走向实验室应用。"}`
	rec = h.request(t, http.MethodPost, "/api/v1/feedback", body, nil)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	var created struct {
		Data struct {
			ID                 string `json:"id"`
			Status             string `json:"status"`
			SubscriptionStatus string `json:"subscription_status"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	require.NotEmpty(t, created.Data.ID)
	require.Equal(t, "pending", created.Data.Status)
	require.Equal(t, "none", created.Data.SubscriptionStatus)

	rec = h.request(t, http.MethodPost, "/api/v1/feedback", body, nil)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = h.request(t, http.MethodPost, "/api/v1/feedback",
		`{"request_id":"req-http-0001","kind":"topic","title":"不同标题","content":"希望讲讲荧光蛋白怎样从水母研究走向实验室应用。"}`, nil)
	require.Equal(t, http.StatusConflict, rec.Code)
	require.Contains(t, rec.Body.String(), "IDEMPOTENCY_CONFLICT")

	rec = h.request(t, http.MethodPost, "/api/v1/feedback",
		`{"request_id":"req-http-0002","kind":"topic","title":"订阅无邮箱","content":"希望讲讲荧光蛋白怎样从水母研究走向实验室应用。","subscribe_updates":true}`, nil)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), `"email"`)

	oversize := fmt.Sprintf(`{"request_id":"req-http-0003","kind":"topic","title":"标题","content":"%s"}`, strings.Repeat("a", 40*1024))
	rec = h.request(t, http.MethodPost, "/api/v1/feedback", oversize, nil)
	require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	require.Contains(t, rec.Body.String(), "PAYLOAD_TOO_LARGE")

	rec = h.request(t, http.MethodGet, "/api/v1/meta", "", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Header().Get("Cache-Control"), "public")
}

func TestAdminHTTPAuthCSRFAndInbox(t *testing.T) {
	h := newFeedbackHarness(t)
	rec := h.request(t, http.MethodGet, "/api/v1/admin/feedback", "", nil)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	require.Equal(t, "no-referrer", rec.Header().Get("Referrer-Policy"))

	rec = h.request(t, http.MethodPost, "/api/v1/admin/login",
		`{"username":"editor","password":"correct-horse-123"}`, nil)
	require.Equal(t, http.StatusForbidden, rec.Code)
	rec = h.request(t, http.MethodPost, "/api/v1/admin/login",
		`{"username":"editor","password":"correct-horse-123"}`, map[string]string{"Origin": "https://evil.example"})
	require.Equal(t, http.StatusForbidden, rec.Code)
	rec = h.request(t, http.MethodPost, "/api/v1/admin/login",
		`{"username":"editor","password":"correct-horse-123"}`, map[string]string{"Origin": adminOrigin, "Content-Type": "text/plain"})
	require.Equal(t, http.StatusBadRequest, rec.Code)

	cookie, csrf := h.login(t)
	authHeaders := map[string]string{"Cookie": cookie}
	rec = h.request(t, http.MethodGet, "/api/v1/admin/me", "", authHeaders)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"csrf_token"`)
	require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))

	body := `{"request_id":"req-admin-0001","kind":"resource","title":"一份实验室手册","content":"推荐一份很棒的实验室安全手册，适合新同学阅读。"}`
	rec = h.request(t, http.MethodPost, "/api/v1/feedback", body, nil)
	require.Equal(t, http.StatusCreated, rec.Code)
	var created struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))

	rec = h.request(t, http.MethodGet, "/api/v1/admin/feedback?status=pending", "", authHeaders)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), created.Data.ID)
	require.NotContains(t, rec.Body.String(), "@example.com")

	rec = h.request(t, http.MethodGet, "/api/v1/admin/feedback/"+created.Data.ID, "", authHeaders)
	require.Equal(t, http.StatusOK, rec.Code)

	patch := fmt.Sprintf(`{"status":"accepted","version":1,"notify_subscriber":true}`)
	rec = h.request(t, http.MethodPatch, "/api/v1/admin/feedback/"+created.Data.ID, patch,
		map[string]string{"Cookie": cookie, "Origin": adminOrigin})
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), "CSRF_INVALID")

	withCSRF := map[string]string{"Cookie": cookie, "X-CSRF-Token": csrf, "Origin": adminOrigin}
	rec = h.request(t, http.MethodPatch, "/api/v1/admin/feedback/"+created.Data.ID, patch, withCSRF)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"notified":false`)
	require.Contains(t, rec.Body.String(), "没有已确认的订阅者")

	rec = h.request(t, http.MethodPatch, "/api/v1/admin/feedback/"+created.Data.ID, patch, withCSRF)
	require.Equal(t, http.StatusConflict, rec.Code)
	require.Contains(t, rec.Body.String(), "CONFLICT")

	rec = h.request(t, http.MethodDelete, "/api/v1/admin/feedback/"+created.Data.ID, "", withCSRF)
	require.Equal(t, http.StatusOK, rec.Code)
	rec = h.request(t, http.MethodGet, "/api/v1/admin/feedback/"+created.Data.ID, "", authHeaders)
	require.Equal(t, http.StatusNotFound, rec.Code)

	rec = h.request(t, http.MethodPost, "/api/v1/admin/logout", "", withCSRF)
	require.Equal(t, http.StatusOK, rec.Code)
	rec = h.request(t, http.MethodGet, "/api/v1/admin/me", "", authHeaders)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestSubscriptionHTTPFlow(t *testing.T) {
	h := newFeedbackHarness(t)
	body := `{"request_id":"req-sub-http-1","kind":"issue","title":"发现一处错别字","content":"故事里有一处年份写错了，希望核对一下原始来源。","email":"reader@example.com","subscribe_updates":true}`
	rec := h.request(t, http.MethodPost, "/api/v1/feedback", body, nil)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var created struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))

	raw := h.confirmToken(t, created.Data.ID)
	confirmBody := fmt.Sprintf(`{"token":"%s"}`, raw)
	rec = h.request(t, http.MethodPost, "/api/v1/feedback/subscriptions/confirm", confirmBody, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"subscription_status":"confirmed"`)

	rec = h.request(t, http.MethodPost, "/api/v1/feedback/subscriptions/confirm", confirmBody, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "already_confirmed")

	rec = h.request(t, http.MethodPost, "/api/v1/feedback/subscriptions/confirm", `{"token":"bogus"}`, nil)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "INVALID_TOKEN")

	rec = h.request(t, http.MethodPost, "/api/v1/feedback/subscriptions/resend",
		fmt.Sprintf(`{"feedback_id":"%s","email":"other@example.com"}`, created.Data.ID), nil)
	require.Equal(t, http.StatusAccepted, rec.Code)

	// Queue a status notification, then unsubscribe through the HTTP API.
	detail, err := h.store.GetFeedback(t.Context(), created.Data.ID)
	require.NoError(t, err)
	feedbackService := service.NewFeedbackService(h.store, service.FeedbackSettings{
		Enabled: true, SubscriptionsEnabled: true, SMTPEnabled: true,
		SiteURL: adminOrigin, NotifyTo: "admin@biodays.test",
	}, h.secret, func() time.Time { return h.now })
	_, err = feedbackService.UpdateFeedback(t.Context(), "editor", service.AdminUpdateInput{
		ID: detail.ID, Version: detail.Version, Status: model.FeedbackStatusAccepted, NotifySubscriber: true,
	})
	require.NoError(t, err)

	unsubRaw := h.unsubscribeToken(t, detail.ID)
	rec = h.request(t, http.MethodPost, "/api/v1/feedback/subscriptions/unsubscribe",
		fmt.Sprintf(`{"token":"%s"}`, unsubRaw), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"subscription_status":"unsubscribed"`)
}

func (h *feedbackHarness) confirmToken(t *testing.T, feedbackID string) string {
	t.Helper()
	tasks, err := h.store.ListOutboxByFeedback(t.Context(), feedbackID)
	require.NoError(t, err)
	for _, task := range tasks {
		if task.Type != model.MailTypeSubscriptionConfirm || task.Status != model.MailStatusPending {
			continue
		}
		var data mailer.TemplateData
		require.NoError(t, json.Unmarshal([]byte(task.Payload), &data))
		return fragmentToken(t, data.ConfirmURL)
	}
	t.Fatalf("no confirmation token for %s", feedbackID)
	return ""
}

func (h *feedbackHarness) unsubscribeToken(t *testing.T, feedbackID string) string {
	t.Helper()
	tasks, err := h.store.ListOutboxByFeedback(t.Context(), feedbackID)
	require.NoError(t, err)
	for _, task := range tasks {
		if task.Type != model.MailTypeUserStatusUpdate {
			continue
		}
		var data mailer.TemplateData
		require.NoError(t, json.Unmarshal([]byte(task.Payload), &data))
		return fragmentToken(t, data.UnsubscribeURL)
	}
	t.Fatalf("no unsubscribe token for %s", feedbackID)
	return ""
}

func fragmentToken(t *testing.T, raw string) string {
	t.Helper()
	index := strings.Index(raw, "#token=")
	require.GreaterOrEqual(t, index, 0, raw)
	return raw[index+len("#token="):]
}

func TestFeedbackDisabledContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	name := strings.ReplaceAll(t.Name(), "/", "_")
	storyStore, err := store.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", name))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, storyStore.Close()) })
	cfg := config.Config{Port: 8080, DBPath: ":memory:", CORSOrigins: []string{adminOrigin}, RateLimit: 100, LogLevel: "error", SiteURL: adminOrigin}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	engine, err := router.NewWithClock(cfg, storyStore, logger, time.Now)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/feedback/options", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"enabled":false`)

	rec = httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/feedback", strings.NewReader("{}")))
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.Contains(t, rec.Body.String(), "FEEDBACK_UNAVAILABLE")

	rec = httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/admin/me", nil))
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestFeedbackContentTypeAndOrigin(t *testing.T) {
	h := newFeedbackHarness(t)
	body := `{"request_id":"req-http-origin-1","kind":"topic","title":"想了解 GFP","content":"希望讲讲荧光蛋白怎样从水母研究走向实验室应用。"}`

	rec := h.request(t, http.MethodPost, "/api/v1/feedback", body, map[string]string{"Content-Type": "text/plain"})
	require.Equal(t, http.StatusUnsupportedMediaType, rec.Code)
	require.Contains(t, rec.Body.String(), "UNSUPPORTED_MEDIA_TYPE")

	rec = h.request(t, http.MethodPost, "/api/v1/feedback", body, map[string]string{"Origin": "https://evil.test"})
	require.Equal(t, http.StatusForbidden, rec.Code)

	rec = h.request(t, http.MethodPost, "/api/v1/feedback", body, map[string]string{"Origin": adminOrigin})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
}

func TestChallengeEndpointAndPowSubmission(t *testing.T) {
	h := newFeedbackHarnessWithSettings(t, func(settings *service.FeedbackSettings) {
		settings.Verification.PowEnabled = true
		settings.Verification.PowDifficulty = 2
		settings.Verification.PowMinSeconds = 1
	})

	rec := h.request(t, http.MethodPost, "/api/v1/feedback/challenge", "", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	var challenge struct {
		Data struct {
			ChallengeID string `json:"challenge_id"`
			Salt        string `json:"salt"`
			Difficulty  int    `json:"difficulty"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &challenge))
	require.NotEmpty(t, challenge.Data.ChallengeID)

	nonce := ""
	for i := 0; ; i++ {
		candidate := strconv.Itoa(i)
		if antispam.VerifyPoW(challenge.Data.Salt, candidate, challenge.Data.Difficulty) {
			nonce = candidate
			break
		}
	}
	h.now = h.now.Add(time.Second)
	body := fmt.Sprintf(`{"request_id":"req-pow-http-1","kind":"topic","title":"想了解 GFP","content":"希望讲讲荧光蛋白怎样从水母研究走向实验室应用。","challenge_id":%q,"challenge_nonce":%q}`, challenge.Data.ChallengeID, nonce)
	rec = h.request(t, http.MethodPost, "/api/v1/feedback", body, nil)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	rec = h.request(t, http.MethodPost, "/api/v1/feedback/challenge", "", nil)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestChallengeDisabled(t *testing.T) {
	h := newFeedbackHarness(t)
	rec := h.request(t, http.MethodPost, "/api/v1/feedback/challenge", "", nil)
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Contains(t, rec.Body.String(), "CHALLENGE_DISABLED")
}
