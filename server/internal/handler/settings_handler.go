package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"biodays/server/internal/settings"
	"biodays/server/internal/store"

	"github.com/gin-gonic/gin"
)

type SettingsHandler struct {
	manager  *settings.Manager
	feedback *store.FeedbackStore
	logger   *slog.Logger
}

func NewSettingsHandler(manager *settings.Manager, feedback *store.FeedbackStore, logger *slog.Logger) *SettingsHandler {
	return &SettingsHandler{manager: manager, feedback: feedback, logger: logger}
}

// secretState is the write-only projection of a secret field: presence and
// readability without any material.
type secretState struct {
	Configured bool `json:"configured"`
	Encrypted  bool `json:"encrypted"`
	Readable   bool `json:"readable"`
}

type smtpPublic struct {
	Enabled   bool        `json:"enabled"`
	Host      string      `json:"host"`
	Port      int         `json:"port"`
	TLSMode   string      `json:"tls_mode"`
	Username  string      `json:"username"`
	From      string      `json:"from"`
	NotifyTo  string      `json:"notify_to"`
	TimeoutMS int         `json:"timeout_ms"`
	Password  secretState `json:"password"`
}

type turnstilePublic struct {
	Enabled   bool        `json:"enabled"`
	SiteKey   string      `json:"site_key"`
	Hostnames []string    `json:"hostnames"`
	TimeoutMS int         `json:"timeout_ms"`
	SecretKey secretState `json:"secret_key"`
}

type aiSpamPublic struct {
	Enabled    bool        `json:"enabled"`
	AccountID  string      `json:"account_id"`
	Model      string      `json:"model"`
	Style      string      `json:"style"`
	TimeoutMS  int         `json:"timeout_ms"`
	Threshold  float64     `json:"threshold"`
	DailyLimit int         `json:"daily_limit"`
	MaxChars   int         `json:"max_chars"`
	APIToken   secretState `json:"api_token"`
}

type settingsPublic struct {
	Revision      int                      `json:"revision"`
	UpdatedAt     string                   `json:"updated_at"`
	UpdatedBy     string                   `json:"updated_by"`
	EncryptionKey bool                     `json:"encryption_key_configured"`
	ClearSentinel string                   `json:"clear_sentinel"`
	Subscriptions bool                     `json:"subscriptions_enabled"`
	SMTP          smtpPublic               `json:"smtp"`
	RateLimits    settings.RateLimitsGroup `json:"rate_limits"`
	Turnstile     turnstilePublic          `json:"turnstile"`
	Spam          settings.SpamGroup       `json:"spam"`
	AISpam        aiSpamPublic             `json:"ai_spam"`
}

func publicSecret(value string, readable bool) secretState {
	return secretState{
		Configured: value != "",
		Encrypted:  value != "",
		Readable:   readable,
	}
}

func (h *SettingsHandler) public(doc settings.Document, encKeyOK, secretsReadable bool) settingsPublic {
	return settingsPublic{
		Revision:      doc.Revision,
		UpdatedAt:     doc.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedBy:     doc.UpdatedBy,
		EncryptionKey: encKeyOK,
		ClearSentinel: settings.ClearSentinel,
		Subscriptions: doc.Subscriptions,
		SMTP: smtpPublic{
			Enabled: doc.SMTP.Enabled, Host: doc.SMTP.Host, Port: doc.SMTP.Port,
			TLSMode: doc.SMTP.TLSMode, Username: doc.SMTP.Username,
			From: doc.SMTP.From, NotifyTo: doc.SMTP.NotifyTo,
			TimeoutMS: doc.SMTP.TimeoutMS, Password: publicSecret(doc.SMTP.Password, secretsReadable),
		},
		RateLimits: doc.RateLimits,
		Turnstile: turnstilePublic{
			Enabled: doc.Turnstile.Enabled, SiteKey: doc.Turnstile.SiteKey,
			Hostnames: doc.Turnstile.Hostnames, TimeoutMS: doc.Turnstile.TimeoutMS,
			SecretKey: publicSecret(doc.Turnstile.SecretKey, secretsReadable),
		},
		Spam: doc.Spam,
		AISpam: aiSpamPublic{
			Enabled: doc.AISpam.Enabled, AccountID: doc.AISpam.AccountID,
			Model: doc.AISpam.Model, Style: doc.AISpam.Style,
			TimeoutMS: doc.AISpam.TimeoutMS, Threshold: doc.AISpam.Threshold,
			DailyLimit: doc.AISpam.DailyLimit, MaxChars: doc.AISpam.MaxChars,
			APIToken: publicSecret(doc.AISpam.APIToken, secretsReadable),
		},
	}
}

type SettingsResponse struct {
	Data settingsPublic `json:"data"`
}

