package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"biodays/server/internal/antispam"
	"biodays/server/internal/mailer"
	"biodays/server/internal/model"
	"biodays/server/internal/security"
	"biodays/server/internal/settings"
	"biodays/server/internal/store"
)

const (
	ConsentVersion = "2026-10-24-1"

	TitleMin     = 1
	TitleMax     = 80
	ContentMin   = 10
	ContentMax   = 5000
	NicknameMax  = 40
	SourceURLMax = 2048
)

var (
	ErrFeedbackDisabled         = errors.New("feedback is not enabled")
	ErrSubscriptionsUnavailable = errors.New("email subscriptions are unavailable")
	ErrRateLimited              = errors.New("rate limited")
	ErrInvalidToken             = store.ErrTokenInvalid
	ErrTokenExpired             = store.ErrTokenExpired
	ErrFeedbackNotFound         = store.ErrFeedbackNotFound
	ErrVersionConflict          = store.ErrVersionConflict
	ErrIdempotencyConflict      = store.ErrRequestConflict
	ErrMailNotFound             = store.ErrOutboxNotFound
	ErrSMTPUnavailable          = errors.New("smtp is not enabled")
	ErrTurnstileRequired        = errors.New("turnstile is required")
	ErrTurnstileFailed          = errors.New("turnstile failed")
	ErrVerificationUnavailable  = errors.New("verification unavailable")
)

// ValidationError maps to a 400 with a field-level message.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

func fieldError(field, message string) error {
	return &ValidationError{Field: field, Message: message}
}

// SettingsSource serves the effective runtime settings. The production
// implementation wraps *settings.Manager (atomic snapshot per read); tests
// use a static source.
type SettingsSource interface {
	// Effective returns the values the feedback flow consumes. Derived
	// clients (Turnstile, spam judge) travel with the values.
	Effective() EffectiveSettings
}

// EffectiveSettings is the flat view of a settings snapshot consumed by the
// feedback flow.
type EffectiveSettings struct {
	Enabled              bool
	SubscriptionsEnabled bool
	SMTPEnabled          bool
	SMTPConfigured       bool
	SMTPMissing          []string
	SMTPError            string
	SiteURL              string
	NotifyTo             string
	RateLimits           settings.RateLimitsGroup
	TurnstileEnabled     bool
	TurnstileSiteKey     string
	TurnstileVerifier    antispam.TurnstileVerifier
	SpamEnabled          bool
	SpamAIAssisted       bool
	SpamJudge            *antispam.Judge
}

// staticSettings is the trivial SettingsSource used by tests and as a
// fallback before the manager loads.
type staticSettings struct {
	values EffectiveSettings
}

func (s staticSettings) Effective() EffectiveSettings { return s.values }

// StaticSettings exposes the fixed source for tests and tooling.
func StaticSettings(values EffectiveSettings) SettingsSource {
	return staticSettings{values: values}
}

type FeedbackService struct {
	store  *store.FeedbackStore
	source SettingsSource
	secret []byte
	now    func() time.Time
	logger *slog.Logger
}

func NewFeedbackService(st *store.FeedbackStore, source SettingsSource, secret []byte, now func() time.Time) *FeedbackService {
	if now == nil {
		now = time.Now
	}
	logger := slog.Default()
	return &FeedbackService{store: st, source: source, secret: secret, now: now, logger: logger}
}

// snapshot reads the effective settings once per call so a request sees a
// consistent view even if an admin saves mid-request.
func (s *FeedbackService) snapshot() EffectiveSettings {
	if s.source == nil {
		return EffectiveSettings{}
	}
	return s.source.Effective()
}

func (s *FeedbackService) subscriptionsAvailable() bool {
	snap := s.snapshot()
	return snap.Enabled && snap.SubscriptionsEnabled && snap.SMTPEnabled
}

