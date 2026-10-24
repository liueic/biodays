package config

import (
	"fmt"
	"log/slog"
	"net"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
	"golang.org/x/crypto/bcrypt"
)

type Config struct {
	Port           int      `env:"BIODAYS_PORT" envDefault:"8080"`
	DBPath         string   `env:"BIODAYS_DB_PATH" envDefault:"./biodays.db"`
	DataDir        string   `env:"BIODAYS_DATA_DIR"`
	CORSOrigins    []string `env:"BIODAYS_CORS_ORIGINS" envSeparator:"," envDefault:"http://localhost:3000"`
	TrustedProxies []string `env:"BIODAYS_TRUSTED_PROXIES" envSeparator:","`
	RateLimit      int      `env:"BIODAYS_RATE_LIMIT" envDefault:"60"`
	LogLevel       string   `env:"BIODAYS_LOG_LEVEL" envDefault:"info"`
	SiteURL        string   `env:"BIODAYS_SITE_URL" envDefault:"http://localhost:3000"`

	FeedbackEnabled      bool          `env:"BIODAYS_FEEDBACK_ENABLED" envDefault:"false"`
	FeedbackDBPath       string        `env:"BIODAYS_FEEDBACK_DB_PATH" envDefault:"./feedback.db"`
	AdminUsername        string        `env:"BIODAYS_ADMIN_USERNAME"`
	AdminPasswordHash    string        `env:"BIODAYS_ADMIN_PASSWORD_HASH"`
	AdminSessionTTL      time.Duration `env:"BIODAYS_ADMIN_SESSION_TTL" envDefault:"8h"`
	AbuseKeySecret       string        `env:"BIODAYS_ABUSE_KEY_SECRET"`
	SubscriptionsEnabled bool          `env:"BIODAYS_FEEDBACK_SUBSCRIPTIONS_ENABLED" envDefault:"true"`
	SubmitHourlyLimit    int           `env:"BIODAYS_FEEDBACK_SUBMIT_HOURLY" envDefault:"5"`
	SubmitDailyLimit     int           `env:"BIODAYS_FEEDBACK_SUBMIT_DAILY" envDefault:"20"`
	ResendHourlyLimit    int           `env:"BIODAYS_FEEDBACK_RESEND_HOURLY" envDefault:"3"`
	ResendDailyLimit     int           `env:"BIODAYS_FEEDBACK_RESEND_DAILY" envDefault:"10"`

	SMTPEnabled      bool          `env:"BIODAYS_SMTP_ENABLED" envDefault:"false"`
	SMTPHost         string        `env:"BIODAYS_SMTP_HOST"`
	SMTPPort         int           `env:"BIODAYS_SMTP_PORT" envDefault:"587"`
	SMTPTLSMode      string        `env:"BIODAYS_SMTP_TLS_MODE" envDefault:"starttls"`
	SMTPUsername     string        `env:"BIODAYS_SMTP_USERNAME"`
	SMTPPassword     string        `env:"BIODAYS_SMTP_PASSWORD"`
	SMTPFrom         string        `env:"BIODAYS_SMTP_FROM"`
	FeedbackNotifyTo string        `env:"BIODAYS_FEEDBACK_NOTIFY_TO"`
	SMTPTimeout      time.Duration `env:"BIODAYS_SMTP_TIMEOUT" envDefault:"10s"`
}

func Load() (Config, error) {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse environment: %w", err)
	}
	if cfg.Port < 1 || cfg.Port > 65535 {
		return Config{}, fmt.Errorf("BIODAYS_PORT must be between 1 and 65535, got %d", cfg.Port)
	}
	if cfg.RateLimit < 1 {
		return Config{}, fmt.Errorf("BIODAYS_RATE_LIMIT must be positive, got %d", cfg.RateLimit)
	}
	origins := make([]string, 0, len(cfg.CORSOrigins))
	for _, origin := range cfg.CORSOrigins {
		if trimmed := strings.TrimSpace(origin); trimmed != "" {
			origins = append(origins, trimmed)
		}
	}
	if len(origins) == 0 {
		origins = []string{"http://localhost:3000"}
	}
	cfg.CORSOrigins = origins

	trustedProxies, err := parseTrustedProxies(cfg.TrustedProxies)
	if err != nil {
		return Config{}, err
	}
	cfg.TrustedProxies = trustedProxies

	if _, err := ParseLogLevel(cfg.LogLevel); err != nil {
		return Config{}, err
	}
	if err := normalizeSiteURL(&cfg); err != nil {
		return Config{}, err
	}
	cfg.CORSOrigins = withSiteOrigin(cfg.CORSOrigins, cfg.SiteURL)
	if err := validateFeedbackConfig(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func parseTrustedProxies(values []string) ([]string, error) {
	proxies := make([]string, 0, len(values))
	for _, value := range values {
		proxy := strings.TrimSpace(value)
		if proxy == "" {
			continue
		}
		if net.ParseIP(proxy) == nil {
			if _, _, err := net.ParseCIDR(proxy); err != nil {
				return nil, fmt.Errorf("BIODAYS_TRUSTED_PROXIES contains invalid IP or CIDR %q", proxy)
			}
		}
		proxies = append(proxies, proxy)
	}
	return proxies, nil
}

func ParseLogLevel(value string) (slog.Level, error) {
	var level slog.Level
	if err := level.UnmarshalText([]byte(strings.TrimSpace(value))); err != nil {
		return slog.LevelInfo, fmt.Errorf("invalid BIODAYS_LOG_LEVEL %q: %w", value, err)
	}
	return level, nil
}

// withSiteOrigin keeps the browser's same-origin requests (which still carry
// an Origin header) from being rejected by the CORS middleware.
func withSiteOrigin(origins []string, siteURL string) []string {
	for _, origin := range origins {
		if origin == "*" {
			return origins
		}
	}
	parsed, err := url.Parse(siteURL)
	if err != nil || parsed.Host == "" {
		return origins
	}
	siteOrigin := parsed.Scheme + "://" + parsed.Host
	for _, origin := range origins {
		if origin == siteOrigin {
			return origins
		}
	}
	return append(origins, siteOrigin)
}

func normalizeSiteURL(cfg *Config) error {
	value := strings.TrimRight(strings.TrimSpace(cfg.SiteURL), "/")
	if value == "" {
		value = "http://localhost:3000"
	}
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("BIODAYS_SITE_URL must be an absolute http(s) origin, got %q", cfg.SiteURL)
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("BIODAYS_SITE_URL must not contain a query or fragment")
	}
	cfg.SiteURL = value
	return nil
}

