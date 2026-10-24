package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"biodays/server/internal/model"
	"biodays/server/internal/service"
	"biodays/server/internal/store"

	"github.com/gin-gonic/gin"
)

type MailHandler struct {
	service *service.FeedbackService
	logger  *slog.Logger
}

func NewMailHandler(svc *service.FeedbackService, logger *slog.Logger) *MailHandler {
	return &MailHandler{service: svc, logger: logger}
}

type MailStatusResponse struct {
	Data model.MailStatusDTO `json:"data"`
}

type MailTaskListResponse struct {
	Data []model.MailTaskDTO `json:"data"`
	Meta ListMeta            `json:"meta"`
}

// Status godoc
// @Summary      SMTP 配置与队列状态
// @Description  返回配置是否齐全、队列计数和最近错误；不返回任何口令。
// @Tags         admin
// @Produce      json
// @Success      200  {object}  MailStatusResponse
// @Router       /api/v1/admin/mail/status [get]
func (h *MailHandler) Status(c *gin.Context) {
	if h.service == nil {
		fail(c, http.StatusServiceUnavailable, CodeFeedbackUnavailable, "feedback is not enabled")
		return
	}
	status, err := h.service.MailStatus(c.Request.Context())
	if err != nil {
		internalError(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, MailStatusResponse{Data: status})
}

// Outbox godoc
// @Summary      邮件任务列表
// @Description  分页查看任务类型、状态、尝试次数与失败原因；不返回收件人与敏感 payload。
// @Tags         admin
// @Produce      json
// @Param        type       query     string  false  "任务类型"
// @Param        status     query     string  false  "任务状态"
// @Param        page       query     int     false  "页码"
// @Param        page_size  query     int     false  "每页数量，最大 100"
// @Success      200        {object}  MailTaskListResponse
// @Router       /api/v1/admin/mail/outbox [get]
func (h *MailHandler) Outbox(c *gin.Context) {
	if h.service == nil {
		fail(c, http.StatusServiceUnavailable, CodeFeedbackUnavailable, "feedback is not enabled")
		return
	}
	mailType := strings.TrimSpace(c.Query("type"))
	status := strings.TrimSpace(c.Query("status"))
	if mailType != "" && !containsString([]string{
		model.MailTypeAdminNewFeedback, model.MailTypeSubscriptionConfirm,
		model.MailTypeUserStatusUpdate, model.MailTypeSMTPTest,
	}, mailType) {
		fail(c, http.StatusBadRequest, CodeInvalidParam, "unknown mail type")
		return
	}
	if status != "" && !containsString([]string{
		model.MailStatusPending, model.MailStatusSending, model.MailStatusSent,
		model.MailStatusFailed, model.MailStatusUnknown, model.MailStatusCancelled,
	}, status) {
		fail(c, http.StatusBadRequest, CodeInvalidParam, "unknown mail status")
		return
	}
	page, pageSize := pagination(c)
	items, total, err := h.service.MailTasks(c.Request.Context(), store.OutboxFilter{
		Type: mailType, Status: status, Page: page, PageSize: pageSize,
	})
	if err != nil {
		internalError(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, MailTaskListResponse{
		Data: items, Meta: ListMeta{Total: total, Page: page, PageSize: pageSize},
	})
}

// SendTest godoc
// @Summary      发送 SMTP 测试邮件
// @Description  仅发往配置的管理员接收地址，返回 202；调用方不能指定收件人。
// @Tags         admin
// @Produce      json
// @Success      202  {object}  AcceptedResponse
// @Failure      503  {object}  ErrorResponse
// @Router       /api/v1/admin/mail/test [post]
func (h *MailHandler) SendTest(c *gin.Context) {
	if h.service == nil {
		fail(c, http.StatusServiceUnavailable, CodeFeedbackUnavailable, "feedback is not enabled")
		return
	}
	if err := h.service.QueueTestMail(c.Request.Context()); err != nil {
		if errors.Is(err, service.ErrSMTPUnavailable) {
			fail(c, http.StatusServiceUnavailable, CodeFeedbackUnavailable, "smtp is not enabled")
			return
		}
		internalError(c, h.logger, err)
		return
	}
	c.JSON(http.StatusAccepted, AcceptedResponse{Data: AcceptedDTO{Accepted: true}})
}

// Retry godoc
// @Summary      重试邮件任务
// @Description  仅 failed/unknown 任务可重试；发送前由 worker 复查订阅与反馈状态。
// @Tags         admin
// @Produce      json
// @Param        id   path      string  true  "任务 ID"
// @Success      202  {object}  AcceptedResponse
// @Failure      404  {object}  ErrorResponse
// @Failure      409  {object}  ErrorResponse
// @Router       /api/v1/admin/mail/outbox/{id}/retry [post]
func (h *MailHandler) Retry(c *gin.Context) {
	if h.service == nil {
		fail(c, http.StatusServiceUnavailable, CodeFeedbackUnavailable, "feedback is not enabled")
		return
	}
	if err := h.service.RetryMail(c.Request.Context(), c.Param("id")); err != nil {
		switch {
		case errors.Is(err, service.ErrMailNotFound):
			fail(c, http.StatusConflict, CodeConflict, "mail task is not retryable")
		case errors.Is(err, service.ErrFeedbackNotFound):
			fail(c, http.StatusNotFound, CodeFeedbackNotFound, "feedback not found")
		default:
			internalError(c, h.logger, err)
		}
		return
	}
	c.JSON(http.StatusAccepted, AcceptedResponse{Data: AcceptedDTO{Accepted: true}})
}