func (s *FeedbackService) Options() model.FeedbackOptionsDTO {
	snap := s.snapshot()
	kinds := make([]model.FeedbackKindOption, 0, len(model.FeedbackKinds))
	for _, kind := range model.FeedbackKinds {
		kinds = append(kinds, model.FeedbackKindOption{Value: kind, Label: mailer.KindLabel(kind)})
	}
	return model.FeedbackOptionsDTO{
		Enabled:              snap.Enabled,
		SubscriptionsEnabled: s.subscriptionsAvailable(),
		Kinds:                kinds,
		Limits: model.FeedbackLimits{
			TitleMin: TitleMin, TitleMax: TitleMax,
			ContentMin: ContentMin, ContentMax: ContentMax,
			NicknameMax: NicknameMax, SourceURLMax: SourceURLMax,
		},
		Verification: model.VerificationOptionsDTO{
			TurnstileEnabled: snap.TurnstileEnabled,
			TurnstileSiteKey: snap.TurnstileSiteKey,
			AIAssisted:       snap.SpamAIAssisted,
		},
	}
}

type SubmitInput struct {
	RequestID      string
	Kind           string
	Title          string
	Content        string
	SourceURL      string
	StoryID        string
	Nickname       string
	Email          string
	Subscribe      bool
	Honeypot       string
	TurnstileToken string
}

type SubmitOutcome struct {
	Feedback          model.Feedback
	Created           bool
	ConfirmationEmail string
}