func validateFeedbackConfig(cfg Config) error {
	if !cfg.FeedbackEnabled && !cfg.SMTPEnabled {
		return nil
	}
	if cfg.FeedbackEnabled {
		if strings.TrimSpace(cfg.FeedbackDBPath) == "" {
			return fmt.Errorf("BIODAYS_FEEDBACK_DB_PATH is required when feedback is enabled")
		}
		if strings.TrimSpace(cfg.AdminUsername) == "" {
			return fmt.Errorf("BIODAYS_ADMIN_USERNAME is required when feedback is enabled")
		}
		if strings.TrimSpace(cfg.AdminPasswordHash) == "" {
			return fmt.Errorf("BIODAYS_ADMIN_PASSWORD_HASH is required when feedback is enabled")
		}
		if _, err := bcrypt.Cost([]byte(cfg.AdminPasswordHash)); err != nil {
			return fmt.Errorf("BIODAYS_ADMIN_PASSWORD_HASH is not a valid bcrypt hash: %w", err)
		}
		if len(cfg.AbuseKeySecret) < 32 {
			return fmt.Errorf("BIODAYS_ABUSE_KEY_SECRET must be at least 32 bytes")
		}
		if cfg.AdminSessionTTL <= 0 {
			return fmt.Errorf("BIODAYS_ADMIN_SESSION_TTL must be positive")
		}
		if cfg.SubmitHourlyLimit <= 0 || cfg.SubmitDailyLimit <= 0 || cfg.ResendHourlyLimit <= 0 || cfg.ResendDailyLimit <= 0 {
			return fmt.Errorf("feedback rate limits must be positive")
		}
	}
	if cfg.SMTPEnabled {
		if missing := cfg.MissingSMTPConfig(); len(missing) > 0 {
			return fmt.Errorf("SMTP is enabled but missing: %s", strings.Join(missing, ", "))
		}
		if cfg.SMTPPort < 1 || cfg.SMTPPort > 65535 {
			return fmt.Errorf("BIODAYS_SMTP_PORT must be between 1 and 65535")
		}
		if cfg.SMTPTimeout <= 0 {
			return fmt.Errorf("BIODAYS_SMTP_TIMEOUT must be positive")
		}
		if cfg.SMTPUsername != "" && cfg.SMTPPassword == "" {
			return fmt.Errorf("BIODAYS_SMTP_PASSWORD is required when BIODAYS_SMTP_USERNAME is set")
		}
	}
	return nil
}

// MissingSMTPConfig lists the required values that are absent. SMTP reachability
// is never part of site readiness; this is configuration only.
func (c Config) MissingSMTPConfig() []string {
	missing := make([]string, 0, 4)
	if strings.TrimSpace(c.SMTPHost) == "" {
		missing = append(missing, "BIODAYS_SMTP_HOST")
	}
	if c.SMTPTLSMode != "starttls" && c.SMTPTLSMode != "implicit_tls" {
		missing = append(missing, "BIODAYS_SMTP_TLS_MODE")
	}
	if !validEmail(c.SMTPFrom) {
		missing = append(missing, "BIODAYS_SMTP_FROM")
	}
	if !validEmail(c.FeedbackNotifyTo) {
		missing = append(missing, "BIODAYS_FEEDBACK_NOTIFY_TO")
	}
	return missing
}

func validEmail(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	parsed, err := mail.ParseAddress(trimmed)
	return err == nil && parsed.Address == trimmed
}
