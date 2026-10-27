package settings_test

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"biodays/server/internal/settings"
	"biodays/server/internal/store"

	"github.com/stretchr/testify/require"
)

func testKey(t *testing.T) []byte {
	t.Helper()
	key, err := settings.ParseEncryptionKey("hhJdBRuWnBinBo3N0GboTHpBhGJJ9OdjcD2SOUuCLk0=")
	require.NoError(t, err)
	return key
}

func validDoc() settings.Document {
	return settings.Document{
		Version:       settings.DocumentVersion,
		Subscriptions: true,
		SMTP: settings.SMTPGroup{
			Enabled: true, Host: "smtp.example.com", Port: 587, TLSMode: "starttls",
			Username: "noreply@example.com", From: "noreply@example.com",
			NotifyTo: "admin@example.com", TimeoutMS: 10000,
		},
		RateLimits: settings.RateLimitsGroup{
			SubmitHourly: 5, SubmitDaily: 20, ResendHourly: 3, ResendDaily: 10,
			ConfirmHourly: 1, ConfirmDaily: 3, ConfirmDailyGlobal: 100, AdminMailHourly: 10,
		},
		Spam: settings.SpamGroup{Enabled: true, ReviewScore: 30, BlockScore: 70},
	}
}

func encryptField(t *testing.T, key []byte, plaintext string) string {
	t.Helper()
	value, err := encryptForTest(key, plaintext)
	require.NoError(t, err)
	return value
}

// encryptForTest reaches the package-internal helper through the exported
// EncryptSecrets on a one-field document.
func encryptForTest(key []byte, plaintext string) (string, error) {
	doc := settings.Document{}
	doc.SMTP.Password = plaintext
	if err := doc.EncryptSecrets(key); err != nil {
		return "", err
	}
	return doc.SMTP.Password, nil
}

func TestCryptoRoundTrip(t *testing.T) {
	key := testKey(t)
	value := encryptField(t, key, "p$ssw0rd-with- specials")
	require.True(t, len(value) > len("v1:"))
	require.Contains(t, value, "v1:")

	doc := validDoc()
	doc.SMTP.Password = "p$ssw0rd"
	doc.Turnstile.SecretKey = "0xAAA"
	doc.AISpam.APIToken = "cf-token"
	require.NoError(t, doc.EncryptSecrets(key))
	require.Contains(t, doc.SMTP.Password, "v1:")
	require.Contains(t, doc.Turnstile.SecretKey, "v1:")
	require.Contains(t, doc.AISpam.APIToken, "v1:")

	password, secret, token, err := doc.SecretPlaintexts(key)
	require.NoError(t, err)
	require.Equal(t, "p$ssw0rd", password)
	require.Equal(t, "0xAAA", secret)
	require.Equal(t, "cf-token", token)

	// A different key cannot read the ciphertext.
	wrongRaw := make([]byte, 32)
	for j := range wrongRaw {
		wrongRaw[j] = byte(j) + 1
	}
	wrongKey := wrongRaw
	require.NoError(t, err)
	_, _, _, err = doc.SecretPlaintexts(wrongKey)
	require.ErrorIs(t, err, settings.ErrBadCiphertext)
}

func TestEncryptSecretsIdempotentForCiphertextAndSentinel(t *testing.T) {
	key := testKey(t)
	doc := validDoc()
	doc.SMTP.Password = settings.ClearSentinel
	doc.Turnstile.SecretKey = ""
	doc.AISpam.APIToken = encryptField(t, key, "already-encrypted")
	require.NoError(t, doc.EncryptSecrets(key))
	require.Equal(t, settings.ClearSentinel, doc.SMTP.Password)
	require.Empty(t, doc.Turnstile.SecretKey)
	require.Contains(t, doc.AISpam.APIToken, "v1:")
}

func TestValidateAcceptsValidDocument(t *testing.T) {
	doc := validDoc()
	doc.SMTP.Password = "v1:" + "irrelevant-but-prefixed"
	require.NoError(t, settings.Validate(doc, true))
}

func TestValidateRejectsCiphertextWithoutEncryptionKey(t *testing.T) {
	doc := validDoc()
	doc.SMTP.Password = "v1:ciphertext"
	err := settings.Validate(doc, false)
	var validation *settings.ValidationError
	require.ErrorAs(t, err, &validation)
	require.Equal(t, "smtp.password", validation.Field)
}

