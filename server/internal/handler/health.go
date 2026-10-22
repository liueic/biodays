package handler

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type Pinger interface {
	Ping(context.Context) error
}

type ReadyHandler struct {
	pinger Pinger
	logger *slog.Logger
}

func NewReadyHandler(pinger Pinger, logger *slog.Logger) *ReadyHandler {
	return &ReadyHandler{pinger: pinger, logger: logger}
}

// Ready godoc
// @Summary      就绪检查
// @Description  检查数据库连通性；失败时返回 503，且不参与限流。
// @Tags         system
// @Produce      json
// @Success      200  {object}  HealthResponse
// @Failure      503  {object}  HealthResponse
// @Router       /readyz [get]
func (h *ReadyHandler) Ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	c.Header("Cache-Control", "no-store")
	if err := h.pinger.Ping(ctx); err != nil {
		h.logger.Error("readiness check failed", "error", err)
		c.JSON(http.StatusServiceUnavailable, HealthResponse{Status: "unavailable"})
		return
	}
	c.JSON(http.StatusOK, HealthResponse{Status: "ok"})
}