type normalizedSubmission struct {
	Kind      string `json:"kind"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	SourceURL string `json:"source_url"`
	StoryID   string `json:"story_id"`
	Nickname  string `json:"nickname"`
	Email     string `json:"email"`
	Subscribe bool   `json:"subscribe"`
}

func (s *FeedbackService) Submit(ctx context.Context, in SubmitInput, clientIP string) (SubmitOutcome, error) {
	snap := s.snapshot()
	if !snap.Enabled {
		return SubmitOutcome{}, ErrFeedbackDisabled
	}
	normalized, err := normalizeSubmission(in)
	if err != nil {
		return SubmitOutcome{}, err
	}
	if strings.TrimSpace(in.Honeypot) != "" {
		return SubmitOutcome{}, fieldError("website", "请留空此字段")
	}
	bodyHash := hashNormalized(normalized)

	existing, err := s.store.GetFeedbackByRequestID(ctx, normalizedRequestID(in.RequestID))
	switch {
	case err == nil:
		if existing.RequestBodyHash == bodyHash {
			return SubmitOutcome{Feedback: existing, Created: false, ConfirmationEmail: existing.ConfirmationEmail}, nil
		}
		return SubmitOutcome{}, ErrIdempotencyConflict
	case !errors.Is(err, store.ErrFeedbackNotFound):
		return SubmitOutcome{}, err
	}

	if normalized.Subscribe && !s.subscriptionsAvailable() {
		return SubmitOutcome{}, fieldError("subscribe_updates", "邮件订阅暂不可用，可先不勾选")
	}

	now := s.now().UTC()
	if err := s.consumeVerificationAttempts(ctx, clientIP, now); err != nil {
		return SubmitOutcome{}, err
	}
	method, err := s.verifySubmission(ctx, in, clientIP, now)
	if err != nil {
		return SubmitOutcome{}, err
	}
	if err := s.consumeSubmitLimits(ctx, clientIP, now); err != nil {
		return SubmitOutcome{}, err
	}

	decision := antispam.Decision{Verdict: model.SpamVerdictAllow, Signals: []string{}}
	if snap.SpamJudge != nil {
		decision = snap.SpamJudge.Evaluate(ctx, antispam.Input{
			Title: normalized.Title, Content: normalized.Content, SourceURL: normalized.SourceURL,
			Email: normalized.Email, ClientIP: clientIP, ElapsedSeconds: -1, Now: now,
		})
	}
	isSpam := decision.Verdict == model.SpamVerdictSpam
	status := model.FeedbackStatusPending
	var closedAt *time.Time
	if isSpam {
		status = model.FeedbackStatusSpam
		closedAt = &now
	}

	confirmationEmail := "none"
	fb := model.Feedback{
		ID:                 newFeedbackID(),
		RequestID:          normalizedRequestID(in.RequestID),
		RequestBodyHash:    bodyHash,
		Kind:               normalized.Kind,
		Title:              normalized.Title,
		Content:            normalized.Content,
		SourceURL:          normalized.SourceURL,
		StoryID:            normalized.StoryID,
		Nickname:           normalized.Nickname,
		Email:              normalized.Email,
		Status:             status,
		Version:            1,
		SubscriptionStatus: model.SubscriptionNone,
		ConfirmationEmail:  confirmationEmail,
		ContentHash:        antispam.ContentHash(normalized.Title, normalized.Content),
		EmailDomain:        antispam.EmailDomain(normalized.Email),
		SpamScore:          decision.Score,
		SpamVerdict:        decision.Verdict,
		SpamSignals:        strings.Join(decision.Signals, ","),
		SpamSource:         decision.Source,
		SpamModel:          decision.Model,
		SpamConfidence:     decision.Confidence,
		VerificationMethod: method,
		VerifiedAt:         &now,
		CreatedAt:          now,
		UpdatedAt:          now,
		ClosedAt:           closedAt,
	}

	input := store.CreateSubmissionInput{
		Feedback: &fb,
		Event: &model.FeedbackEvent{
			Seq: 1, FromStatus: "", ToStatus: status,
			Actor: "visitor", CreatedAt: now,
		},
		Now: now,
	}

	if normalized.Subscribe && !isSpam {
		fb.SubscriptionStatus = model.SubscriptionPendingConfirmation
		fb.ConsentVersion = ConsentVersion
		fb.ConsentedAt = &now
		if s.allowConfirmation(ctx, normalized.Email, now) {
			raw, token, err := s.newToken(fb.ID, model.TokenPurposeConfirm, normalized.Email, 24*time.Hour)
			if err != nil {
				return SubmitOutcome{}, err
			}
			input.Tokens = append(input.Tokens, token)
			data := s.confirmData(fb, token, raw)
			payload, err := encodePayload(data)
			if err != nil {
				return SubmitOutcome{}, err
			}
			input.Outbox = append(input.Outbox, model.MailOutbox{
				ID: newFeedbackID(), Type: model.MailTypeSubscriptionConfirm,
				EventKey:  "confirm:" + fb.ID + ":" + token.TokenHash[:12],
				Recipient: normalized.Email, Template: "subscription_confirm",
				Payload: payload, Status: model.MailStatusPending, NextAttemptAt: now,
			})
			confirmationEmail = "queued"
		} else {
			confirmationEmail = "deferred"
		}
		fb.ConfirmationEmail = confirmationEmail
	}

	adminNotified := false
	if !isSpam && s.allowAdminNotification(ctx, now) {
		adminNotified = true
		adminData := s.dataFor(fb)
		adminData.AdminURL = siteURL(snap.SiteURL) + "/admin/feedback/" + fb.ID
		adminPayload, err := encodePayload(adminData)
		if err != nil {
			return SubmitOutcome{}, err
		}
		input.Outbox = append(input.Outbox, model.MailOutbox{
			ID: newFeedbackID(), Type: model.MailTypeAdminNewFeedback,
			EventKey: "admin:" + fb.ID, Recipient: snap.NotifyTo,
			Template: "admin_new_feedback", Payload: adminPayload,
			Status: model.MailStatusPending, NextAttemptAt: now,
		})
	}
	if !isSpam && !adminNotified {
		s.logger.Warn("admin notification suppressed by hourly cap")
	}

	if err := s.store.CreateSubmission(ctx, input); err != nil {
		if errors.Is(err, store.ErrRequestConflict) {
			// Concurrent retry with the same request_id.
			existing, getErr := s.store.GetFeedbackByRequestID(ctx, fb.RequestID)
			if getErr == nil && existing.RequestBodyHash == bodyHash {
				return SubmitOutcome{Feedback: existing, Created: false, ConfirmationEmail: existing.ConfirmationEmail}, nil
			}
			return SubmitOutcome{}, ErrIdempotencyConflict
		}
		return SubmitOutcome{}, err
	}
	return SubmitOutcome{Feedback: fb, Created: true, ConfirmationEmail: confirmationEmail}, nil
}

func normalizeSubmission(in SubmitInput) (normalizedSubmission, error) {
	requestID := normalizedRequestID(in.RequestID)
	if err := validateRequestID(requestID); err != nil {
		return normalizedSubmission{}, err
	}
	kind := strings.TrimSpace(in.Kind)
	if !contains(model.FeedbackKinds, kind) {
		return normalizedSubmission{}, fieldError("kind", "请选择反馈类型")
	}
	title := strings.TrimSpace(in.Title)
	if count := utf8.RuneCountInString(title); count < TitleMin || count > TitleMax {
		return normalizedSubmission{}, fieldError("title", fmt.Sprintf("标题需为 %d–%d 个字符", TitleMin, TitleMax))
	}
	content := strings.TrimSpace(in.Content)
	if count := utf8.RuneCountInString(content); count < ContentMin || count > ContentMax {
		return normalizedSubmission{}, fieldError("content", fmt.Sprintf("内容需为 %d–%d 个字符", ContentMin, ContentMax))
	}
	nickname := strings.TrimSpace(in.Nickname)
	if utf8.RuneCountInString(nickname) > NicknameMax {
		return normalizedSubmission{}, fieldError("nickname", fmt.Sprintf("称呼最多 %d 个字符", NicknameMax))
	}
	storyID := strings.TrimSpace(in.StoryID)
	if len(storyID) > 64 || hasControl(storyID) {
		return normalizedSubmission{}, fieldError("story_id", "关联故事无效")
	}

	sourceURL := strings.TrimSpace(in.SourceURL)
	if sourceURL != "" {
		if len(sourceURL) > SourceURLMax {
			return normalizedSubmission{}, fieldError("source_url", "链接过长")
		}
		parsed, err := url.Parse(sourceURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return normalizedSubmission{}, fieldError("source_url", "请填写合法的 http/https 链接")
		}
	}

	email, err := NormalizeEmail(in.Email)
	if err != nil {
		return normalizedSubmission{}, err
	}
	if in.Subscribe && email == "" {
		return normalizedSubmission{}, fieldError("email", "勾选邮件通知后需要填写邮箱")
	}

	return normalizedSubmission{
		Kind: kind, Title: title, Content: content, SourceURL: sourceURL,
		StoryID: storyID, Nickname: nickname, Email: email, Subscribe: in.Subscribe,
	}, nil
}

// NormalizeEmail trims, rejects control characters/lists, and lower-cases the
// domain while preserving the local part verbatim.
func NormalizeEmail(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}
	if len(trimmed) > 254 {
		return "", fieldError("email", "邮箱过长")
	}
	if hasControl(trimmed) {
		return "", fieldError("email", "邮箱包含非法字符")
	}
	parsed, err := mail.ParseAddress(trimmed)
	if err != nil {
		return "", fieldError("email", "邮箱格式不正确")
	}
	if parsed.Address != trimmed {
		return "", fieldError("email", "请只填写一个邮箱地址，不要包含姓名或尖括号")
	}
	at := strings.LastIndex(trimmed, "@")
	if at <= 0 || at == len(trimmed)-1 {
		return "", fieldError("email", "邮箱格式不正确")
	}
	local := trimmed[:at]
	domain := strings.ToLower(trimmed[at+1:])
	normalized := local + "@" + domain
	if _, err := mail.ParseAddress(normalized); err != nil {
		return "", fieldError("email", "邮箱格式不正确")
	}
	return normalized, nil
}

func validateRequestID(value string) error {
	if len(value) < 8 || len(value) > 64 {
		return fieldError("request_id", "请求标识无效")
	}
	for _, r := range value {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return fieldError("request_id", "请求标识无效")
		}
	}
	return nil
}

func normalizedRequestID(value string) string { return strings.TrimSpace(value) }

func hashNormalized(n normalizedSubmission) string {
	encoded, _ := json.Marshal(n)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func (s *FeedbackService) consumeSubmitLimits(ctx context.Context, clientIP string, now time.Time) error {
	limits := s.snapshot().RateLimits
	key := security.HMACKey(s.secret, "feedback:ip", clientIP)
	hour := now.Truncate(time.Hour)
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	for _, window := range []struct {
		start time.Time
		ttl   time.Duration
		limit int
	}{
		{hour, time.Hour, limits.SubmitHourly},
		{day, 24 * time.Hour, limits.SubmitDaily},
	} {
		allowed, err := s.store.ConsumeRateLimit(ctx, key, window.start, window.start.Add(window.ttl), window.limit)
		if err != nil {
			return err
		}
		if !allowed {
			return ErrRateLimited
		}
	}
	return nil
}

type SubmitReceipt struct {
	ID                 string
	Status             string
	SubscriptionStatus string
	CreatedAt          time.Time
}

func Receipt(fb model.Feedback) SubmitReceipt {
	return SubmitReceipt{ID: fb.ID, Status: fb.Status, SubscriptionStatus: fb.SubscriptionStatus, CreatedAt: fb.CreatedAt}
}

type TokenResult struct {
	SubscriptionStatus string
	Outcome            string
}

func (s *FeedbackService) Confirm(ctx context.Context, rawToken string) (TokenResult, error) {
	if strings.TrimSpace(rawToken) == "" {
		return TokenResult{}, ErrInvalidToken
	}
	hash := security.HashToken(rawToken)
	now := s.now().UTC()

	token, err := s.store.GetToken(ctx, hash, model.TokenPurposeConfirm)
	if err != nil {
		return TokenResult{}, err
	}
	fb, err := s.store.GetFeedback(ctx, token.FeedbackID)
	if err != nil {
		return TokenResult{}, err
	}
	if token.ConsumedAt == nil && !token.ExpiresAt.After(now) {
		return TokenResult{}, ErrTokenExpired
	}

	unsubRaw, unsubToken, err := s.newToken(fb.ID, model.TokenPurposeUnsubscribe, fb.Email, 10*365*24*time.Hour)
	if err != nil {
		return TokenResult{}, err
	}
	input := store.ConfirmInput{
		TokenHash:  hash,
		Now:        now,
		UnsubToken: &unsubToken,
	}
	if model.IsNotifiableStatus(fb.Status) {
		data := s.dataFor(fb)
		data.UnsubscribeURL = mailer.UnsubscribeURL(siteURL(s.snapshot().SiteURL), unsubRaw)
		payload, err := encodePayload(data)
		if err != nil {
			return TokenResult{}, err
		}
		input.Outbox = append(input.Outbox, model.MailOutbox{
			ID: newFeedbackID(), Type: model.MailTypeUserStatusUpdate,
			EventKey:  "confirm-catchup:" + fb.ID + ":" + fb.Status,
			Recipient: fb.Email, Template: "user_status_update",
			Payload: payload, Status: model.MailStatusPending, NextAttemptAt: now,
		})
	}

	updated, outcome, err := s.store.ConfirmSubscription(ctx, input)
	if err != nil {
		return TokenResult{}, err
	}
	return TokenResult{SubscriptionStatus: updated.SubscriptionStatus, Outcome: string(outcome)}, nil
}

// ResendConfirmation always succeeds publicly. Internally it no-ops unless the
// address matches, the subscription is still pending, and rate limits allow it.
func (s *FeedbackService) ResendConfirmation(ctx context.Context, feedbackID, email, clientIP string) error {
	if !s.subscriptionsAvailable() {
		return nil
	}
	normalizedEmail, err := NormalizeEmail(email)
	if err != nil || normalizedEmail == "" {
		return nil
	}
	now := s.now().UTC()
	if !s.allowGlobalConfirmation(ctx, now) {
		return nil
	}
	if !s.consumeResendLimits(ctx, normalizedEmail, feedbackID, now) {
		return nil
	}
	fb, err := s.store.GetFeedback(ctx, strings.TrimSpace(feedbackID))
	if err != nil {
		return nil
	}
	if fb.Email == "" || !strings.EqualFold(fb.Email, normalizedEmail) {
		return nil
	}
	if fb.SubscriptionStatus != model.SubscriptionPendingConfirmation {
		return nil
	}

	raw, token, err := s.newToken(fb.ID, model.TokenPurposeConfirm, fb.Email, 24*time.Hour)
	if err != nil {
		return err
	}
	data := s.confirmData(fb, token, raw)
	payload, err := encodePayload(data)
	if err != nil {
		return err
	}
	outbox := &model.MailOutbox{
		ID: newFeedbackID(), Type: model.MailTypeSubscriptionConfirm,
		EventKey:  "confirm:" + fb.ID + ":" + token.TokenHash[:12],
		Recipient: fb.Email, Template: "subscription_confirm",
		Payload: payload, Status: model.MailStatusPending, NextAttemptAt: now,
	}
	return s.store.QueueConfirmation(ctx, &token, outbox, now)
}

func (s *FeedbackService) consumeResendLimits(ctx context.Context, email, feedbackID string, now time.Time) bool {
	hour := now.Truncate(time.Hour)
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	minute := now.Truncate(time.Minute)
	checks := []struct {
		key   string
		start time.Time
		ttl   time.Duration
		limit int
	}{
		{security.HMACKey(s.secret, "resend:email", email), hour, time.Hour, s.snapshot().RateLimits.ResendHourly},
		{security.HMACKey(s.secret, "resend:email:day", email), day, 24 * time.Hour, s.snapshot().RateLimits.ResendDaily},
		{security.HMACKey(s.secret, "resend:feedback", feedbackID), minute, time.Minute, 1},
	}
	for _, check := range checks {
		allowed, err := s.store.ConsumeRateLimit(ctx, check.key, check.start, check.start.Add(check.ttl), check.limit)
		if err != nil || !allowed {
			return false
		}
	}
	return true
}

func (s *FeedbackService) Unsubscribe(ctx context.Context, rawToken string) (TokenResult, error) {
	if strings.TrimSpace(rawToken) == "" {
		return TokenResult{}, ErrInvalidToken
	}
	fb, outcome, err := s.store.Unsubscribe(ctx, security.HashToken(rawToken), s.now().UTC())
	if err != nil {
		return TokenResult{}, err
	}
	return TokenResult{SubscriptionStatus: fb.SubscriptionStatus, Outcome: string(outcome)}, nil
}

type AdminUpdateInput struct {
	ID               string
	Version          int
	Status           string
	InternalNote     *string
	PublicReply      *string
	NotifySubscriber bool
}

type AdminUpdateResult struct {
	Feedback          model.Feedback
	Notified          bool
	NotifySkippedNote string
}

func (s *FeedbackService) UpdateFeedback(ctx context.Context, admin string, in AdminUpdateInput) (AdminUpdateResult, error) {
	current, err := s.store.GetFeedback(ctx, in.ID)
	if err != nil {
		return AdminUpdateResult{}, err
	}
	newStatus := current.Status
	if in.Status != "" {
		if !contains(model.FeedbackStatuses, in.Status) {
			return AdminUpdateResult{}, fieldError("status", "未知的反馈状态")
		}
		newStatus = in.Status
	}
	statusChanged := newStatus != current.Status

	var internalNote, publicReply *string
	if in.InternalNote != nil {
		note := strings.TrimSpace(*in.InternalNote)
		if utf8.RuneCountInString(note) > 5000 {
			return AdminUpdateResult{}, fieldError("internal_note", "内部备注最多 5000 个字符")
		}
		internalNote = &note
	}
	if in.PublicReply != nil {
		reply := strings.TrimSpace(*in.PublicReply)
		if utf8.RuneCountInString(reply) > 5000 {
			return AdminUpdateResult{}, fieldError("public_reply", "处理说明最多 5000 个字符")
		}
		publicReply = &reply
	}

	effectiveReply := current.PublicReply
	if publicReply != nil {
		effectiveReply = *publicReply
	}

	update := store.FeedbackUpdate{
		ID: in.ID, Version: in.Version, Status: in.Status,
		InternalNote: internalNote, PublicReply: publicReply,
		Actor: admin, RecordEvent: statusChanged, Now: s.now().UTC(),
	}
	if statusChanged {
		update.Event = &model.FeedbackEvent{ToStatus: newStatus}
	}

	result := AdminUpdateResult{}
	if statusChanged && in.NotifySubscriber {
		switch {
		case newStatus == model.FeedbackStatusSpam:
			result.NotifySkippedNote = "垃圾信息不会向投稿邮箱发信"
		case !model.IsNotifiableStatus(newStatus):
			result.NotifySkippedNote = "当前状态不会向订阅者发信"
		case current.SubscriptionStatus != model.SubscriptionConfirmed:
			result.NotifySkippedNote = "该反馈没有已确认的订阅者，不会发信"
		default:
			raw, token, err := s.newToken(current.ID, model.TokenPurposeUnsubscribe, current.Email, 10*365*24*time.Hour)
			if err != nil {
				return AdminUpdateResult{}, err
			}
			update.Tokens = append(update.Tokens, token)
			data := s.dataFor(current)
			data.Status = newStatus
			data.StatusLabel = mailer.StatusLabel(newStatus)
			data.PublicReply = effectiveReply
			data.UnsubscribeURL = mailer.UnsubscribeURL(siteURL(s.snapshot().SiteURL), raw)
			payload, err := encodePayload(data)
			if err != nil {
				return AdminUpdateResult{}, err
			}
			update.Outbox = append(update.Outbox, model.MailOutbox{
				ID: newFeedbackID(), Type: model.MailTypeUserStatusUpdate,
				EventKey:  fmt.Sprintf("status:%s:%s:%d", current.ID, newStatus, current.Version+1),
				Recipient: current.Email, Template: "user_status_update",
				Payload: payload, Status: model.MailStatusPending, NextAttemptAt: s.now().UTC(),
			})
			result.Notified = true
		}
	}

	updated, err := s.store.UpdateFeedback(ctx, update)
	if err != nil {
		return AdminUpdateResult{}, err
	}
	if statusChanged && newStatus == model.FeedbackStatusSpam {
		if err := s.store.CancelPendingByFeedback(ctx, current.ID, []string{model.MailTypeUserStatusUpdate},
			"feedback marked as spam", s.now().UTC()); err != nil {
			return AdminUpdateResult{}, err
		}
	}
	result.Feedback = updated
	return result, nil
}

func (s *FeedbackService) DeleteFeedback(ctx context.Context, admin, id string) error {
	return s.store.DeleteFeedback(ctx, id, admin, s.now().UTC())
}

func (s *FeedbackService) ListFeedback(ctx context.Context, filter store.FeedbackListFilter) ([]model.FeedbackListItemDTO, int64, error) {
	items, total, err := s.store.ListFeedback(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	out := make([]model.FeedbackListItemDTO, 0, len(items))
	for _, item := range items {
		out = append(out, model.FeedbackListItem(item))
	}
	return out, total, nil
}

func (s *FeedbackService) FeedbackDetail(ctx context.Context, id string) (model.FeedbackDetailDTO, error) {
	fb, err := s.store.GetFeedback(ctx, id)
	if err != nil {
		return model.FeedbackDetailDTO{}, err
	}
	events, err := s.store.ListEvents(ctx, id)
	if err != nil {
		return model.FeedbackDetailDTO{}, err
	}
	mail, err := s.store.ListOutboxByFeedback(ctx, id)
	if err != nil {
		return model.FeedbackDetailDTO{}, err
	}
	return model.FeedbackDetail(fb, events, mail), nil
}

func (s *FeedbackService) MailStatus(ctx context.Context) (model.MailStatusDTO, error) {
	counts, err := s.store.OutboxCounts(ctx)
	if err != nil {
		return model.MailStatusDTO{}, err
	}
	recent, err := s.store.RecentOutboxErrors(ctx, 10)
	if err != nil {
		return model.MailStatusDTO{}, err
	}
	snap := s.snapshot()
	status := model.MailStatusDTO{
		SMTPEnabled:      snap.SMTPEnabled,
		Configured:       snap.SMTPEnabled && snap.SMTPConfigured && snap.NotifyTo != "",
		Missing:          snap.SMTPMissing,
		NotifyConfigured: snap.NotifyTo != "",
		Error:            snap.SMTPError,
		Counts:           counts,
		RecentErrors:     make([]model.MailTaskDTO, 0, len(recent)),
	}
	for _, item := range recent {
		status.RecentErrors = append(status.RecentErrors, model.MailTaskDTO{
			ID: item.ID, Type: item.Type, Status: item.Status, Attempts: item.Attempts,
			NextAttemptAt: item.NextAttemptAt, LastError: item.LastError,
			SMTPAcceptedAt: item.SMTPAcceptedAt, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
		})
	}
	return status, nil
}

func (s *FeedbackService) QueueTestMail(ctx context.Context) error {
	snap := s.snapshot()
	if !snap.SMTPEnabled {
		return ErrSMTPUnavailable
	}
	if snap.SMTPError != "" {
		return errors.New("smtp is not ready: " + snap.SMTPError)
	}
	if snap.NotifyTo == "" {
		return errors.New("admin notify address is not configured")
	}
	now := s.now().UTC()
	data := mailer.TemplateData{
		Type: model.MailTypeSMTPTest, SiteName: "biodays",
		SiteURL: siteURL(snap.SiteURL),
	}
	payload, err := encodePayload(data)
	if err != nil {
		return err
	}
	item := &model.MailOutbox{
		ID: newFeedbackID(), Type: model.MailTypeSMTPTest,
		EventKey: "smtp-test:" + newFeedbackID(), Recipient: snap.NotifyTo,
		Template: "smtp_test", Payload: payload, Status: model.MailStatusPending,
		NextAttemptAt: now,
	}
	return s.store.InsertOutbox(ctx, item, now)
}

func (s *FeedbackService) RetryMail(ctx context.Context, id string) error {
	task, err := s.store.GetOutbox(ctx, id)
	if err != nil {
		return err
	}
	if task.Type != model.MailTypeSMTPTest {
		if _, err := s.store.GetFeedback(ctx, task.FeedbackID); err != nil {
			return err
		}
	}
	return s.store.ResetOutboxForRetry(ctx, id, s.now().UTC())
}

func (s *FeedbackService) MailTasks(ctx context.Context, filter store.OutboxFilter) ([]model.MailTaskDTO, int64, error) {
	items, total, err := s.store.ListOutbox(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	out := make([]model.MailTaskDTO, 0, len(items))
	for _, item := range items {
		out = append(out, model.MailTaskDTO{
			ID: item.ID, Type: item.Type, Status: item.Status, Attempts: item.Attempts,
			NextAttemptAt: item.NextAttemptAt, LastError: item.LastError,
			SMTPAcceptedAt: item.SMTPAcceptedAt, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
		})
	}
	return out, total, nil
}

func (s *FeedbackService) Cleanup(ctx context.Context) (store.CleanupReport, error) {
	return s.store.Cleanup(ctx, s.now().UTC())
}

func (s *FeedbackService) newToken(feedbackID, purpose, email string, ttl time.Duration) (string, model.SubscriptionToken, error) {
	raw, err := security.RandomToken()
	if err != nil {
		return "", model.SubscriptionToken{}, err
	}
	now := s.now().UTC()
	return raw, model.SubscriptionToken{
		TokenHash:        security.HashToken(raw),
		FeedbackID:       feedbackID,
		Purpose:          purpose,
		EmailFingerprint: security.HMACKey(s.secret, "email", strings.ToLower(email)),
		ExpiresAt:        now.Add(ttl),
		CreatedAt:        now,
	}, nil
}

// confirmData deliberately carries no submitted free text: the confirmation
// email only explains the subscription and links to the site.
func (s *FeedbackService) confirmData(fb model.Feedback, token model.SubscriptionToken, raw string) mailer.TemplateData {
	return mailer.TemplateData{
		Type:     model.MailTypeSubscriptionConfirm,
		SiteName: "biodays", SiteURL: siteURL(s.snapshot().SiteURL),
		FeedbackID:       fb.ID,
		ConfirmURL:       mailer.ConfirmURL(siteURL(s.snapshot().SiteURL), raw),
		TokenHash:        token.TokenHash,
		EmailFingerprint: token.EmailFingerprint,
	}
}

func (s *FeedbackService) dataFor(fb model.Feedback) mailer.TemplateData {
	return mailer.TemplateData{
		SiteName: "biodays", SiteURL: siteURL(s.snapshot().SiteURL),
		FeedbackID: fb.ID, Kind: fb.Kind, KindLabel: mailer.KindLabel(fb.Kind),
		Title: fb.Title, Nickname: fb.Nickname, Email: fb.Email,
		Content: fb.Content, SourceURL: fb.SourceURL, StoryID: fb.StoryID,
		Status: fb.Status, StatusLabel: mailer.StatusLabel(fb.Status),
		PublicReply:      fb.PublicReply,
		EmailFingerprint: security.HMACKey(s.secret, "email", strings.ToLower(fb.Email)),
	}
}

func encodePayload(data mailer.TemplateData) (string, error) {
	encoded, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func siteURL(value string) string {
	if strings.TrimSpace(value) == "" {
		return "http://localhost:3000"
	}
	return strings.TrimRight(value, "/")
}

func newFeedbackID() string {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

func hasControl(value string) bool {
	for _, r := range value {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
