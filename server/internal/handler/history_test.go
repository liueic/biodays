package handler

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestHistoryCookieFitsWholeDataset(t *testing.T) {
	ids := make([]string, 408)
	for i := range ids {
		ids[i] = fmt.Sprintf("BIO-C%d-%02d", i/50+1, i%50+1)
	}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/stories/random", nil)
	c.Request.Header.Set("X-Forwarded-Proto", "https")
	writeHistory(c, ids)
	cookies := rec.Result().Cookies()
	require.Len(t, cookies, 1)
	cookie := cookies[0]
	require.Less(t, len(cookie.String()), 4096)
	require.True(t, cookie.HttpOnly)
	require.True(t, cookie.Secure)
	require.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
	require.Equal(t, historyMaxAge, cookie.MaxAge)
	require.Equal(t, "/", cookie.Path)
	c.Request.AddCookie(cookie)
	require.Equal(t, ids, readHistory(c.Request))
	require.Equal(t, []string{"B", "A"}, appendHistory([]string{"A", "B"}, "A"))
}

func TestHistoryIgnoresMalformedOrOversizedCookie(t *testing.T) {
	encode := func(value any) string {
		raw, err := json.Marshal(value)
		require.NoError(t, err)
		var buffer bytes.Buffer
		w := zlib.NewWriter(&buffer)
		_, err = w.Write(raw)
		require.NoError(t, err)
		require.NoError(t, w.Close())
		return base64.RawURLEncoding.EncodeToString(buffer.Bytes())
	}
	for _, value := range []string{"broken!", encode([]string{""}), encode([]string{"abcdefghijklmnopq"}), encode(make([]string, 20_000))} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: historyCookie, Value: value})
		require.Empty(t, readHistory(req))
	}
}
