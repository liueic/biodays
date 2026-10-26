package router_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"biodays/server/internal/config"
	"biodays/server/internal/model"
	"biodays/server/internal/router"
	"biodays/server/internal/store"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newTestEngine(t *testing.T, rateLimit int) *gin.Engine {
	t.Helper()
	return newTestEngineWithProxies(t, rateLimit, nil)
}

func newTestEngineWithProxies(t *testing.T, rateLimit int, trustedProxies []string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	name := strings.ReplaceAll(t.Name(), "/", "_")
	st, err := store.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", name))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, st.Close()) })

	ctx := context.Background()
	for _, story := range []struct {
		id          string
		month, day  int
		year        int
		title       string
		summary     string
		labItem     string
		content     string
		quote       string
		tags        []string
		sourceType  string
		citationURL string
	}{
		{"Q1-01", 1, 5, 1961, "移液器", "嘴吸时代终结", "移液枪", "content", "quote", []string{"仪器极客", "工业设计"}, "patent", "https://example.com/1"},
		{"Q4-01", 10, 9, 2002, "质谱诺奖", "软电离", "质谱仪", "content", "quote", []string{"仪器极客", "质谱与组学"}, "nobel", "https://example.com/2"},
		{"Q4-02", 10, 16, 1980, "液氮研钵", "RNase 冻结", "研钵", "content", "quote", []string{"样品前处理"}, "paper", "https://example.com/3"},
	} {
		require.NoError(t, st.Upsert(ctx, &model.Story{
			ID: story.id, Month: story.month, Day: story.day, Year: story.year,
			Title: story.title, Summary: story.summary, LabItem: story.labItem,
			Content: story.content, GeekQuote: story.quote,
			SourceType: story.sourceType, SourceCitation: "citation", SourceURL: story.citationURL,
		}, story.tags, false, false))
	}

	cfg := config.Config{
		Port:           8080,
		DBPath:         ":memory:",
		CORSOrigins:    []string{"http://localhost:3000"},
		TrustedProxies: trustedProxies,
		RateLimit:      rateLimit,
		LogLevel:       "error",
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	clock := func() time.Time { return time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC) }
	engine, err := router.NewWithClock(cfg, st, logger, clock)
	require.NoError(t, err)
	return engine
}

func do(t *testing.T, engine *gin.Engine, method, path string, header map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	return doFrom(t, engine, method, path, "", header)
}

func doFrom(t *testing.T, engine *gin.Engine, method, path, remoteAddr string, header map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	if remoteAddr != "" {
		req.RemoteAddr = remoteAddr
	}
	for key, value := range header {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var value T
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &value), rec.Body.String())
	return value
}

type todayEnvelope struct {
	Data model.StoryDTO `json:"data"`
	Meta struct {
		IsFallback bool `json:"is_fallback"`
	} `json:"meta"`
}

type listEnvelope struct {
	Data []model.StoryDTO `json:"data"`
	Meta struct {
		Total    int64 `json:"total"`
		Page     int   `json:"page"`
		PageSize int   `json:"page_size"`
	} `json:"meta"`
}

type errorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func TestHealth(t *testing.T) {
	engine := newTestEngine(t, 60)
	rec := do(t, engine, http.MethodGet, "/healthz", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"status":"ok"}`, rec.Body.String())
}

func TestReady(t *testing.T) {
	engine := newTestEngine(t, 60)
	rec := do(t, engine, http.MethodGet, "/readyz", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"status":"ok"}`, rec.Body.String())
	require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
}

