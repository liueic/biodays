package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"biodays/server/internal/middleware"
	"biodays/server/internal/model"
	"biodays/server/internal/service"
	"biodays/server/internal/store"

	"github.com/gin-gonic/gin"
)

type AdminHandler struct {
	admin    *service.AdminService
	feedback *service.FeedbackService
	logger   *slog.Logger
	secure   bool
}

func NewAdminHandler(admin *service.AdminService, feedback *service.FeedbackService, logger *slog.Logger, secureCookie bool) *AdminHandler {
	return &AdminHandler{admin: admin, feedback: feedback, logger: logger, secure: secureCookie}
}

type LoginRequest struct {
	Username string `json:"username" example:"admin"`
	Password string `json:"password" example:"correct horse battery staple"`
}

type AdminSessionDTO struct {
	Username  string    `json:"username"`
	CSRFToken string    `json:"csrf_token"`
	ExpiresAt time.Time `json:"expires_at"`
}

type AdminSessionResponse struct {
	Data AdminSessionDTO `json:"data"`
}

// Login godoc
// @Summary      管理员登录
// @Description  同源 JSON 请求；成功设置 HttpOnly + SameSite=Strict 会话 Cookie，返回 CSRF 令牌。失败尝试按 IP/账号独立限流。
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        body  body      LoginRequest  true  "账号密码"
// @Success      200   {object}  AdminSessionResponse
// @Failure      401   {object}  ErrorResponse
// @Failure      429   {object}  ErrorResponse
// @Router       /api/v1/admin/login [post]
func (h *AdminHandler) Login(c *gin.Context) {
	if h.admin == nil {
		fail(c, http.StatusServiceUnavailable, CodeFeedbackUnavailable, "feedback is not enabled")
		return
	}
	if !strings.HasPrefix(strings.ToLower(c.GetHeader("Content-Type")), "application/json") {
		fail(c, http.StatusBadRequest, CodeInvalidParam, "Content-Type must be application/json")
		return
	}
	var req LoginRequest
	if !bindJSON(c, &req, 8<<10) {
		return
	}
	identity, err := h.admin.Login(c.Request.Context(), req.Username, req.Password, c.ClientIP())
	switch {
	case errors.Is(err, service.ErrLoginRateLimited):
		c.Header("Retry-After", "900")
		fail(c, http.StatusTooManyRequests, CodeRateLimited, "too many login attempts; retry later")
		return
	case errors.Is(err, service.ErrInvalidCredentials):
		fail(c, http.StatusUnauthorized, CodeUnauthenticated, "invalid username or password")
		return
	case errors.Is(err, service.ErrAdminNotInitialized):
		fail(c, http.StatusServiceUnavailable, CodeAdminNotInitialized, "admin account is not initialized; run 'biodays admin set-password' on the server")
		return
	case err != nil:
		internalError(c, h.logger, err)
		return
	}
	maxAge := int(time.Until(identity.ExpiresAt).Seconds())
	if maxAge <= 0 {
		maxAge = int((8 * time.Hour).Seconds())
	}
	h.setSessionCookie(c, identity.RawToken, maxAge)
	c.JSON(http.StatusOK, AdminSessionResponse{Data: AdminSessionDTO{
		Username: identity.Username, CSRFToken: identity.CSRF, ExpiresAt: identity.ExpiresAt,
	}})
}

// Logout godoc
// @Summary      管理员退出
// @Description  立即撤销当前会话并清除 Cookie。
// @Tags         admin
// @Produce      json
// @Success      200   {object}  AcceptedResponse
// @Failure      401   {object}  ErrorResponse
// @Failure      403   {object}  ErrorResponse
// @Router       /api/v1/admin/logout [post]
func (h *AdminHandler) Logout(c *gin.Context) {
	raw, _ := c.Cookie(middleware.AdminCookieName)
	if h.admin != nil && raw != "" {
		if err := h.admin.Logout(c.Request.Context(), raw); err != nil {
			internalError(c, h.logger, err)
			return
		}
	}
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(middleware.AdminCookieName, "", -1, "/", "", h.secure, true)
	c.JSON(http.StatusOK, AcceptedResponse{Data: AcceptedDTO{Accepted: true}})
}