func TestValidateRejectsCrossFieldRules(t *testing.T) {
	key := testKey(t)
	cases := []struct {
		name  string
		mutat func(*settings.Document)
		field string
	}{
		{"smtp missing host", func(d *settings.Document) { d.SMTP.Host = "" }, "smtp.host"},
		{"smtp bad port", func(d *settings.Document) { d.SMTP.Port = 70000 }, "smtp.port"},
		{"smtp bad tls mode", func(d *settings.Document) { d.SMTP.TLSMode = "none" }, "smtp.tls_mode"},
		{"smtp bad from", func(d *settings.Document) { d.SMTP.From = "not-an-email" }, "smtp.from"},
		{"smtp username without password", func(d *settings.Document) { d.SMTP.Username = "u"; d.SMTP.Password = "" }, "smtp.password"},
		{"smtp password plaintext", func(d *settings.Document) { d.SMTP.Password = "raw-secret" }, "smtp.password"},
		{"rate limit zero", func(d *settings.Document) { d.RateLimits.SubmitHourly = 0 }, "rate_limits.submit_hourly"},
		{"rate limit too large", func(d *settings.Document) { d.RateLimits.AdminMailHourly = 20000 }, "rate_limits.admin_mail_hourly"},
		{"spam block below fifty", func(d *settings.Document) { d.Spam.BlockScore = 40 }, "spam.review_score"},
		{"spam review equals block", func(d *settings.Document) { d.Spam.ReviewScore = 70 }, "spam.review_score"},
		{"turnstile without site key", func(d *settings.Document) { d.Turnstile.Enabled = true; d.Turnstile.SecretKey = "v1:x" }, "turnstile.site_key"},
		{"turnstile bad timeout", func(d *settings.Document) {
			d.Turnstile.Enabled = true
			d.Turnstile.SiteKey = "site"
			d.Turnstile.SecretKey = "v1:x"
			d.Turnstile.TimeoutMS = 50
		}, "turnstile.timeout_ms"},
		{"ai without spam", func(d *settings.Document) {
			d.Spam.Enabled = false
			d.AISpam.Enabled = true
			d.AISpam.AccountID = "acc"
			d.AISpam.APIToken = "v1:x"
			d.AISpam.Model = "m"
			d.AISpam.Style = "decision"
			d.AISpam.TimeoutMS = 2000
			d.AISpam.Threshold = 0.8
			d.AISpam.DailyLimit = 100
			d.AISpam.MaxChars = 4000
		}, "ai_spam.enabled"},
		{"ai bad threshold", func(d *settings.Document) {
			d.Spam.Enabled = true
			d.AISpam.Enabled = true
			d.AISpam.AccountID = "acc"
			d.AISpam.APIToken = "v1:x"
			d.AISpam.Model = "m"
			d.AISpam.Style = "decision"
			d.AISpam.TimeoutMS = 2000
			d.AISpam.Threshold = 0.2
			d.AISpam.DailyLimit = 100
			d.AISpam.MaxChars = 4000
		}, "ai_spam.threshold"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := validDoc()
			doc.SMTP.Password = encryptField(t, key, "base-password")
			tc.mutat(&doc)
			err := settings.Validate(doc, true)
			var validation *settings.ValidationError
			require.ErrorAs(t, err, &validation)
			require.Equal(t, tc.field, validation.Field)
		})
	}
}

type managerFixture struct {
	manager *settings.Manager
	st      *store.FeedbackStore
}

