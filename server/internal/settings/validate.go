package settings

import (
	"fmt"
	"net/mail"
	"strings"
)

// ValidationError mirrors service.ValidationError: a field key plus a
// human-readable message, mapped to a 400 with field errors by the handler.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

func newFieldError(field, format string, args ...any) error {
	return &ValidationError{Field: field, Message: fmt.Sprintf(format, args...)}
}

// Validate checks the document invariants shared by the environment loading
// path and the admin save path. hasEncryptionKey controls whether secret
// fields may be written: without the AES key secrets cannot be stored, so
// only documents without secrets pass.
func Validate(doc Document, hasEncryptionKey bool) error {
	if err := validateSMTP(doc.SMTP, hasEncryptionKey); err != nil {
		return err
	}
	if err := validateRateLimits(doc.RateLimits); err != nil {
		return err
	}
	if err := validateTurnstile(doc.Turnstile, hasEncryptionKey); err != nil {
		return err
	}
	if err := validateSpam(doc.Spam); err != nil {
		return err
	}
	return validateAISpam(doc.AISpam, doc.Spam, hasEncryptionKey)
}

func validateSMTP(g SMTPGroup, hasEncryptionKey bool) error {
	if !g.Enabled {
		return nil
	}
	if strings.TrimSpace(g.Host) == "" {
		return newFieldError("smtp.host", "SMTP 启用时必须填写服务器地址")
	}
	if g.Port < 1 || g.Port > 65535 {
		return newFieldError("smtp.port", "SMTP 端口必须在 1–65535 之间")
	}
	if g.TLSMode != "starttls" && g.TLSMode != "implicit_tls" {
		return newFieldError("smtp.tls_mode", "TLS 模式必须是 starttls 或 implicit_tls")
	}
	if !validEmail(g.From) {
		return newFieldError("smtp.from", "发件地址必须是合法邮箱")
	}
	if !validEmail(g.NotifyTo) {
		return newFieldError("smtp.notify_to", "管理员通知地址必须是合法邮箱")
	}
	if strings.TrimSpace(g.Username) != "" && strings.TrimSpace(g.Password) == "" {
		return newFieldError("smtp.password", "填写 SMTP 用户名时必须提供密码")
	}
	if err := validateSecret(g.Password, "smtp.password", "SMTP 密码", hasEncryptionKey); err != nil {
		return err
	}
	if g.TimeoutMS < 1000 || g.TimeoutMS > 60000 {
		return newFieldError("smtp.timeout_ms", "SMTP 超时必须在 1–60 秒之间")
	}
	return nil
}

// validateSecret enforces the write-only contract: stored values must be
// ciphertext (v1:…), the sentinel erases, and plaintext only passes while
// the manager is about to encrypt it (handled by callers that pass the
// pre-encryption document with already-encrypted values).
func validateSecret(value, field, label string, hasEncryptionKey bool) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || trimmed == ClearSentinel {
		return nil
	}
	if !isCiphertext(trimmed) {
		return newFieldError(field, "%s不是有效的写入指令（留空表示不变，%s 表示清除）", label, ClearSentinel)
	}
	if !hasEncryptionKey {
		return newFieldError(field, "服务器未配置 BIODAYS_SETTINGS_ENC_KEY，无法保存%s", label)
	}
	return nil
}

func validateRateLimits(g RateLimitsGroup) error {
	fields := []struct {
		name  string
		value int
	}{
		{"rate_limits.submit_hourly", g.SubmitHourly},
		{"rate_limits.submit_daily", g.SubmitDaily},
		{"rate_limits.resend_hourly", g.ResendHourly},
		{"rate_limits.resend_daily", g.ResendDaily},
		{"rate_limits.confirm_hourly", g.ConfirmHourly},
		{"rate_limits.confirm_daily", g.ConfirmDaily},
		{"rate_limits.confirm_daily_global", g.ConfirmDailyGlobal},
		{"rate_limits.admin_mail_hourly", g.AdminMailHourly},
	}
	for _, f := range fields {
		if f.value < 1 || f.value > 10000 {
			return newFieldError(f.name, "频控限额必须是 1–10000 之间的整数")
		}
	}
	return nil
}

func validateTurnstile(g TurnstileGroup, hasEncryptionKey bool) error {
	if !g.Enabled {
		return nil
	}
	if strings.TrimSpace(g.SiteKey) == "" {
		return newFieldError("turnstile.site_key", "Turnstile 启用时必须填写 Site Key")
	}
	if strings.TrimSpace(g.SecretKey) == "" {
		return newFieldError("turnstile.secret_key", "Turnstile 启用时必须填写 Secret Key")
	}
	if err := validateSecret(g.SecretKey, "turnstile.secret_key", "Turnstile Secret Key", hasEncryptionKey); err != nil {
		return err
	}
	for _, host := range g.Hostnames {
		trimmed := strings.ToLower(strings.TrimSpace(host))
		if trimmed == "" || strings.ContainsAny(trimmed, " /") {
			return newFieldError("turnstile.hostnames", "Hostname 列表包含非法条目")
		}
	}
	if g.TimeoutMS < 1000 || g.TimeoutMS > 10000 {
		return newFieldError("turnstile.timeout_ms", "Turnstile 超时必须在 1–10 秒之间")
	}
	return nil
}

func validateSpam(g SpamGroup) error {
	if !g.Enabled {
		return nil
	}
	if g.ReviewScore < 1 || g.BlockScore < 50 || g.BlockScore > 100 || g.ReviewScore >= g.BlockScore {
		return newFieldError("spam.review_score", "评分阈值需满足 1 ≤ 待复核 < 自动隔离 ≤ 100，且自动隔离 ≥ 50")
	}
	return nil
}

func validateAISpam(g AISpamGroup, spam SpamGroup, hasEncryptionKey bool) error {
	if !g.Enabled {
		return nil
	}
	if !spam.Enabled {
		return newFieldError("ai_spam.enabled", "AI 复核需要先启用规则评分")
	}
	if strings.TrimSpace(g.AccountID) == "" {
		return newFieldError("ai_spam.account_id", "AI 复核启用时必须填写 Cloudflare Account ID")
	}
	if strings.TrimSpace(g.APIToken) == "" {
		return newFieldError("ai_spam.api_token", "AI 复核启用时必须填写 API Token")
	}
	if err := validateSecret(g.APIToken, "ai_spam.api_token", "API Token", hasEncryptionKey); err != nil {
		return err
	}
	if strings.TrimSpace(g.Model) == "" {
		return newFieldError("ai_spam.model", "AI 复核启用时必须填写模型名称")
	}
	if g.Style != "decision" && g.Style != "chat" {
		return newFieldError("ai_spam.style", "AI 请求形态必须是 decision 或 chat")
	}
	if g.TimeoutMS < 1000 || g.TimeoutMS > 10000 {
		return newFieldError("ai_spam.timeout_ms", "AI 复核超时必须在 1–10 秒之间")
	}
	if g.Threshold < 0.5 || g.Threshold > 1 {
		return newFieldError("ai_spam.threshold", "AI 判定阈值必须在 0.5–1 之间")
	}
	if g.DailyLimit < 1 || g.DailyLimit > 10000 {
		return newFieldError("ai_spam.daily_limit", "AI 每日额度必须是 1–10000 之间的整数")
	}
	if g.MaxChars < 500 || g.MaxChars > 8000 {
		return newFieldError("ai_spam.max_chars", "AI 送审字符上限必须在 500–8000 之间")
	}
	return nil
}

func validEmail(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	parsed, err := mail.ParseAddress(trimmed)
	return err == nil && parsed.Address == trimmed
}
