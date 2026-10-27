package handler

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"

	"biodays/server/internal/model"
	"biodays/server/internal/service"

	"github.com/gin-gonic/gin"
)

const maxFeedbackBody = 32 << 10

type FeedbackHandler struct {
	service *service.FeedbackService
	logger  *slog.Logger
}

func NewFeedbackHandler(svc *service.FeedbackService, logger *slog.Logger) *FeedbackHandler {
	return &FeedbackHandler{service: svc, logger: logger}
}

type FeedbackOptionsResponse struct {
	Data model.FeedbackOptionsDTO `json:"data"`
}

type FeedbackReceiptResponse struct {
	Data model.SubmitReceiptDTO `json:"data"`
}

type SubscriptionResultDTO struct {
	SubscriptionStatus string `json:"subscription_status" example:"confirmed"`
	Result             string `json:"result" example:"confirmed"`
}

type SubscriptionResultResponse struct {
	Data SubscriptionResultDTO `json:"data"`
}

type AcceptedDTO struct {
	Accepted bool `json:"accepted" example:"true"`
}

type AcceptedResponse struct {
	Data AcceptedDTO `json:"data"`
}

// Options godoc
// @Summary      反馈表单能力
// @Description  返回反馈是否启用、是否支持邮件订阅、反馈类型和字段限制；不含任何 SMTP 凭据。
// @Tags         feedback
// @Produce      json
// @Success      200  {object}  FeedbackOptionsResponse
// @Router       /api/v1/feedback/options [get]
func (h *FeedbackHandler) Options(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusOK, FeedbackOptionsResponse{Data: model.FeedbackOptionsDTO{
			Enabled: false,
			Kinds:   []model.FeedbackKindOption{},
			Limits: model.FeedbackLimits{
				TitleMin: service.TitleMin, TitleMax: service.TitleMax,
				ContentMin: service.ContentMin, ContentMax: service.ContentMax,
				NicknameMax: service.NicknameMax, SourceURLMax: service.SourceURLMax,
			},
			Verification: model.VerificationOptionsDTO{},
		}})
		return
	}
	c.JSON(http.StatusOK, FeedbackOptionsResponse{Data: h.service.Options()})
}

type submitRequest struct {
	RequestID        string `json:"request_id"`
	Kind             string `json:"kind"`
	Title            string `json:"title"`
	Content          string `json:"content"`
	SourceURL        string `json:"source_url"`
	StoryID          string `json:"story_id"`
	Nickname         string `json:"nickname"`
	Email            string `json:"email"`
	SubscribeUpdates bool   `json:"subscribe_updates"`
	Website          string `json:"website"` // honeypot: must stay empty
	TurnstileToken   string `json:"turnstile_token"`
}

// Submit godoc
// @Summary      匿名提交反馈
// @Description  无需注册。首次写入返回 201；相同 request_id 与相同内容的重试返回 200 与原回执；相同 request_id 不同内容返回 409。
// @Tags         feedback
// @Accept       json
// @Produce      json
// @Param        body  body      submitRequest  true  "反馈内容"
// @Success      201   {object}  FeedbackReceiptResponse
// @Success      200   {object}  FeedbackReceiptResponse
// @Failure      400   {object}  ErrorResponse
// @Failure      409   {object}  ErrorResponse
// @Failure      413   {object}  ErrorResponse
// @Failure      429   {object}  ErrorResponse
// @Failure      503   {object}  ErrorResponse
// @Router       /api/v1/feedback [post]
func (h *FeedbackHandler) Submit(c *gin.Context) {
	if h.service == nil {
		fail(c, http.StatusServiceUnavailable, CodeFeedbackUnavailable, "feedback is not enabled")
		return
	}
	if !requireJSON(c) {
		return
	}
	var req submitRequest
	if !bindJSON(c, &req, maxFeedbackBody) {
		return
	}
	outcome, err := h.service.Submit(c.Request.Context(), service.SubmitInput{
		RequestID: req.RequestID, Kind: req.Kind, Title: req.Title, Content: req.Content,
		SourceURL: req.SourceURL, StoryID: req.StoryID, Nickname: req.Nickname,
		Email: req.Email, Subscribe: req.SubscribeUpdates, Honeypot: req.Website,
		TurnstileToken: req.TurnstileToken,
	}, c.ClientIP())
	if err != nil {
		h.writeError(c, err)
		return
	}
	status := http.StatusOK
	if outcome.Created {
		status = http.StatusCreated
	}
	c.JSON(status, FeedbackReceiptResponse{Data: model.SubmitReceiptDTO{
		ID: outcome.Feedback.ID, Status: outcome.Feedback.Status,
		SubscriptionStatus: outcome.Feedback.SubscriptionStatus,
		ConfirmationEmail:  outcome.ConfirmationEmail, CreatedAt: outcome.Feedback.CreatedAt,
	}})
}

type tokenRequest struct {
	Token string `json:"token"`
}

// ConfirmSubscription godoc
// @Summary      确认邮件订阅
// @Description  GET 不修改状态；本接口由页面按钮 POST 调用，单次使用，重复确认返回稳定结果。
// @Tags         feedback
// @Accept       json
// @Produce      json
// @Param        body  body      tokenRequest  true  "确认令牌"
// @Success      200   {object}  SubscriptionResultResponse
// @Failure      400   {object}  ErrorResponse
// @Failure      410   {object}  ErrorResponse
// @Router       /api/v1/feedback/subscriptions/confirm [post]
func (h *FeedbackHandler) ConfirmSubscription(c *gin.Context) {
	if h.service == nil {
		fail(c, http.StatusServiceUnavailable, CodeFeedbackUnavailable, "feedback is not enabled")
		return
	}
	var req tokenRequest
	if !bindJSON(c, &req, maxFeedbackBody) {
		return
	}
	result, err := h.service.Confirm(c.Request.Context(), req.Token)
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, SubscriptionResultResponse{Data: SubscriptionResultDTO{
		SubscriptionStatus: result.SubscriptionStatus, Result: result.Outcome,
	}})
}