// GetSettings godoc
// @Summary      运行时设置
// @Description  返回 SMTP、频控、Turnstile、垃圾评分与订阅开关的当前值；密钥字段只返回配置状态，永不回传内容。
// @Tags         admin
// @Produce      json
// @Success      200  {object}  SettingsResponse
// @Failure      503  {object}  ErrorResponse
// @Router       /api/v1/admin/settings [get]
func (h *SettingsHandler) GetSettings(c *gin.Context) {
	if h.manager == nil {
		fail(c, http.StatusServiceUnavailable, CodeFeedbackUnavailable, "feedback is not enabled")
		return
	}
	snap := h.manager.Snapshot()
	c.JSON(http.StatusOK, SettingsResponse{Data: h.public(snap.Document, h.manager.HasEncryptionKey(), snap.SMTPError == "" || snap.Document.SMTP.Password == "")})
}

type settingsPatchRequest struct {
	BaseRevision  int                       `json:"base_revision"`
	Subscriptions *bool                     `json:"subscriptions_enabled"`
	SMTP          *settings.SMTPGroup       `json:"smtp"`
	RateLimits    *settings.RateLimitsGroup `json:"rate_limits"`
	Turnstile     *settings.TurnstileGroup  `json:"turnstile"`
	Spam          *settings.SpamGroup       `json:"spam"`
	AISpam        *settings.AISpamGroup     `json:"ai_spam"`
}

// UpdateSettings godoc
// @Summary      更新运行时设置
// @Description  按 base_revision 做乐观并发；未提供的组不变。密钥字段留空表示不变，填 __clear__ 表示清除；保存后立即生效，无需重启。
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        body  body      settingsPatchRequest  true  "设置补丁"
// @Success      200   {object}  SettingsResponse
// @Failure      400   {object}  ErrorResponse
// @Failure      409   {object}  ErrorResponse
// @Failure      503   {object}  ErrorResponse
// @Router       /api/v1/admin/settings [patch]
func (h *SettingsHandler) UpdateSettings(c *gin.Context) {
	if h.manager == nil {
		fail(c, http.StatusServiceUnavailable, CodeFeedbackUnavailable, "feedback is not enabled")
		return
	}
	var req settingsPatchRequest
	if !bindJSON(c, &req, 64<<10) {
		return
	}
	if req.SMTP == nil && req.RateLimits == nil && req.Turnstile == nil &&
		req.Spam == nil && req.AISpam == nil && req.Subscriptions == nil {
		fail(c, http.StatusBadRequest, CodeInvalidParam, "no changes supplied")
		return
	}
	doc, err := h.manager.Update(c.Request.Context(), settings.PatchInput{
		BaseRevision: req.BaseRevision, Actor: identityUsername(c),
		Subscriptions: req.Subscriptions, SMTP: req.SMTP, RateLimits: req.RateLimits,
		Turnstile: req.Turnstile, Spam: req.Spam, AISpam: req.AISpam,
	}, h.feedback)
	if err != nil {
		var validation *settings.ValidationError
		switch {
		case errors.As(err, &validation):
			failFields(c, http.StatusBadRequest, CodeInvalidParam, validation.Message,
				map[string]string{validation.Field: validation.Message})
		case errors.Is(err, settings.ErrRevisionConflict):
			fail(c, http.StatusConflict, CodeSettingsRevisionConflict, "settings were modified by another request; refresh and retry")
		case errors.Is(err, settings.ErrSecretUnwritable):
			fail(c, http.StatusBadRequest, CodeSettingsSecretUnwritable, "server has no settings encryption key; secret fields cannot be saved")
		default:
			internalError(c, h.logger, err)
		}
		return
	}
	snap := h.manager.Snapshot()
	c.JSON(http.StatusOK, SettingsResponse{Data: h.public(doc, h.manager.HasEncryptionKey(), snap.SMTPError == "")})
}

type settingsAuditDTO struct {
	ID        uint   `json:"id"`
	Revision  int    `json:"revision"`
	Actor     string `json:"actor"`
	Changes   string `json:"changes"`
	CreatedAt string `json:"created_at"`
}

type SettingsAuditResponse struct {
	Data []settingsAuditDTO `json:"data"`
	Meta ListMeta           `json:"meta"`
}

// ListSettingsAudit godoc
// @Summary      设置修改审计
// @Description  分页返回设置修改记录（新到旧）；密钥变更只显示掩码。
// @Tags         admin
// @Produce      json
// @Param        page       query int false "页码"
// @Param        page_size  query int false "每页数量，最大 100"
// @Success      200  {object}  SettingsAuditResponse
// @Failure      503  {object}  ErrorResponse
// @Router       /api/v1/admin/settings/audit [get]
func (h *SettingsHandler) ListSettingsAudit(c *gin.Context) {
	if h.feedback == nil {
		fail(c, http.StatusServiceUnavailable, CodeFeedbackUnavailable, "feedback is not enabled")
		return
	}
	page, pageSize := pagination(c)
	items, total, err := h.feedback.ListSettingsAudit(c.Request.Context(), page, pageSize)
	if err != nil {
		internalError(c, h.logger, err)
		return
	}
	out := make([]settingsAuditDTO, 0, len(items))
	for _, item := range items {
		out = append(out, settingsAuditDTO{
			ID: item.ID, Revision: item.Revision, Actor: item.Actor,
			Changes: item.Changes, CreatedAt: item.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		})
	}
	c.JSON(http.StatusOK, SettingsAuditResponse{Data: out, Meta: ListMeta{Total: total, Page: page, PageSize: pageSize}})
}
