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
	Port    int    `env:"BIODAYS_PORT" envDefault:"8080"`
	DBPath  string `env:"BIODAYS_DB_PATH" envDefault:"./biodays.db"`
	DataDir string `env:"BIODAYS_DATA_DIR"`
	// DBDSN switches both story and feedback storage to one PostgreSQL
	// database when set; the SQLite file paths are then ignored.
	DBDSN string `env:"BIODAYS_DB_DSN"`
	// AdminAPIToken is an optional static bearer token that grants
	// story-management access only (programmatic publishing).
	AdminAPIToken  string   `env:"BIODAYS_ADMIN_API_TOKEN"`
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

	ConfirmHourlyLimit   int           `env:"BIODAYS_FEEDBACK_CONFIRM_HOURLY" envDefault:"1"`
	ConfirmDailyLimit    int           `env:"BIODAYS_FEEDBACK_CONFIRM_DAILY" envDefault:"3"`
	ConfirmDailyGlobal   int           `env:"BIODAYS_FEEDBACK_CONFIRM_DAILY_GLOBAL" envDefault:"100"`
	AdminMailHourlyLimit int           `env:"BIODAYS_FEEDBACK_ADMIN_MAIL_HOURLY" envDefault:"10"`
	PowEnabled           bool          `env:"BIODAYS_FEEDBACK_POW_ENABLED" envDefault:"true"`
	PowDifficulty        int           `env:"BIODAYS_FEEDBACK_POW_DIFFICULTY" envDefault:"4"`
	PowTTL               time.Duration `env:"BIODAYS_FEEDBACK_POW_TTL" envDefault:"10m"`
	PowMinSeconds        int           `env:"BIODAYS_FEEDBACK_POW_MIN_SECONDS" envDefault:"3"`
	TurnstileEnabled     bool          `env:"BIODAYS_FEEDBACK_TURNSTILE_ENABLED" envDefault:"false"`
	TurnstileSiteKey     string        `env:"BIODAYS_TURNSTILE_SITE_KEY"`
	TurnstileSecretKey   string        `env:"BIODAYS_TURNSTILE_SECRET_KEY"`
	TurnstileHostnames   []string      `env:"BIODAYS_TURNSTILE_HOSTNAMES" envSeparator:","`
	TurnstileTimeout     time.Duration `env:"BIODAYS_TURNSTILE_TIMEOUT" envDefault:"3s"`
	SpamEnabled          bool          `env:"BIODAYS_FEEDBACK_SPAM_ENABLED" envDefault:"true"`
	SpamBlockScore       int           `env:"BIODAYS_FEEDBACK_SPAM_BLOCK_SCORE" envDefault:"70"`
	SpamReviewScore      int           `env:"BIODAYS_FEEDBACK_SPAM_REVIEW_SCORE" envDefault:"30"`
	AISpamEnabled        bool          `env:"BIODAYS_AI_SPAM_ENABLED" envDefault:"false"`
	CFAccountID          string        `env:"BIODAYS_CF_ACCOUNT_ID"`
	CFAPIToken           string        `env:"BIODAYS_CF_API_TOKEN"`
	AISpamModel          string        `env:"BIODAYS_AI_SPAM_MODEL" envDefault:"@cf/cloudflare/clef-flash"`
	AISpamStyle          string        `env:"BIODAYS_AI_SPAM_STYLE" envDefault:"decision"`
	AISpamTimeout        time.Duration `env:"BIODAYS_AI_SPAM_TIMEOUT" envDefault:"2s"`
	AISpamThreshold      float64       `env:"BIODAYS_AI_SPAM_THRESHOLD" envDefault:"0.8"`
	AISpamDailyLimit     int           `env:"BIODAYS_AI_SPAM_DAILY_LIMIT" envDefault:"500"`
	AISpamMaxChars       int           `env:"BIODAYS_AI_SPAM_MAX_CHARS" envDefault:"4000"`

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
	cfg.TurnstileHostnames = normalizeHostnames(cfg.TurnstileHostnames)

	if _, err := ParseLogLevel(cfg.LogLevel); err != nil {
		return Config{}, err
	}
	if err := normalizeSiteURL(&cfg); err != nil {
		return Config{}, err
	}
	cfg.CORSOrigins = withSiteOrigin(cfg.CORSOrigins, cfg.SiteURL)
	if err := validateStorageConfig(cfg); err != nil {
		return Config{}, err
	}
	if err := validateFeedbackConfig(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func validateStorageConfig(cfg Config) error {
	if strings.TrimSpace(cfg.DBDSN) == "" {
		// nothing more to check for SQLite mode
	} else if !strings.HasPrefix(cfg.DBDSN, "postgres://") && !strings.HasPrefix(cfg.DBDSN, "postgresql://") {
		return fmt.Errorf("BIODAYS_DB_DSN must start with postgres:// or postgresql://")
	}
	if cfg.AdminAPIToken != "" && len(cfg.AdminAPIToken) < 32 {
		return fmt.Errorf("BIODAYS_ADMIN_API_TOKEN must be at least 32 bytes when set")
	}
	return nil
}

// UsingPostgres reports whether storage runs on PostgreSQL via BIODAYS_DB_DSN.
func (c Config) UsingPostgres() bool { return strings.TrimSpace(c.DBDSN) != "" }

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

func normalizeHostnames(values []string) []string {
	hosts := make([]string, 0, len(values))
	for _, value := range values {
		host := strings.ToLower(strings.TrimSpace(value))
		if host != "" {
			hosts = append(hosts, host)
		}
	}
	return hosts
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
		// The admin account lives in the database and is created with
		// `biodays admin set-password`. The environment variables below remain
		// as an optional first-boot import for existing deployments.
		usernameSet := strings.TrimSpace(cfg.AdminUsername) != ""
		hashSet := strings.TrimSpace(cfg.AdminPasswordHash) != ""
		if usernameSet != hashSet {
			return fmt.Errorf("BIODAYS_ADMIN_USERNAME and BIODAYS_ADMIN_PASSWORD_HASH must be set together")
		}
		if hashSet {
			if _, err := bcrypt.Cost([]byte(cfg.AdminPasswordHash)); err != nil {
				return fmt.Errorf("BIODAYS_ADMIN_PASSWORD_HASH is not a valid bcrypt hash: %w", err)
			}
		}
		// BIODAYS_ABUSE_KEY_SECRET is optional: when empty, a random key is
		// generated once and persisted in the database.
		if cfg.AbuseKeySecret != "" && len(cfg.AbuseKeySecret) < 32 {
			return fmt.Errorf("BIODAYS_ABUSE_KEY_SECRET must be at least 32 bytes")
		}
		if cfg.AdminSessionTTL <= 0 {
			return fmt.Errorf("BIODAYS_ADMIN_SESSION_TTL must be positive")
		}
		if cfg.SubmitHourlyLimit <= 0 || cfg.SubmitDailyLimit <= 0 || cfg.ResendHourlyLimit <= 0 || cfg.ResendDailyLimit <= 0 {
			return fmt.Errorf("feedback rate limits must be positive")
		}
		if cfg.ConfirmHourlyLimit <= 0 || cfg.ConfirmDailyLimit <= 0 || cfg.ConfirmDailyGlobal <= 0 || cfg.AdminMailHourlyLimit <= 0 {
			return fmt.Errorf("feedback mail limits must be positive")
		}
		if cfg.SpamReviewScore < 1 || cfg.SpamBlockScore < 50 || cfg.SpamBlockScore > 100 || cfg.SpamReviewScore >= cfg.SpamBlockScore {
			return fmt.Errorf("feedback spam score thresholds are invalid")
		}
		if cfg.PowEnabled {
			if cfg.PowDifficulty < 3 || cfg.PowDifficulty > 5 {
				return fmt.Errorf("BIODAYS_FEEDBACK_POW_DIFFICULTY must be between 3 and 5")
			}
			if cfg.PowTTL < time.Minute || cfg.PowTTL > time.Hour {
				return fmt.Errorf("BIODAYS_FEEDBACK_POW_TTL must be between 1m and 1h")
			}
			if cfg.PowMinSeconds < 0 || cfg.PowMinSeconds > 30 {
				return fmt.Errorf("BIODAYS_FEEDBACK_POW_MIN_SECONDS must be between 0 and 30")
			}
		}
		if cfg.TurnstileEnabled {
			if strings.TrimSpace(cfg.TurnstileSiteKey) == "" || strings.TrimSpace(cfg.TurnstileSecretKey) == "" {
				return fmt.Errorf("Turnstile is enabled but BIODAYS_TURNSTILE_SITE_KEY or BIODAYS_TURNSTILE_SECRET_KEY is missing")
			}
			if cfg.TurnstileTimeout < time.Second || cfg.TurnstileTimeout > 10*time.Second {
				return fmt.Errorf("BIODAYS_TURNSTILE_TIMEOUT must be between 1s and 10s")
			}
		}
		if cfg.AISpamEnabled {
			if !cfg.SpamEnabled {
				return fmt.Errorf("BIODAYS_AI_SPAM_ENABLED requires BIODAYS_FEEDBACK_SPAM_ENABLED=true")
			}
			if strings.TrimSpace(cfg.CFAccountID) == "" || strings.TrimSpace(cfg.CFAPIToken) == "" || strings.TrimSpace(cfg.AISpamModel) == "" {
				return fmt.Errorf("AI spam classification is enabled but Cloudflare account ID, API token, or model is missing")
			}
			if cfg.AISpamStyle != "decision" && cfg.AISpamStyle != "chat" {
				return fmt.Errorf("BIODAYS_AI_SPAM_STYLE must be decision or chat")
			}
			if cfg.AISpamTimeout < time.Second || cfg.AISpamTimeout > 10*time.Second {
				return fmt.Errorf("BIODAYS_AI_SPAM_TIMEOUT must be between 1s and 10s")
			}
			if cfg.AISpamThreshold < 0.5 || cfg.AISpamThreshold > 1 {
				return fmt.Errorf("BIODAYS_AI_SPAM_THRESHOLD must be between 0.5 and 1")
			}
			if cfg.AISpamDailyLimit <= 0 || cfg.AISpamDailyLimit > 10000 {
				return fmt.Errorf("BIODAYS_AI_SPAM_DAILY_LIMIT must be between 1 and 10000")
			}
			if cfg.AISpamMaxChars < 500 || cfg.AISpamMaxChars > 8000 {
				return fmt.Errorf("BIODAYS_AI_SPAM_MAX_CHARS must be between 500 and 8000")
			}
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