type resendRequest struct {
	FeedbackID string `json:"feedback_id"`
	Email      string `json:"email"`
}

// ResendConfirmation godoc
// @Summary      重发订阅确认邮件
// @Description  始终返回 202，不泄露邮箱与反馈的对应关系；实际发送需匹配且通过频率限制。
// @Tags         feedback
// @Accept       json
// @Produce      json
// @Param        body  body      resendRequest  true  "反馈编号与邮箱"
// @Success      202   {object}  AcceptedResponse
// @Router       /api/v1/feedback/subscriptions/resend [post]
func (h *FeedbackHandler) ResendConfirmation(c *gin.Context) {
	if h.service == nil {
		fail(c, http.StatusServiceUnavailable, CodeFeedbackUnavailable, "feedback is not enabled")
		return
	}
	var req resendRequest
	if !bindJSON(c, &req, maxFeedbackBody) {
		return
	}
	if err := h.service.ResendConfirmation(c.Request.Context(), req.FeedbackID, req.Email, c.ClientIP()); err != nil {
		h.logger.Warn("resend confirmation failed", "error", err)
	}
	c.JSON(http.StatusAccepted, AcceptedResponse{Data: AcceptedDTO{Accepted: true}})
}

// Unsubscribe godoc
// @Summary      退订反馈邮件
// @Description  使用专用令牌，幂等；已退订时返回稳定结果。
// @Tags         feedback
// @Accept       json
// @Produce      json
// @Param        body  body      tokenRequest  true  "退订令牌"
// @Success      200   {object}  SubscriptionResultResponse
// @Failure      400   {object}  ErrorResponse
// @Router       /api/v1/feedback/subscriptions/unsubscribe [post]
func (h *FeedbackHandler) Unsubscribe(c *gin.Context) {
	if h.service == nil {
		fail(c, http.StatusServiceUnavailable, CodeFeedbackUnavailable, "feedback is not enabled")
		return
	}
	var req tokenRequest
	if !bindJSON(c, &req, maxFeedbackBody) {
		return
	}
	result, err := h.service.Unsubscribe(c.Request.Context(), req.Token)
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, SubscriptionResultResponse{Data: SubscriptionResultDTO{
		SubscriptionStatus: result.SubscriptionStatus, Result: result.Outcome,
	}})
}

func (h *FeedbackHandler) writeError(c *gin.Context, err error) {
	var validation *service.ValidationError
	switch {
	case errors.As(err, &validation):
		failFields(c, http.StatusBadRequest, CodeInvalidParam, validation.Message,
			map[string]string{validation.Field: validation.Message})
	case errors.Is(err, service.ErrFeedbackDisabled):
		fail(c, http.StatusServiceUnavailable, CodeFeedbackUnavailable, "feedback is not enabled")
	case errors.Is(err, service.ErrRateLimited):
		c.Header("Retry-After", "60")
		fail(c, http.StatusTooManyRequests, CodeRateLimited, "too many requests; retry later")
	case errors.Is(err, service.ErrTurnstileRequired):
		fail(c, http.StatusBadRequest, CodeTurnstileRequired, "turnstile token is required")
	case errors.Is(err, service.ErrTurnstileFailed):
		fail(c, http.StatusForbidden, CodeTurnstileFailed, "turnstile verification failed")
	case errors.Is(err, service.ErrVerificationUnavailable):
		c.Header("Retry-After", "30")
		fail(c, http.StatusServiceUnavailable, CodeVerificationUnavailable, "verification is temporarily unavailable")
	case errors.Is(err, service.ErrInvalidToken):
		fail(c, http.StatusBadRequest, CodeInvalidToken, "token is invalid")
	case errors.Is(err, service.ErrTokenExpired):
		fail(c, http.StatusGone, CodeTokenExpired, "token has expired")
	case errors.Is(err, service.ErrFeedbackNotFound):
		fail(c, http.StatusNotFound, CodeFeedbackNotFound, "feedback not found")
	case errors.Is(err, service.ErrVersionConflict):
		fail(c, http.StatusConflict, CodeConflict, "feedback was modified by another request")
	case errors.Is(err, service.ErrIdempotencyConflict):
		fail(c, http.StatusConflict, CodeIdempotencyConflict, "request_id was already used with different content")
	default:
		internalError(c, h.logger, err)
	}
}

func requireJSON(c *gin.Context) bool {
	contentType := c.GetHeader("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "application/json" {
		fail(c, http.StatusUnsupportedMediaType, CodeUnsupportedMediaType, "Content-Type must be application/json")
		return false
	}
	return true
}

// bindJSON enforces the request body cap and returns false after writing the error.
func bindJSON(c *gin.Context, dst any, maxBytes int64) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
	decoder := json.NewDecoder(c.Request.Body)
	if err := decoder.Decode(dst); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			fail(c, http.StatusRequestEntityTooLarge, CodePayloadTooLarge, "request body is too large")
			return false
		}
		if errors.Is(err, io.EOF) {
			fail(c, http.StatusBadRequest, CodeInvalidParam, "request body is required")
			return false
		}
		fail(c, http.StatusBadRequest, CodeInvalidParam, "invalid JSON body")
		return false
	}
	return true
}
