package settings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"biodays/server/internal/antispam"
	"biodays/server/internal/mailer"
	"biodays/server/internal/model"
)

// SettingsDocKey is the app_settings key holding the runtime settings JSON.
const SettingsDocKey = "runtime_settings"

// ErrRevisionConflict reports a concurrent edit: the caller based the patch
// on a revision older than the stored document.
var ErrRevisionConflict = errors.New("settings were modified by another request")

// ErrSecretUnwritable reports a save that would store secrets without the
// encryption key being configured.
var ErrSecretUnwritable = errors.New("secrets cannot be saved without BIODAYS_SETTINGS_ENC_KEY")

// Store is the persistence surface the manager needs. FeedbackStore
// satisfies it.
type Store interface {
	GetAppSetting(ctx context.Context, key string) (string, error)
	SetAppSetting(ctx context.Context, key, value string, now time.Time) error
	// SaveSettingsDocument persists the JSON and writes the audit record in
	// one transaction so revisions and history can never diverge.
	SaveSettingsDocument(ctx context.Context, key, value string, record *model.SettingsAudit, now time.Time) error
}

// JudgeStore is the slice of FeedbackStore the antispam Judge needs.
type JudgeStore = antispam.JudgeStore

// snapshot bundles the document with the derived clients rebuilt on every
// change. Readers take the whole struct atomically.
type Snapshot struct {
	Document Document

	SMTPSender mailer.Sender
	SMTPError  string // why delivery is paused; empty when healthy
	Turnstile  antispam.TurnstileVerifier
	SpamJudge  *antispam.Judge

	// Decrypted once per snapshot so request paths never re-run AES.
	SMTPPassword    string
	TurnstileSecret string
	AIToken         string
}

type Manager struct {
	store    Store
	logger   *slog.Logger
	encKey   []byte // nil when encryption is not configured
	now      func() time.Time
	snapshot atomic.Pointer[Snapshot]
}

func NewManager(st Store, encKey []byte, logger *slog.Logger, now func() time.Time) *Manager {
	if now == nil {
		now = time.Now
	}
	return &Manager{store: st, encKey: encKey, logger: logger, now: now}
}

// Load reads (or seeds) the document and publishes the first snapshot.
// seed is consulted only when the database has no runtime_settings key yet;
// afterwards the database is the single source of truth.
func (m *Manager) Load(ctx context.Context, seed Document, judgeStore JudgeStore) error {
	raw, err := m.store.GetAppSetting(ctx, SettingsDocKey)
	if err != nil {
		return err
	}
	doc := seed
	if strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &doc); err != nil {
			return fmt.Errorf("parse runtime settings: %w", err)
		}
	} else {
		// First boot: normalize environment-provided secrets into ciphertext
		// (or drop them when encryption is unavailable), then persist so
		// later environment changes cannot silently win.
		if err := m.encryptOrDropSecrets(&doc); err != nil {
			return err
		}
		doc.Version = DocumentVersion
		encoded, err := json.Marshal(doc)
		if err != nil {
			return err
		}
		if err := m.store.SetAppSetting(ctx, SettingsDocKey, string(encoded), m.now()); err != nil {
			return err
		}
		m.logger.Info("runtime settings seeded from environment", "revision", doc.Revision)
	}
	snap, err := m.buildSnapshot(doc, judgeStore)
	if err != nil {
		return err
	}
	m.snapshot.Store(snap)
	return nil
}

// encryptOrDropSecrets handles first-boot seeding: with a key the plaintext
// environment values become ciphertext; without one they are dropped (the
// admin re-enters them in the UI) so plaintext never lands in the database.
func (m *Manager) encryptOrDropSecrets(doc *Document) error {
	if len(m.encKey) == 0 {
		doc.SMTP.Password = ""
		doc.Turnstile.SecretKey = ""
		doc.AISpam.APIToken = ""
		return nil
	}
	return doc.EncryptSecrets(m.encKey)
}

// Snapshot returns the current immutable snapshot. It never fails; before
// Load it returns a zero snapshot.
func (m *Manager) Snapshot() Snapshot {
	if snap := m.snapshot.Load(); snap != nil {
		return *snap
	}
	return Snapshot{}
}

func (m *Manager) HasEncryptionKey() bool { return len(m.encKey) > 0 }

// Sender returns the current snapshot's SMTP sender for the mail worker.
func (m *Manager) Sender() mailer.Sender {
	sender := m.Snapshot().SMTPSender
	if sender == nil {
		return mailer.PausedSender{Reason: "settings are not loaded yet"}
	}
	return sender
}

