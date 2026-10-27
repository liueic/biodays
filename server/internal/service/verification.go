package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"biodays/server/internal/antispam"
	"biodays/server/internal/model"
	"biodays/server/internal/security"
)

// consumeVerificationAttempts charges the per-IP verification budget only
// when Turnstile is active, so enabling the challenge cannot be used to burn
// an innocent IP's submit quota.
func (s *FeedbackService) consumeVerificationAttempts(ctx context.Context, clientIP string, now time.Time) error {
	if !s.snapshot().TurnstileEnabled {
		return nil
	}
	key := security.HMACKey(s.secret, "verify:ip", clientIP)
	hour := now.Truncate(time.Hour)
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	for _, window := range []struct {
		start time.Time
		ttl   time.Duration
		limit int
	}{
		{hour, time.Hour, 30},
		{day, 24 * time.Hour, 300},
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

// verifySubmission resolves the human-verification step. With Turnstile off
// (the default, and the only option for mainland visitors) submissions pass
// with method none; honeypot, rate limits and spam scoring still apply.
func (s *FeedbackService) verifySubmission(ctx context.Context, in SubmitInput, clientIP string, now time.Time) (string, error) {
	snap := s.snapshot()
	if !snap.TurnstileEnabled {
		return model.VerificationMethodNone, nil
	}
	token := strings.TrimSpace(in.TurnstileToken)
	if token == "" {
		return "", ErrTurnstileRequired
	}
	verifier := snap.TurnstileVerifier
	if verifier == nil {
		return "", ErrVerificationUnavailable
	}
	err := verifier.Verify(ctx, token, clientIP)
	switch {
	case err == nil:
		return model.VerificationMethodTurnstile, nil
	case errors.Is(err, antispam.ErrTurnstileRejected):
		return "", ErrTurnstileFailed
	default:
		return "", ErrVerificationUnavailable
	}
}

func (s *FeedbackService) allowConfirmation(ctx context.Context, email string, now time.Time) bool {
	email = strings.TrimSpace(email)
	if email == "" {
		return false
	}
	limits := s.snapshot().RateLimits
	hour := now.Truncate(time.Hour)
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	emailKey := security.HMACKey(s.secret, "confirm:email", email)
	checks := []struct {
		key   string
		start time.Time
		ttl   time.Duration
		limit int
	}{
		{emailKey, hour, time.Hour, limits.ConfirmHourly},
		{security.HMACKey(s.secret, "confirm:email:day", email), day, 24 * time.Hour, limits.ConfirmDaily},
	}
	for _, check := range checks {
		allowed, err := s.store.ConsumeRateLimit(ctx, check.key, check.start, check.start.Add(check.ttl), check.limit)
		if err != nil || !allowed {
			return false
		}
	}
	return s.allowGlobalConfirmation(ctx, now)
}

func (s *FeedbackService) allowGlobalConfirmation(ctx context.Context, now time.Time) bool {
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	allowed, err := s.store.ConsumeRateLimit(ctx,
		security.HMACKey(s.secret, "confirm:global:day"), day, day.Add(24*time.Hour), s.snapshot().RateLimits.ConfirmDailyGlobal)
	return err == nil && allowed
}

func (s *FeedbackService) allowAdminNotification(ctx context.Context, now time.Time) bool {
	hour := now.Truncate(time.Hour)
	allowed, err := s.store.ConsumeRateLimit(ctx,
		security.HMACKey(s.secret, "admin:notify"), hour, hour.Add(time.Hour), s.snapshot().RateLimits.AdminMailHourly)
	return err == nil && allowed
}