func TestToday(t *testing.T) {
	engine := newTestEngine(t, 60)

	rec := do(t, engine, http.MethodGet, "/api/v1/today", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	body := decode[todayEnvelope](t, rec)
	require.Equal(t, "Q4-02", body.Data.ID)
	require.False(t, body.Meta.IsFallback)
	require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	require.Contains(t, strings.Join(rec.Header().Values("Vary"), ","), "Cookie")

	rec = do(t, engine, http.MethodGet, "/api/v1/today?tz=-13:00", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	body = decode[todayEnvelope](t, rec)
	require.True(t, body.Meta.IsFallback, "UTC-13 is still 2026-10-15")
	require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))

	rec = do(t, engine, http.MethodGet, "/api/v1/today?tz=-13:00&fallback=none", nil)
	require.Equal(t, http.StatusNotFound, rec.Code)
	errBody := decode[errorEnvelope](t, rec)
	require.Equal(t, "STORY_NOT_FOUND", errBody.Error.Code)
	require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))

	for _, path := range []string{"/api/v1/today?tz=8", "/api/v1/today?fallback=sometimes"} {
		rec = do(t, engine, http.MethodGet, path, nil)
		require.Equal(t, http.StatusBadRequest, rec.Code, path)
		require.Equal(t, "INVALID_PARAM", decode[errorEnvelope](t, rec).Error.Code)
	}
}

func TestTodayOnEmptyDatabaseIsBusiness404(t *testing.T) {
	st, err := store.Open("file:" + t.Name() + "?mode=memory&cache=shared")
	require.NoError(t, err)
	defer st.Close()
	engine, err := router.New(config.Config{
		CORSOrigins: []string{"http://localhost:3000"}, RateLimit: 60,
	}, st, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)
	rec := do(t, engine, http.MethodGet, "/api/v1/today", nil)
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Equal(t, "STORY_NOT_FOUND", decode[errorEnvelope](t, rec).Error.Code)
	require.Contains(t, rec.Header().Get("Content-Type"), "application/json")
}

