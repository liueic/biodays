package config_test

import (
	"strings"
	"testing"

	"biodays/server/internal/config"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func enableFeedback(t *testing.T) {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-horse-123"), bcrypt.MinCost)
	require.NoError(t, err)
	t.Setenv("BIODAYS_FEEDBACK_ENABLED", "true")
	t.Setenv("BIODAYS_ADMIN_USERNAME", "admin")
	t.Setenv("BIODAYS_ADMIN_PASSWORD_HASH", string(hash))
	t.Setenv("BIODAYS_ABUSE_KEY_SECRET", strings.Repeat("a", 32))
}

func TestLoadTrustedProxies(t *testing.T) {
	t.Setenv("BIODAYS_TRUSTED_PROXIES", " 127.0.0.1 , 10.0.0.0/8, ,::1 ")

	cfg, err := config.Load()
	require.NoError(t, err)
	require.Equal(t, []string{"127.0.0.1", "10.0.0.0/8", "::1"}, cfg.TrustedProxies)
}

func TestLoadEmptyTrustedProxies(t *testing.T) {
	t.Setenv("BIODAYS_TRUSTED_PROXIES", "")

	cfg, err := config.Load()
	require.NoError(t, err)
	require.Empty(t, cfg.TrustedProxies)
}

func TestLoadAddsSiteOriginToCORS(t *testing.T) {
	t.Setenv("BIODAYS_SITE_URL", "https://biodays.example.com/")
	t.Setenv("BIODAYS_CORS_ORIGINS", "http://localhost:3000")

	cfg, err := config.Load()
	require.NoError(t, err)
	require.Equal(t, "https://biodays.example.com", cfg.SiteURL)
	require.Contains(t, cfg.CORSOrigins, "https://biodays.example.com")
	require.Contains(t, cfg.CORSOrigins, "http://localhost:3000")
}

func TestLoadKeepsWildcardCORS(t *testing.T) {
	t.Setenv("BIODAYS_SITE_URL", "https://biodays.example.com")
	t.Setenv("BIODAYS_CORS_ORIGINS", "*")

	cfg, err := config.Load()
	require.NoError(t, err)
	require.Equal(t, []string{"*"}, cfg.CORSOrigins)
}

func TestLoadRejectsInvalidTrustedProxy(t *testing.T) {
	t.Setenv("BIODAYS_TRUSTED_PROXIES", "not-an-ip")

	_, err := config.Load()
	require.ErrorContains(t, err, "BIODAYS_TRUSTED_PROXIES")
}

func TestAntiSpamDefaults(t *testing.T) {
	enableFeedback(t)
	cfg, err := config.Load()
	require.NoError(t, err)
	require.True(t, cfg.PowEnabled)
	require.Equal(t, 4, cfg.PowDifficulty)
	require.False(t, cfg.TurnstileEnabled)
	require.True(t, cfg.SpamEnabled)
	require.Equal(t, 30, cfg.SpamReviewScore)
	require.Equal(t, 70, cfg.SpamBlockScore)
	require.False(t, cfg.AISpamEnabled)
	require.Equal(t, "@cf/cloudflare/clef-flash", cfg.AISpamModel)
	require.Equal(t, "decision", cfg.AISpamStyle)
}

func TestLoadRejectsInvalidPowDifficulty(t *testing.T) {
	enableFeedback(t)
	t.Setenv("BIODAYS_FEEDBACK_POW_DIFFICULTY", "9")
	_, err := config.Load()
	require.ErrorContains(t, err, "BIODAYS_FEEDBACK_POW_DIFFICULTY")
}

func TestLoadRejectsTurnstileWithoutKeys(t *testing.T) {
	enableFeedback(t)
	t.Setenv("BIODAYS_FEEDBACK_TURNSTILE_ENABLED", "true")
	_, err := config.Load()
	require.ErrorContains(t, err, "Turnstile is enabled")
}

func TestLoadRejectsAIWithoutCredentials(t *testing.T) {
	enableFeedback(t)
	t.Setenv("BIODAYS_AI_SPAM_ENABLED", "true")
	_, err := config.Load()
	require.ErrorContains(t, err, "AI spam classification is enabled")
}

func TestLoadRejectsInvalidAIStyle(t *testing.T) {
	enableFeedback(t)
	t.Setenv("BIODAYS_AI_SPAM_ENABLED", "true")
	t.Setenv("BIODAYS_CF_ACCOUNT_ID", "account")
	t.Setenv("BIODAYS_CF_API_TOKEN", "token")
	t.Setenv("BIODAYS_AI_SPAM_STYLE", "magic")
	_, err := config.Load()
	require.ErrorContains(t, err, "BIODAYS_AI_SPAM_STYLE")
}

func TestLoadRejectsInvalidDBDSN(t *testing.T) {
	t.Setenv("BIODAYS_DB_DSN", "mysql://not-supported")
	_, err := config.Load()
	require.ErrorContains(t, err, "BIODAYS_DB_DSN")
}

func TestLoadAcceptsPostgresDSNAndShortAdminTokenFails(t *testing.T) {
	t.Setenv("BIODAYS_DB_DSN", "postgres://user:pass@db:5432/biodays")
	cfg, err := config.Load()
	require.NoError(t, err)
	require.True(t, cfg.UsingPostgres())

	t.Setenv("BIODAYS_ADMIN_API_TOKEN", "short")
	_, err = config.Load()
	require.ErrorContains(t, err, "BIODAYS_ADMIN_API_TOKEN")
}