// PatchInput carries an update. Nil groups mean "leave unchanged".
type PatchInput struct {
	BaseRevision  int
	Actor         string
	Subscriptions *bool
	SMTP          *SMTPGroup
	RateLimits    *RateLimitsGroup
	Turnstile     *TurnstileGroup
	Spam          *SpamGroup
	AISpam        *AISpamGroup
}

// Update applies the patch: merge onto the current document, apply the
// write-only secret semantics, encrypt, validate, persist with a revision
// bump, rebuild derived clients, publish and audit.
func (m *Manager) Update(ctx context.Context, in PatchInput, judgeStore JudgeStore) (Document, error) {
	current := m.Snapshot().Document
	if in.BaseRevision != current.Revision {
		return Document{}, ErrRevisionConflict
	}

	merged := current.Clone()
	if in.Subscriptions != nil {
		merged.Subscriptions = *in.Subscriptions
	}
	if in.SMTP != nil {
		merged.SMTP = mergeSMTP(current.SMTP, *in.SMTP)
	}
	if in.RateLimits != nil {
		merged.RateLimits = *in.RateLimits
	}
	if in.Turnstile != nil {
		merged.Turnstile = mergeTurnstile(current.Turnstile, *in.Turnstile)
	}
	if in.Spam != nil {
		merged.Spam = *in.Spam
	}
	if in.AISpam != nil {
		merged.AISpam = mergeAISpam(current.AISpam, *in.AISpam)
	}

	if err := m.resolveSecrets(&merged, current); err != nil {
		return Document{}, err
	}
	if err := Validate(merged, m.HasEncryptionKey()); err != nil {
		return Document{}, err
	}

	now := m.now().UTC()
	merged.Version = DocumentVersion
	merged.Revision = current.Revision + 1
	merged.UpdatedAt = now
	merged.UpdatedBy = in.Actor

	encoded, err := json.Marshal(merged)
	if err != nil {
		return Document{}, err
	}
	record := auditRecord(current, merged, in.Actor, now)
	if err := m.store.SaveSettingsDocument(ctx, SettingsDocKey, string(encoded), record, now); err != nil {
		return Document{}, err
	}

	snap, err := m.buildSnapshot(merged, judgeStore)
	if err != nil {
		return Document{}, err
	}
	m.snapshot.Store(snap)
	m.logger.Info("runtime settings updated", "revision", merged.Revision, "actor", in.Actor)
	return merged.Clone(), nil
}

// resolveSecrets turns write-only inputs into stored ciphertext: plaintext
// is encrypted, empty keeps the old value, the sentinel clears it.
func (m *Manager) resolveSecrets(merged *Document, current Document) error {
	resolve := func(incoming, existing string) (string, error) {
		trimmed := strings.TrimSpace(incoming)
		switch {
		case trimmed == "":
			return existing, nil
		case trimmed == ClearSentinel:
			return "", nil
		case isCiphertext(trimmed):
			return trimmed, nil // already-encrypted value echoed back unchanged
		default:
			if len(m.encKey) == 0 {
				return "", ErrSecretUnwritable
			}
			return encrypt(m.encKey, trimmed)
		}
	}
	password, err := resolve(merged.SMTP.Password, current.SMTP.Password)
	if err != nil {
		return fmt.Errorf("smtp.password: %w", err)
	}
	merged.SMTP.Password = password
	// Clearing the password also drops the username: a relay never
	// authenticates with a user but no password.
	if merged.SMTP.Password == "" {
		merged.SMTP.Username = ""
	}
	secret, err := resolve(merged.Turnstile.SecretKey, current.Turnstile.SecretKey)
	if err != nil {
		return fmt.Errorf("turnstile.secret_key: %w", err)
	}
	merged.Turnstile.SecretKey = secret
	token, err := resolve(merged.AISpam.APIToken, current.AISpam.APIToken)
	if err != nil {
		return fmt.Errorf("ai_spam.api_token: %w", err)
	}
	merged.AISpam.APIToken = token
	return nil
}

