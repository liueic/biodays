package handler_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"biodays/server/internal/handler"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type pingerFunc func(context.Context) error

func (f pingerFunc) Ping(ctx context.Context) error {
	return f(ctx)
}

func newReadyContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/readyz", nil)
	return c, rec
}

func TestReadyHandlerOK(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	c, rec := newReadyContext()

	handler.NewReadyHandler(pingerFunc(func(context.Context) error { return nil }), logger).Ready(c)

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"status":"ok"}`, rec.Body.String())
	require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
}

func TestReadyHandlerUnavailable(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	c, rec := newReadyContext()

	handler.NewReadyHandler(pingerFunc(func(context.Context) error {
		return errors.New("database unavailable")
	}), logger).Ready(c)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.JSONEq(t, `{"status":"unavailable"}`, rec.Body.String())
	require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
}
