package handler

import (
	"log/slog"
	"net/http"

	"biodays/server/internal/model"

	"github.com/gin-gonic/gin"
)

const (
	CodeInvalidParam             = "INVALID_PARAM"
	CodeStoryNotFound            = "STORY_NOT_FOUND"
	CodeRateLimited              = "RATE_LIMITED"
	CodeInternalError            = "INTERNAL_ERROR"
	CodeUnauthenticated          = "UNAUTHENTICATED"
	CodeForbidden                = "FORBIDDEN"
	CodeCSRFInvalid              = "CSRF_INVALID"
	CodeInvalidToken             = "INVALID_TOKEN"
	CodeTokenExpired             = "TOKEN_EXPIRED"
	CodeFeedbackNotFound         = "FEEDBACK_NOT_FOUND"
	CodeConflict                 = "CONFLICT"
	CodeIdempotencyConflict      = "IDEMPOTENCY_CONFLICT"
	CodePayloadTooLarge          = "PAYLOAD_TOO_LARGE"
	CodeFeedbackUnavailable      = "FEEDBACK_UNAVAILABLE"
	CodeAdminNotInitialized      = "ADMIN_NOT_INITIALIZED"
	CodeSettingsRevisionConflict = "SETTINGS_REVISION_CONFLICT"
	CodeSettingsSecretUnwritable = "SETTINGS_SECRET_UNWRITABLE"
	CodeUnsupportedMediaType     = "UNSUPPORTED_MEDIA_TYPE"
	CodeTurnstileRequired        = "TURNSTILE_REQUIRED"
	CodeTurnstileFailed          = "TURNSTILE_FAILED"
	CodeVerificationUnavailable  = "VERIFICATION_UNAVAILABLE"
)

type ErrorBody struct {
	Code    string            `json:"code" example:"STORY_NOT_FOUND"`
	Message string            `json:"message" example:"story Q2-99 not found"`
	Fields  map[string]string `json:"fields,omitempty"`
}

type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

type TodayMeta struct {
	IsFallback bool `json:"is_fallback" example:"false"`
}

type TodayResponse struct {
	Data model.StoryDTO `json:"data"`
	Meta TodayMeta      `json:"meta"`
}

type ListMeta struct {
	Total    int64 `json:"total" example:"104"`
	Page     int   `json:"page" example:"1"`
	PageSize int   `json:"page_size" example:"20"`
}

type StoryListResponse struct {
	Data []model.StoryDTO `json:"data"`
	Meta ListMeta         `json:"meta"`
}

type StorySummaryListResponse struct {
	Data []model.StorySummaryDTO `json:"data"`
	Meta ListMeta                `json:"meta"`
}

type TagsResponse struct {
	Data []model.TagCount `json:"data"`
}

type MetaResponse struct {
	Data model.Meta `json:"data"`
}

type HealthResponse struct {
	Status string `json:"status" example:"ok"`
}

type HealthHandler struct{}

// Health godoc
// @Summary      健康检查
// @Description  返回服务存活状态，不参与限流。
// @Tags         system
// @Produce      json
// @Success      200  {object}  HealthResponse
// @Router       /healthz [get]
func (HealthHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, HealthResponse{Status: "ok"})
}

func fail(c *gin.Context, status int, code, message string) {
	c.Header("Cache-Control", "no-store")
	c.AbortWithStatusJSON(status, ErrorResponse{Error: ErrorBody{Code: code, Message: message}})
}

func failFields(c *gin.Context, status int, code, message string, fields map[string]string) {
	c.Header("Cache-Control", "no-store")
	c.AbortWithStatusJSON(status, ErrorResponse{Error: ErrorBody{Code: code, Message: message, Fields: fields}})
}

func internalError(c *gin.Context, logger *slog.Logger, err error) {
	logger.Error("request failed", "path", c.Request.URL.Path, "error", err)
	fail(c, http.StatusInternalServerError, CodeInternalError, "internal server error")
}