// buildSnapshot rebuilds the derived clients from the document. Construction
// failures degrade instead of failing: an invalid SMTP config produces a
// paused sender carrying the reason, so mail stays queued and the admin sees
// the error in the mail status.
func (m *Manager) buildSnapshot(doc Document, judgeStore JudgeStore) (*Snapshot, error) {
	snap := &Snapshot{Document: doc}

	if len(m.encKey) > 0 {
		password, secret, token, err := doc.SecretPlaintexts(m.encKey)
		if err != nil {
			// A key rotation or data issue made a secret unreadable. Keep
			// the document working for everything else and surface it.
			snap.SMTPError = err.Error()
			m.logger.Error("decrypt settings secrets", "error", err)
		} else {
			snap.SMTPPassword = password
			snap.TurnstileSecret = secret
			snap.AIToken = token
		}
	}

	snap.SMTPSender = mailer.PausedSender{Reason: "smtp is disabled"}
	if doc.SMTP.Enabled {
		reason := snap.SMTPError
		needsPassword := strings.TrimSpace(doc.SMTP.Username) != ""
		if reason == "" && needsPassword && snap.SMTPPassword == "" {
			reason = "SMTP password is not configured"
		}
		if reason != "" {
			snap.SMTPError = reason
			snap.SMTPSender = mailer.PausedSender{Reason: reason}
		} else {
			sender, err := mailer.NewSMTPSender(mailer.SMTPConfig{
				Host: doc.SMTP.Host, Port: doc.SMTP.Port, TLSMode: doc.SMTP.TLSMode,
				Username: doc.SMTP.Username, Password: snap.SMTPPassword,
				From: doc.SMTP.From, Timeout: doc.SMTP.Timeout(),
			})
			if err != nil {
				snap.SMTPError = err.Error()
				snap.SMTPSender = mailer.PausedSender{Reason: err.Error()}
			} else {
				snap.SMTPSender = sender
			}
		}
	}

	if doc.Turnstile.Enabled && snap.TurnstileSecret != "" {
		snap.Turnstile = antispam.NewTurnstileClient(antispam.TurnstileConfig{
			SecretKey: snap.TurnstileSecret, Hostnames: doc.Turnstile.Hostnames,
			Timeout: doc.Turnstile.Timeout(),
		})
	}

	if doc.Spam.Enabled && judgeStore != nil {
		snap.SpamJudge = antispam.NewJudge(judgeStore, nil, antispam.Settings{
			Enabled: doc.Spam.Enabled, ReviewScore: doc.Spam.ReviewScore,
			BlockScore:  doc.Spam.BlockScore,
			AIThreshold: doc.AISpam.Threshold, AIDailyLimit: doc.AISpam.DailyLimit,
			AIMaxChars: doc.AISpam.MaxChars,
		}, nil, m.logger, m.now)
		// Attach the classifier when AI review is on and the token decrypted.
		if doc.AISpam.Enabled && snap.AIToken != "" {
			client, err := antispam.NewCloudflareClassifier(antispam.CloudflareConfig{
				AccountID: doc.AISpam.AccountID, APIToken: snap.AIToken,
				Model: doc.AISpam.Model, Style: doc.AISpam.Style, Timeout: doc.AISpam.Timeout(),
			})
			if err != nil {
				m.logger.Error("configure workers ai spam classifier", "error", err)
			} else {
				snap.SpamJudge = antispam.NewJudge(judgeStore, nil, antispam.Settings{
					Enabled: doc.Spam.Enabled, ReviewScore: doc.Spam.ReviewScore,
					BlockScore:  doc.Spam.BlockScore,
					AIThreshold: doc.AISpam.Threshold, AIDailyLimit: doc.AISpam.DailyLimit,
					AIMaxChars: doc.AISpam.MaxChars,
				}, client, m.logger, m.now)
			}
		}
	}
	return snap, nil
}

// mergeSMTP overlays a PATCH body group onto the stored one. All non-secret
// fields come from the request; the secret keeps the stored ciphertext when
// the request left it empty.
func mergeSMTP(current, incoming SMTPGroup) SMTPGroup {
	merged := incoming
	if strings.TrimSpace(incoming.Password) == "" {
		merged.Password = current.Password
	}
	return merged
}

func mergeTurnstile(current, incoming TurnstileGroup) TurnstileGroup {
	merged := incoming
	if strings.TrimSpace(incoming.SecretKey) == "" {
		merged.SecretKey = current.SecretKey
	}
	if incoming.Hostnames == nil {
		merged.Hostnames = current.Hostnames
	}
	return merged
}

func mergeAISpam(current, incoming AISpamGroup) AISpamGroup {
	merged := incoming
	if strings.TrimSpace(incoming.APIToken) == "" {
		merged.APIToken = current.APIToken
	}
	return merged
}