func newManagerFixture(t *testing.T, encKey []byte, seed settings.Document) *managerFixture {
	t.Helper()
	st, err := store.OpenFeedback(filepath.Join(t.TempDir(), "feedback.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, st.Close()) })
	logger := discardLogger()
	manager := settings.NewManager(st, encKey, logger, time.Now)
	require.NoError(t, manager.Load(t.Context(), seed, st))
	return &managerFixture{manager: manager, st: st}
}

func TestManagerSeedsFirstBootAndDatabaseWinsAfterwards(t *testing.T) {
	key := testKey(t)
	seed := validDoc()
	seed.SMTP.Password = "first-boot-secret"
	f := newManagerFixture(t, key, seed)

	// The seed secret was encrypted before the first persist.
	stored := f.manager.Snapshot().Document
	require.Contains(t, stored.SMTP.Password, "v1:")
	require.Equal(t, 5, stored.RateLimits.SubmitHourly)

	// A second boot with different environment values keeps the DB document.
	seed2 := validDoc()
	seed2.RateLimits.SubmitHourly = 99
	seed2.SMTP.Password = "should-be-ignored"
	manager2 := settings.NewManager(f.st, key, discardLogger(), time.Now)
	require.NoError(t, manager2.Load(t.Context(), seed2, f.st))
	require.Equal(t, 5, manager2.Snapshot().Document.RateLimits.SubmitHourly)
}

func TestManagerSeedingWithoutKeyDropsSecrets(t *testing.T) {
	seed := validDoc()
	seed.SMTP.Password = "plaintext-secret"
	f := newManagerFixture(t, nil, seed)
	require.Empty(t, f.manager.Snapshot().Document.SMTP.Password)

	raw, err := f.st.GetAppSetting(t.Context(), "runtime_settings")
	require.NoError(t, err)
	require.NotContains(t, raw, "plaintext-secret")
}

func TestManagerUpdateHotReloadAndRevisionConflict(t *testing.T) {
	key := testKey(t)
	f := newManagerFixture(t, key, validDoc())

	// Save a new SMTP password: plaintext in, ciphertext out, snapshot live.
	doc, err := f.manager.Update(t.Context(), settings.PatchInput{
		BaseRevision: 0,
		Actor:        "editor",
		SMTP: &settings.SMTPGroup{
			Enabled: true, Host: "smtp2.example.com", Port: 465, TLSMode: "implicit_tls",
			Username: "noreply@example.com", Password: "rotated-secret",
			From: "noreply@example.com", NotifyTo: "admin@example.com", TimeoutMS: 10000,
		},
	}, f.st)
	require.NoError(t, err)
	require.Equal(t, 1, doc.Revision)
	require.Contains(t, doc.SMTP.Password, "v1:")
	require.Equal(t, "smtp2.example.com", f.manager.Snapshot().Document.SMTP.Host)

	password, _, _, err := f.manager.Snapshot().Document.SecretPlaintexts(key)
	require.NoError(t, err)
	require.Equal(t, "rotated-secret", password)

	// Empty password keeps the stored secret; the sentinel clears it.
	kept, err := f.manager.Update(t.Context(), settings.PatchInput{
		BaseRevision: 1, Actor: "editor",
		SMTP: &settings.SMTPGroup{
			Enabled: true, Host: "smtp2.example.com", Port: 465, TLSMode: "implicit_tls",
			Username: "noreply@example.com", Password: "",
			From: "noreply@example.com", NotifyTo: "admin@example.com", TimeoutMS: 10000,
		},
	}, f.st)
	require.NoError(t, err)
	require.Equal(t, doc.SMTP.Password, kept.SMTP.Password)

	cleared, err := f.manager.Update(t.Context(), settings.PatchInput{
		BaseRevision: 2, Actor: "editor",
		SMTP: &settings.SMTPGroup{
			Enabled: true, Host: "smtp2.example.com", Port: 465, TLSMode: "implicit_tls",
			Username: "noreply@example.com", Password: settings.ClearSentinel,
			From: "noreply@example.com", NotifyTo: "admin@example.com", TimeoutMS: 10000,
		},
	}, f.st)
	require.NoError(t, err)
	require.Empty(t, cleared.SMTP.Password)

	// Stale revision is rejected.
	_, err = f.manager.Update(t.Context(), settings.PatchInput{BaseRevision: 0, Actor: "editor"}, f.st)
	require.ErrorIs(t, err, settings.ErrRevisionConflict)
}

func TestManagerUpdateWithoutKeyRejectsNewSecret(t *testing.T) {
	f := newManagerFixture(t, nil, validDoc())
	doc := validDoc()
	doc.SMTP.Password = "brand-new-secret"
	_, err := f.manager.Update(t.Context(), settings.PatchInput{
		BaseRevision: 0, Actor: "editor", SMTP: &doc.SMTP,
	}, f.st)
	require.ErrorIs(t, err, settings.ErrSecretUnwritable)
}

func TestManagerAuditMasksSecrets(t *testing.T) {
	key := testKey(t)
	f := newManagerFixture(t, key, validDoc())
	_, err := f.manager.Update(t.Context(), settings.PatchInput{
		BaseRevision: 0, Actor: "editor",
		SMTP: &settings.SMTPGroup{
			Enabled: true, Host: "smtp.example.com", Port: 587, TLSMode: "starttls",
			Username: "noreply@example.com", Password: "audit-secret",
			From: "noreply@example.com", NotifyTo: "admin@example.com", TimeoutMS: 10000,
		},
	}, f.st)
	require.NoError(t, err)

	records, total, err := f.st.ListSettingsAudit(t.Context(), 1, 20)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, "editor", records[0].Actor)
	require.Contains(t, records[0].Changes, "••••")
	require.NotContains(t, records[0].Changes, "audit-secret")

	var changes map[string]struct {
		Old any `json:"old"`
		New any `json:"new"`
	}
	require.NoError(t, json.Unmarshal([]byte(records[0].Changes), &changes))
	require.Contains(t, changes, "smtp.password")
	require.Equal(t, "", changes["smtp.password"].Old)
	require.Equal(t, "••••", changes["smtp.password"].New)
}