func TestList(t *testing.T) {
	engine := newTestEngine(t, 60)

	rec := do(t, engine, http.MethodGet, "/api/v1/stories?page=1&page_size=2", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	body := decode[listEnvelope](t, rec)
	require.EqualValues(t, 3, body.Meta.Total)
	require.Len(t, body.Data, 2)
	require.Equal(t, 1, body.Meta.Page)
	require.NotEmpty(t, body.Data[0].Content)
	rec = do(t, engine, http.MethodGet, "/api/v1/stories?view=summary&page_size=2", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	summary := decode[struct {
		Data []map[string]any `json:"data"`
	}](t, rec)
	require.Len(t, summary.Data, 2)
	require.NotEmpty(t, summary.Data[0]["title"])
	require.NotEmpty(t, summary.Data[0]["tags"])
	for _, key := range []string{"content", "source", "geek_quote"} {
		require.NotContains(t, summary.Data[0], key)
	}

	rec = do(t, engine, http.MethodGet, "/api/v1/stories?tag=仪器极客", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	require.EqualValues(t, 2, decode[listEnvelope](t, rec).Meta.Total)

	rec = do(t, engine, http.MethodGet, "/api/v1/stories?q=RNase", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	body = decode[listEnvelope](t, rec)
	require.EqualValues(t, 1, body.Meta.Total)
	require.Equal(t, "Q4-02", body.Data[0].ID)

	rec = do(t, engine, http.MethodGet, "/api/v1/stories?month=10&day=16", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	require.EqualValues(t, 1, decode[listEnvelope](t, rec).Meta.Total)

	for _, path := range []string{
		"/api/v1/stories?month=13",
		"/api/v1/stories?day=5",
		"/api/v1/stories?page_size=101",
		"/api/v1/stories?page=0",
		"/api/v1/stories?view=unknown",
	} {
		rec = do(t, engine, http.MethodGet, path, nil)
		require.Equal(t, http.StatusBadRequest, rec.Code, path)
		require.Equal(t, "INVALID_PARAM", decode[errorEnvelope](t, rec).Error.Code)
	}
}

func TestGetAndRandom(t *testing.T) {
	engine := newTestEngine(t, 60)

	rec := do(t, engine, http.MethodGet, "/api/v1/stories/Q1-01", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	story := decode[model.StoryDTO](t, rec)
	require.Equal(t, "移液器", story.Title)
	require.Equal(t, "patent", story.Source.Type)
	require.Equal(t, []string{"仪器极客", "工业设计"}, story.Tags)

	rec = do(t, engine, http.MethodGet, "/api/v1/stories/Q2-99", nil)
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Equal(t, "STORY_NOT_FOUND", decode[errorEnvelope](t, rec).Error.Code)

	rec = do(t, engine, http.MethodGet, "/api/v1/stories/random?exclude=Q1-01,Q4-02", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "Q4-01", decode[model.StoryDTO](t, rec).ID)
	require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))

	rec = do(t, engine, http.MethodGet, "/api/v1/stories/random?exclude=Q1-01,Q4-01,Q4-02", nil)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestRandomCookieAvoidsRepeatsAcrossRounds(t *testing.T) {
	engine := newTestEngine(t, 1000)
	var cookie *http.Cookie
	current := ""
	for round := 0; round < 2; round++ {
		seen := map[string]bool{}
		for i := 0; i < 3; i++ {
			headers := map[string]string{}
			if cookie != nil {
				headers["Cookie"] = cookie.String()
			}
			path := "/api/v1/stories/random"
			if current != "" {
				path += "?exclude=" + current
			}
			rec := do(t, engine, http.MethodGet, path, headers)
			require.Equal(t, http.StatusOK, rec.Code)
			require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
			require.Contains(t, strings.Join(rec.Header().Values("Vary"), ","), "Cookie")
			story := decode[model.StoryDTO](t, rec)
			require.False(t, seen[story.ID], "must exhaust unread stories before repeating")
			require.NotEqual(t, current, story.ID, "new rounds still exclude the current story")
			seen[story.ID] = true
			current = story.ID
			cookies := rec.Result().Cookies()
			require.Len(t, cookies, 1)
			cookie = cookies[0]
		}
		require.Len(t, seen, 3)
	}
}

func TestViewedStoryCookieIsUsedByRandom(t *testing.T) {
	engine := newTestEngine(t, 1000)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/history", strings.NewReader(`{"id":"Q1-01"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	cookies := rec.Result().Cookies()
	require.Len(t, cookies, 1)
	rec = do(t, engine, http.MethodGet, "/api/v1/stories/random?exclude=Q4-02", map[string]string{"Cookie": cookies[0].String()})
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "Q4-01", decode[model.StoryDTO](t, rec).ID)
	// Another browser's cookie jar is independent.
	rec = do(t, engine, http.MethodGet, "/api/v1/stories/random?exclude=Q4-01,Q4-02", nil)
	require.Equal(t, "Q1-01", decode[model.StoryDTO](t, rec).ID)
	for _, test := range []struct {
		body   string
		status int
	}{
		{`{"id":"missing"}`, http.StatusNotFound}, {`{"id":""}`, http.StatusBadRequest}, {`{`, http.StatusBadRequest},
	} {
		req = httptest.NewRequest(http.MethodPost, "/api/v1/history", strings.NewReader(test.body))
		req.Header.Set("Content-Type", "application/json")
		rec = httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		require.Equal(t, test.status, rec.Code)
		require.Empty(t, rec.Result().Cookies())
	}
}

func TestOtherIDFormatsReachTodayArchiveAndRandom(t *testing.T) {
	st, err := store.Open("file:" + t.Name() + "?mode=memory&cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, st.Close()) })
	for _, entry := range []struct {
		id  string
		day int
	}{
		{"BIO-C4-01", 16}, {"article-42", 16}, {"Q4-01", 9},
	} {
		require.NoError(t, st.Upsert(context.Background(), &model.Story{
			ID: entry.id, Month: 10, Day: entry.day, Year: 2000,
			Title: entry.id, Content: "content", GeekQuote: "quote",
		}, nil, false, false))
	}
	engine, err := router.NewWithClock(config.Config{CORSOrigins: []string{"*"}, RateLimit: 1000}, st,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		func() time.Time { return time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC) })
	require.NoError(t, err)
	rec := do(t, engine, http.MethodGet, "/api/v1/stories", nil)
	require.EqualValues(t, 3, decode[listEnvelope](t, rec).Meta.Total)
	for _, id := range []string{"BIO-C4-01", "article-42"} {
		rec = do(t, engine, http.MethodGet, "/api/v1/stories/"+id, nil)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, id, decode[model.StoryDTO](t, rec).ID)
	}
	var cookie *http.Cookie
	for _, id := range []string{"BIO-C4-01", "article-42"} {
		if cookie != nil {
			rec = do(t, engine, http.MethodGet, "/api/v1/today", map[string]string{"Cookie": cookie.String()})
			require.Equal(t, "article-42", decode[todayEnvelope](t, rec).Data.ID)
			require.Empty(t, rec.Result().Cookies(), "selection and prefetch must not mark a story as read")
		}
		req := httptest.NewRequest(http.MethodPost, "/api/v1/history", strings.NewReader(`{"id":"`+id+`"}`))
		req.Header.Set("Content-Type", "application/json")
		if cookie != nil {
			req.AddCookie(cookie)
		}
		rec = httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		require.Equal(t, http.StatusNoContent, rec.Code)
		cookie = rec.Result().Cookies()[0]
	}
	rec = do(t, engine, http.MethodGet, "/api/v1/today", map[string]string{"Cookie": cookie.String()})
	require.False(t, decode[todayEnvelope](t, rec).Meta.IsFallback)
	rec = do(t, engine, http.MethodGet, "/api/v1/stories/random", map[string]string{"Cookie": cookie.String()})
	require.Equal(t, "Q4-01", decode[model.StoryDTO](t, rec).ID, "reading all today's stories must preserve global history")
}

func TestTagsAndMeta(t *testing.T) {
	engine := newTestEngine(t, 60)

	rec := do(t, engine, http.MethodGet, "/api/v1/tags", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	tags := decode[struct {
		Data []model.TagCount `json:"data"`
	}](t, rec)
	require.NotEmpty(t, tags.Data)
	require.Equal(t, "仪器极客", tags.Data[0].Name)
	require.EqualValues(t, 2, tags.Data[0].Count)

	rec = do(t, engine, http.MethodGet, "/api/v1/meta", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	meta := decode[struct {
		Data model.Meta `json:"data"`
	}](t, rec)
	require.EqualValues(t, 3, meta.Data.Total)
	require.Equal(t, []string{"Q1", "Q4"}, meta.Data.Quarters)
	require.Equal(t, "1月 / 10月", meta.Data.Coverage)
	require.Equal(t, map[int]int64{1: 1, 10: 2}, meta.Data.MonthCounts)
}

func TestRateLimitAndHealthExemption(t *testing.T) {
	engine := newTestEngine(t, 1)

	rec := do(t, engine, http.MethodGet, "/api/v1/meta", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "1", rec.Header().Get("X-RateLimit-Limit"))

	rec = do(t, engine, http.MethodGet, "/api/v1/meta", nil)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	require.Equal(t, "RATE_LIMITED", decode[errorEnvelope](t, rec).Error.Code)
	require.Equal(t, "60", rec.Header().Get("Retry-After"))

	rec = do(t, engine, http.MethodGet, "/healthz", nil)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = do(t, engine, http.MethodGet, "/readyz", nil)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestClientIPBehindProxy(t *testing.T) {
	for _, tc := range []struct {
		name    string
		proxies []string
		remote  string
		headers map[string]string
		want    string
	}{
		{"Caddy through Next", []string{"127.0.0.1", "::1"}, "127.0.0.1:1234", map[string]string{"X-Forwarded-For": "198.51.100.7"}, "198.51.100.7"},
		{"IPv6 client", []string{"::1"}, "[::1]:1234", map[string]string{"X-Forwarded-For": "2001:db8::7"}, "2001:db8::7"},
		{"real IP fallback", []string{"127.0.0.1"}, "127.0.0.1:1234", map[string]string{"X-Real-IP": "198.51.100.7"}, "198.51.100.7"},
		{"untrusted real IP ignored", nil, "192.0.2.10:1234", map[string]string{"X-Real-IP": "198.51.100.7"}, "192.0.2.10"},
		{"trusted chain parsed from right", []string{"127.0.0.1", "172.18.0.0/16"}, "127.0.0.1:1234", map[string]string{"X-Forwarded-For": "203.0.113.99, 198.51.100.7, 172.18.0.2"}, "198.51.100.7"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			engine := newTestEngineWithProxies(t, 60, tc.proxies)
			engine.GET("/client-ip", func(c *gin.Context) { c.String(http.StatusOK, c.ClientIP()) })
			rec := doFrom(t, engine, http.MethodGet, "/client-ip", tc.remote, tc.headers)
			require.Equal(t, tc.want, rec.Body.String())
		})
	}

	t.Run("forwarded-for ignored without trusted proxies", func(t *testing.T) {
		engine := newTestEngine(t, 1)
		headers := map[string]string{"X-Forwarded-For": "198.51.100.1"}
		rec := doFrom(t, engine, http.MethodGet, "/api/v1/meta", "192.0.2.10:1234", headers)
		require.Equal(t, http.StatusOK, rec.Code)

		headers["X-Forwarded-For"] = "198.51.100.2"
		rec = doFrom(t, engine, http.MethodGet, "/api/v1/meta", "192.0.2.10:1234", headers)
		require.Equal(t, http.StatusTooManyRequests, rec.Code)
	})

	t.Run("forwarded-for used from trusted proxy", func(t *testing.T) {
		engine := newTestEngineWithProxies(t, 1, []string{"127.0.0.1"})
		rec := doFrom(t, engine, http.MethodGet, "/api/v1/meta", "127.0.0.1:1234", map[string]string{
			"X-Forwarded-For": "198.51.100.1",
		})
		require.Equal(t, http.StatusOK, rec.Code)

		rec = doFrom(t, engine, http.MethodGet, "/api/v1/meta", "127.0.0.1:1234", map[string]string{
			"X-Forwarded-For": "198.51.100.1",
		})
		require.Equal(t, http.StatusTooManyRequests, rec.Code)

		rec = doFrom(t, engine, http.MethodGet, "/api/v1/meta", "127.0.0.1:1234", map[string]string{
			"X-Forwarded-For": "198.51.100.2",
		})
		require.Equal(t, http.StatusOK, rec.Code)
	})
}

func TestCors(t *testing.T) {
	engine := newTestEngine(t, 60)

	rec := do(t, engine, http.MethodGet, "/api/v1/meta", map[string]string{"Origin": "http://localhost:3000"})
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "http://localhost:3000", rec.Header().Get("Access-Control-Allow-Origin"))

	rec = do(t, engine, http.MethodGet, "/api/v1/meta", map[string]string{"Origin": "https://evil.example"})
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestSwaggerUI(t *testing.T) {
	engine := newTestEngine(t, 60)
	rec := do(t, engine, http.MethodGet, "/swagger/index.html", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "swagger")
}

func TestSwaggerWin98Theme(t *testing.T) {
	engine := newTestEngine(t, 60)
	rec := do(t, engine, http.MethodGet, "/swagger/index.css", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Header().Get("Content-Type"), "text/css")
	require.Contains(t, rec.Body.String(), "--w98-face")

	// The rest of the Swagger UI keeps working.
	rec = do(t, engine, http.MethodGet, "/swagger/doc.json", nil)
	require.Equal(t, http.StatusOK, rec.Code)
}