// Me godoc
// @Summary      当前管理员会话
// @Description  返回管理员用户名、CSRF 令牌与过期时间；令牌不进入 localStorage。
// @Tags         admin
// @Produce      json
// @Success      200  {object}  AdminSessionResponse
// @Failure      401  {object}  ErrorResponse
// @Router       /api/v1/admin/me [get]
func (h *AdminHandler) Me(c *gin.Context) {
	identity, ok := middleware.AdminIdentity(c)
	if !ok {
		fail(c, http.StatusUnauthorized, CodeUnauthenticated, "admin login required")
		return
	}
	c.JSON(http.StatusOK, AdminSessionResponse{Data: AdminSessionDTO{
		Username: identity.Username, CSRFToken: identity.CSRF, ExpiresAt: identity.ExpiresAt,
	}})
}

type AdminFeedbackListResponse struct {
	Data []model.FeedbackListItemDTO `json:"data"`
	Meta ListMeta                    `json:"meta"`
}

// ListFeedback godoc
// @Summary      反馈收件箱列表
// @Description  按新到旧排序，支持 kind/status/q 筛选与分页；列表不返回完整邮箱。
// @Tags         admin
// @Produce      json
// @Param        kind       query     string  false  "反馈类型"
// @Param        status     query     string  false  "反馈状态"
// @Param        q          query     string  false  "关键词"
// @Param        page       query     int     false  "页码"
// @Param        page_size  query     int     false  "每页数量，最大 100"
// @Success      200        {object}  AdminFeedbackListResponse
// @Failure      400        {object}  ErrorResponse
// @Failure      401        {object}  ErrorResponse
// @Router       /api/v1/admin/feedback [get]
func (h *AdminHandler) ListFeedback(c *gin.Context) {
	if h.feedback == nil {
		fail(c, http.StatusServiceUnavailable, CodeFeedbackUnavailable, "feedback is not enabled")
		return
	}
	kind := strings.TrimSpace(c.Query("kind"))
	status := strings.TrimSpace(c.Query("status"))
	query := strings.TrimSpace(c.Query("q"))
	if len([]rune(query)) > 100 {
		fail(c, http.StatusBadRequest, CodeInvalidParam, "q accepts at most 100 characters")
		return
	}
	if kind != "" && !containsString(model.FeedbackKinds, kind) {
		fail(c, http.StatusBadRequest, CodeInvalidParam, "unknown kind")
		return
	}
	if status != "" && !containsString(model.FeedbackStatuses, status) {
		fail(c, http.StatusBadRequest, CodeInvalidParam, "unknown status")
		return
	}
	page, pageSize := pagination(c)
	items, total, err := h.feedback.ListFeedback(c.Request.Context(), store.FeedbackListFilter{
		Kind: kind, Status: status, Query: query, Page: page, PageSize: pageSize,
	})
	if err != nil {
		internalError(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, AdminFeedbackListResponse{
		Data: items, Meta: ListMeta{Total: total, Page: page, PageSize: pageSize},
	})
}

type AdminFeedbackDetailResponse struct {
	Data model.FeedbackDetailDTO `json:"data"`
}

// GetFeedback godoc
// @Summary      反馈详情
// @Description  返回完整投稿、联系方式、处理记录和邮件任务状态。
// @Tags         admin
// @Produce      json
// @Param        id   path      string  true  "反馈编号"
// @Success      200  {object}  AdminFeedbackDetailResponse
// @Failure      404  {object}  ErrorResponse
// @Router       /api/v1/admin/feedback/{id} [get]
func (h *AdminHandler) GetFeedback(c *gin.Context) {
	if h.feedback == nil {
		fail(c, http.StatusServiceUnavailable, CodeFeedbackUnavailable, "feedback is not enabled")
		return
	}
	detail, err := h.feedback.FeedbackDetail(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.writeFeedbackError(c, err)
		return
	}
	c.JSON(http.StatusOK, AdminFeedbackDetailResponse{Data: detail})
}

type AdminPatchRequest struct {
	Status           string  `json:"status"`
	InternalNote     *string `json:"internal_note"`
	PublicReply      *string `json:"public_reply"`
	Version          int     `json:"version"`
	NotifySubscriber bool    `json:"notify_subscriber"`
}

type AdminUpdateMeta struct {
	Notified          bool   `json:"notified"`
	NotifySkippedNote string `json:"notify_skipped_note,omitempty"`
}

type AdminUpdateResponse struct {
	Data model.FeedbackDetailDTO `json:"data"`
	Meta AdminUpdateMeta         `json:"meta"`
}

// UpdateFeedback godoc
// @Summary      更新反馈状态与备注
// @Description  携带 version 做并发控制；状态变化时记录事件。只有已确认订阅且显式 notify_subscriber 时才排队用户邮件。
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        id    path      string             true  "反馈编号"
// @Param        body  body      AdminPatchRequest  true  "更新内容"
// @Success      200   {object}  AdminUpdateResponse
// @Failure      400   {object}  ErrorResponse
// @Failure      404   {object}  ErrorResponse
// @Failure      409   {object}  ErrorResponse
// @Router       /api/v1/admin/feedback/{id} [patch]
func (h *AdminHandler) UpdateFeedback(c *gin.Context) {
	if h.feedback == nil {
		fail(c, http.StatusServiceUnavailable, CodeFeedbackUnavailable, "feedback is not enabled")
		return
	}
	var req AdminPatchRequest
	if !bindJSON(c, &req, 64<<10) {
		return
	}
	if req.Version < 1 {
		fail(c, http.StatusBadRequest, CodeInvalidParam, "version is required")
		return
	}
	if req.Status == "" && req.InternalNote == nil && req.PublicReply == nil {
		fail(c, http.StatusBadRequest, CodeInvalidParam, "no changes supplied")
		return
	}
	actor := identityUsername(c)
	result, err := h.feedback.UpdateFeedback(c.Request.Context(), actor, service.AdminUpdateInput{
		ID: c.Param("id"), Version: req.Version, Status: req.Status,
		InternalNote: req.InternalNote, PublicReply: req.PublicReply,
		NotifySubscriber: req.NotifySubscriber,
	})
	if err != nil {
		h.writeFeedbackError(c, err)
		return
	}
	detail, err := h.feedback.FeedbackDetail(c.Request.Context(), result.Feedback.ID)
	if err != nil {
		internalError(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, AdminUpdateResponse{
		Data: detail,
		Meta: AdminUpdateMeta{Notified: result.Notified, NotifySkippedNote: result.NotifySkippedNote},
	})
}

// DeleteFeedback godoc
// @Summary      删除反馈
// @Description  清理联系方式、订阅令牌、事件与未发送邮件，保留最小删除审计信息。
// @Tags         admin
// @Produce      json
// @Param        id   path      string  true  "反馈编号"
// @Success      200  {object}  AcceptedResponse
// @Failure      404  {object}  ErrorResponse
// @Router       /api/v1/admin/feedback/{id} [delete]
func (h *AdminHandler) DeleteFeedback(c *gin.Context) {
	if h.feedback == nil {
		fail(c, http.StatusServiceUnavailable, CodeFeedbackUnavailable, "feedback is not enabled")
		return
	}
	actor := identityUsername(c)
	if err := h.feedback.DeleteFeedback(c.Request.Context(), actor, c.Param("id")); err != nil {
		h.writeFeedbackError(c, err)
		return
	}
	c.JSON(http.StatusOK, AcceptedResponse{Data: AcceptedDTO{Accepted: true}})
}

func (h *AdminHandler) writeFeedbackError(c *gin.Context, err error) {
	var validation *service.ValidationError
	switch {
	case errors.As(err, &validation):
		failFields(c, http.StatusBadRequest, CodeInvalidParam, validation.Message,
			map[string]string{validation.Field: validation.Message})
	case errors.Is(err, service.ErrFeedbackNotFound):
		fail(c, http.StatusNotFound, CodeFeedbackNotFound, "feedback not found")
	case errors.Is(err, service.ErrVersionConflict):
		fail(c, http.StatusConflict, CodeConflict, "feedback was modified by another request")
	case errors.Is(err, service.ErrIdempotencyConflict):
		fail(c, http.StatusConflict, CodeIdempotencyConflict, "request_id was already used with different content")
	case errors.Is(err, service.ErrMailNotFound):
		fail(c, http.StatusConflict, CodeConflict, "mail task is not retryable")
	default:
		internalError(c, h.logger, err)
	}
}

func (h *AdminHandler) setSessionCookie(c *gin.Context, raw string, maxAge int) {
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(middleware.AdminCookieName, raw, maxAge, "/", "", h.secure, true)
}

// identityUsername reports the session's account name for audit records.
func identityUsername(c *gin.Context) string {
	if identity, ok := middleware.AdminIdentity(c); ok {
		return identity.Username
	}
	return "admin"
}

func pagination(c *gin.Context) (int, int) {
	page := 1
	pageSize := 20
	if raw := c.Query("page"); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value > 0 {
			page = value
		}
	}
	if raw := c.Query("page_size"); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value > 0 {
			pageSize = min(value, 100)
		}
	}
	return page, pageSize
}

func containsString(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
