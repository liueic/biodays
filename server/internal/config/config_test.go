package config_test

import (
	"strings"
	"testing"

	"biodays/server/internal/config"
	"biodays/server/internal/settings"

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
	require.False(t, cfg.TurnstileEnabled)
	require.True(t, cfg.SpamEnabled)
	require.Equal(t, 30, cfg.SpamReviewScore)
	require.Equal(t, 70, cfg.SpamBlockScore)
	require.False(t, cfg.AISpamEnabled)
	require.Equal(t, "@cf/cloudflare/clef-flash", cfg.AISpamModel)
	require.Equal(t, "decision", cfg.AISpamStyle)
}

func TestSettingsSeedCarriesEnvironmentValues(t *testing.T) {
	enableFeedback(t)
	t.Setenv("BIODAYS_FEEDBACK_SUBMIT_HOURLY", "7")
	t.Setenv("BIODAYS_FEEDBACK_SPAM_REVIEW_SCORE", "25")
	cfg, err := config.Load()
	require.NoError(t, err)
	seed := cfg.SettingsSeed()
	require.Equal(t, 7, seed.RateLimits.SubmitHourly)
	require.Equal(t, 25, seed.Spam.ReviewScore)
	require.Equal(t, settings.DocumentVersion, seed.Version)
}

func TestSettingsEncryptionKeyValidation(t *testing.T) {
	enableFeedback(t)
	t.Setenv("BIODAYS_SETTINGS_ENC_KEY", "not-base64!!")
	_, err := config.Load()
	require.ErrorContains(t, err, "BIODAYS_SETTINGS_ENC_KEY")

	t.Setenv("BIODAYS_SETTINGS_ENC_KEY", "aGVsbG8=") // valid base64, wrong length
	_, err = config.Load()
	require.ErrorContains(t, err, "32 bytes")

	t.Setenv("BIODAYS_SETTINGS_ENC_KEY", "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=") // 32 bytes
	_, err = config.Load()
	require.NoError(t, err)
}

func TestSettingsSeedValidationRejectsInvalidAIStyle(t *testing.T) {
	enableFeedback(t)
	t.Setenv("BIODAYS_AI_SPAM_ENABLED", "true")
	t.Setenv("BIODAYS_CF_ACCOUNT_ID", "account")
	t.Setenv("BIODAYS_CF_API_TOKEN", "token")
	t.Setenv("BIODAYS_AI_SPAM_STYLE", "magic")
	cfg, err := config.Load()
	require.NoError(t, err) // env load defers group validation to the seed document
	seed := cfg.SettingsSeed()
	seed.AISpam.APIToken = "v1:plaintext-placeholder" // pretend the manager encrypted it
	err = settings.Validate(seed, true)
	require.Error(t, err)
	var validation *settings.ValidationError
	require.ErrorAs(t, err, &validation)
	require.Equal(t, "ai_spam.style", validation.Field)
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

func TestFeedbackConfigWithoutAdminAccount(t *testing.T) {
	// Fresh deployment: no admin env values, no abuse secret. Both are now
	// optional — the account is created with `biodays admin set-password` and
	// the secret is generated and persisted at startup.
	t.Setenv("BIODAYS_FEEDBACK_ENABLED", "true")

	_, err := config.Load()
	require.NoError(t, err)
}

func TestFeedbackConfigRejectsHalfAdminEnv(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-horse-123"), bcrypt.MinCost)
	require.NoError(t, err)
	t.Setenv("BIODAYS_FEEDBACK_ENABLED", "true")
	t.Setenv("BIODAYS_ADMIN_PASSWORD_HASH", string(hash))

	_, err = config.Load()
	require.ErrorContains(t, err, "must be set together")
}

func TestFeedbackConfigRejectsBadHash(t *testing.T) {
	t.Setenv("BIODAYS_FEEDBACK_ENABLED", "true")
	t.Setenv("BIODAYS_ADMIN_USERNAME", "admin")
	t.Setenv("BIODAYS_ADMIN_PASSWORD_HASH", "not-a-bcrypt-hash")

	_, err := config.Load()
	require.ErrorContains(t, err, "not a valid bcrypt hash")
}

func TestFeedbackConfigRejectsShortAbuseSecret(t *testing.T) {
	t.Setenv("BIODAYS_FEEDBACK_ENABLED", "true")
	t.Setenv("BIODAYS_ABUSE_KEY_SECRET", "too-short")

	_, err := config.Load()
	require.ErrorContains(t, err, "BIODAYS_ABUSE_KEY_SECRET must be at least 32 bytes")
}