// auditRecord diffs two revisions field by field. Secrets are compared as
// presence booleans and rendered as •••• so the audit trail never carries
// ciphertext (which would be equivalent to plaintext after a key leak).
func auditRecord(before, after Document, actor string, now time.Time) *model.SettingsAudit {
	changes := map[string]any{}
	put := func(field string, oldValue, newValue any) {
		if fmt.Sprint(oldValue) != fmt.Sprint(newValue) {
			changes[field] = map[string]any{"old": oldValue, "new": newValue}
		}
	}
	mask := func(value string) any {
		if value == "" {
			return ""
		}
		return "••••"
	}
	put("subscriptions_enabled", before.Subscriptions, after.Subscriptions)

	put("smtp.enabled", before.SMTP.Enabled, after.SMTP.Enabled)
	put("smtp.host", before.SMTP.Host, after.SMTP.Host)
	put("smtp.port", before.SMTP.Port, after.SMTP.Port)
	put("smtp.tls_mode", before.SMTP.TLSMode, after.SMTP.TLSMode)
	put("smtp.username", before.SMTP.Username, after.SMTP.Username)
	put("smtp.password", mask(before.SMTP.Password), mask(after.SMTP.Password))
	put("smtp.from", before.SMTP.From, after.SMTP.From)
	put("smtp.notify_to", before.SMTP.NotifyTo, after.SMTP.NotifyTo)
	put("smtp.timeout_ms", before.SMTP.TimeoutMS, after.SMTP.TimeoutMS)

	put("rate_limits.submit_hourly", before.RateLimits.SubmitHourly, after.RateLimits.SubmitHourly)
	put("rate_limits.submit_daily", before.RateLimits.SubmitDaily, after.RateLimits.SubmitDaily)
	put("rate_limits.resend_hourly", before.RateLimits.ResendHourly, after.RateLimits.ResendHourly)
	put("rate_limits.resend_daily", before.RateLimits.ResendDaily, after.RateLimits.ResendDaily)
	put("rate_limits.confirm_hourly", before.RateLimits.ConfirmHourly, after.RateLimits.ConfirmHourly)
	put("rate_limits.confirm_daily", before.RateLimits.ConfirmDaily, after.RateLimits.ConfirmDaily)
	put("rate_limits.confirm_daily_global", before.RateLimits.ConfirmDailyGlobal, after.RateLimits.ConfirmDailyGlobal)
	put("rate_limits.admin_mail_hourly", before.RateLimits.AdminMailHourly, after.RateLimits.AdminMailHourly)

	put("turnstile.enabled", before.Turnstile.Enabled, after.Turnstile.Enabled)
	put("turnstile.site_key", before.Turnstile.SiteKey, after.Turnstile.SiteKey)
	put("turnstile.secret_key", mask(before.Turnstile.SecretKey), mask(after.Turnstile.SecretKey))
	put("turnstile.hostnames", strings.Join(before.Turnstile.Hostnames, ","), strings.Join(after.Turnstile.Hostnames, ","))
	put("turnstile.timeout_ms", before.Turnstile.TimeoutMS, after.Turnstile.TimeoutMS)

	put("spam.enabled", before.Spam.Enabled, after.Spam.Enabled)
	put("spam.review_score", before.Spam.ReviewScore, after.Spam.ReviewScore)
	put("spam.block_score", before.Spam.BlockScore, after.Spam.BlockScore)

	put("ai_spam.enabled", before.AISpam.Enabled, after.AISpam.Enabled)
	put("ai_spam.account_id", before.AISpam.AccountID, after.AISpam.AccountID)
	put("ai_spam.api_token", mask(before.AISpam.APIToken), mask(after.AISpam.APIToken))
	put("ai_spam.model", before.AISpam.Model, after.AISpam.Model)
	put("ai_spam.style", before.AISpam.Style, after.AISpam.Style)
	put("ai_spam.timeout_ms", before.AISpam.TimeoutMS, after.AISpam.TimeoutMS)
	put("ai_spam.threshold", before.AISpam.Threshold, after.AISpam.Threshold)
	put("ai_spam.daily_limit", before.AISpam.DailyLimit, after.AISpam.DailyLimit)
	put("ai_spam.max_chars", before.AISpam.MaxChars, after.AISpam.MaxChars)

	encoded, _ := json.Marshal(changes)
	return &model.SettingsAudit{
		Revision: after.Revision, Actor: actor,
		Changes: string(encoded), CreatedAt: now,
	}
}
