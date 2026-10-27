// Package settings holds the runtime-managed configuration document: SMTP,
// rate limits, antispam and subscription toggles that administrators edit in
// the admin UI and that take effect without a restart.
//
// Storage model: one JSON document in app_settings (key "runtime_settings").
// On first boot the document is seeded from environment values; afterwards
// the database wins and the environment variables act as first-boot defaults
// only. Secret fields (SMTP password, Turnstile secret key, Cloudflare API
// token) are encrypted with AES-256-GCM before the document is persisted;
// the encryption key comes from BIODAYS_SETTINGS_ENC_KEY and never touches
// the database.
package settings

import (
	"strings"
	"time"
)

// DocumentVersion prefixes the stored JSON so future schema changes can
// migrate old documents on read.
const DocumentVersion = 1

// ClearSentinel is the write-only value that erases a secret. Empty strings
// mean "keep the current value" so forms never need to echo secrets back.
const ClearSentinel = "__clear__"

// SecretFields lists the JSON paths that hold encrypted material. Validation
// and audit masking consult this set.
var SecretFields = map[string]bool{
	"smtp.password":     true,
	"turnstile.secret":  true,
	"ai_spam.api_token": true,
}

type Document struct {
	Version       int             `json:"version"`
	Revision      int             `json:"revision"`
	UpdatedAt     time.Time       `json:"updated_at"`
	UpdatedBy     string          `json:"updated_by"`
	Subscriptions bool            `json:"subscriptions_enabled"`
	SMTP          SMTPGroup       `json:"smtp"`
	RateLimits    RateLimitsGroup `json:"rate_limits"`
	Turnstile     TurnstileGroup  `json:"turnstile"`
	Spam          SpamGroup       `json:"spam"`
	AISpam        AISpamGroup     `json:"ai_spam"`
}

type SMTPGroup struct {
	Enabled   bool   `json:"enabled"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	TLSMode   string `json:"tls_mode"` // starttls | implicit_tls
	Username  string `json:"username"`
	Password  string `json:"password"` // encrypted; never returned by the API
	From      string `json:"from"`
	NotifyTo  string `json:"notify_to"`
	TimeoutMS int    `json:"timeout_ms"`
}

func (g SMTPGroup) Timeout() time.Duration {
	if g.TimeoutMS <= 0 {
		return 10 * time.Second
	}
	return time.Duration(g.TimeoutMS) * time.Millisecond
}

// MissingConfig lists the required fields that are absent, mirroring the
// pre-settings BIODAYS_* names the admin mail page already displayed.
func (g SMTPGroup) MissingConfig() []string {
	if !g.Enabled {
		return nil
	}
	missing := make([]string, 0, 3)
	if strings.TrimSpace(g.Host) == "" {
		missing = append(missing, "BIODAYS_SMTP_HOST")
	}
	if !validEmail(g.From) {
		missing = append(missing, "BIODAYS_SMTP_FROM")
	}
	if !validEmail(g.NotifyTo) {
		missing = append(missing, "BIODAYS_FEEDBACK_NOTIFY_TO")
	}
	return missing
}

type RateLimitsGroup struct {
	SubmitHourly       int `json:"submit_hourly"`
	SubmitDaily        int `json:"submit_daily"`
	ResendHourly       int `json:"resend_hourly"`
	ResendDaily        int `json:"resend_daily"`
	ConfirmHourly      int `json:"confirm_hourly"`
	ConfirmDaily       int `json:"confirm_daily"`
	ConfirmDailyGlobal int `json:"confirm_daily_global"`
	AdminMailHourly    int `json:"admin_mail_hourly"`
}

type TurnstileGroup struct {
	Enabled   bool     `json:"enabled"`
	SiteKey   string   `json:"site_key"`
	SecretKey string   `json:"secret_key"` // encrypted; never returned
	Hostnames []string `json:"hostnames"`
	TimeoutMS int      `json:"timeout_ms"`
}

func (g TurnstileGroup) Timeout() time.Duration {
	if g.TimeoutMS <= 0 {
		return 3 * time.Second
	}
	return time.Duration(g.TimeoutMS) * time.Millisecond
}

type SpamGroup struct {
	Enabled     bool `json:"enabled"`
	ReviewScore int  `json:"review_score"`
	BlockScore  int  `json:"block_score"`
}

type AISpamGroup struct {
	Enabled    bool    `json:"enabled"`
	AccountID  string  `json:"account_id"`
	APIToken   string  `json:"api_token"` // encrypted; never returned
	Model      string  `json:"model"`
	Style      string  `json:"style"` // decision | chat
	TimeoutMS  int     `json:"timeout_ms"`
	Threshold  float64 `json:"threshold"`
	DailyLimit int     `json:"daily_limit"`
	MaxChars   int     `json:"max_chars"`
}

func (g AISpamGroup) Timeout() time.Duration {
	if g.TimeoutMS <= 0 {
		return 2 * time.Second
	}
	return time.Duration(g.TimeoutMS) * time.Millisecond
}

// Clone returns a deep copy so callers can never mutate the shared snapshot.
func (d Document) Clone() Document {
	copy := d
	copy.Turnstile.Hostnames = append([]string(nil), d.Turnstile.Hostnames...)
	return copy
}

// redactSecrets replaces every encrypted field with a presence marker. Used
// before logging or auditing the document.
func (d *Document) redactSecrets() {
	mask := func(value string) string {
		if value == "" {
			return ""
		}
		return "••••"
	}
	d.SMTP.Password = mask(d.SMTP.Password)
	d.Turnstile.SecretKey = mask(d.Turnstile.SecretKey)
	d.AISpam.APIToken = mask(d.AISpam.APIToken)
}
