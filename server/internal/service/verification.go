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

func (s *FeedbackService) consumeVerificationAttempts(ctx context.Context, clientIP string, now time.Time) error {
	settings := s.settings.Verification
	if !settings.PowEnabled && !settings.TurnstileEnabled {
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

func (s *FeedbackService) verifySubmission(ctx context.Context, in SubmitInput, clientIP string, now time.Time) (string, time.Time, error) {
	settings := s.settings.Verification
	if !settings.PowEnabled && !settings.TurnstileEnabled {
		return model.VerificationMethodNone, time.Time{}, nil
	}
	token := strings.TrimSpace(in.TurnstileToken)
	challengeID := strings.TrimSpace(in.ChallengeID)

	if settings.TurnstileEnabled {
		if token != "" {
			if settings.Turnstile == nil {
				return "", time.Time{}, ErrVerificationUnavailable
			}
			err := settings.Turnstile.Verify(ctx, token, clientIP)
			switch {
			case err == nil:
				return model.VerificationMethodTurnstile, time.Time{}, nil
			case errors.Is(err, antispam.ErrTurnstileRejected):
				return "", time.Time{}, ErrTurnstileFailed
			case errors.Is(err, antispam.ErrTurnstileUnavailable):
				if !settings.PowEnabled || challengeID == "" {
					return "", time.Time{}, ErrVerificationUnavailable
				}
			default:
				return "", time.Time{}, ErrVerificationUnavailable
			}
		}
		if !settings.PowEnabled {
			if token == "" {
				return "", time.Time{}, ErrTurnstileRequired
			}
			return "", time.Time{}, ErrVerificationUnavailable
		}
	}

	if settings.PowEnabled {
		issuedAt, err := s.verifyPoW(ctx, challengeID, in.ChallengeNonce, clientIP, now)
		if err != nil {
			return "", time.Time{}, err
		}
		return model.VerificationMethodPow, issuedAt, nil
	}
	return model.VerificationMethodNone, time.Time{}, nil
}

func (s *FeedbackService) allowConfirmation(ctx context.Context, email string, now time.Time) bool {
	email = strings.TrimSpace(email)
	if email == "" {
		return false
	}
	hour := now.Truncate(time.Hour)
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	emailKey := security.HMACKey(s.secret, "confirm:email", email)
	checks := []struct {
		key   string
		start time.Time
		ttl   time.Duration
		limit int
	}{
		{emailKey, hour, time.Hour, s.settings.ConfirmHourly},
		{security.HMACKey(s.secret, "confirm:email:day", email), day, 24 * time.Hour, s.settings.ConfirmDaily},
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
		security.HMACKey(s.secret, "confirm:global:day"), day, day.Add(24*time.Hour), s.settings.ConfirmDailyGlobal)
	return err == nil && allowed
}

func (s *FeedbackService) allowAdminNotification(ctx context.Context, now time.Time) bool {
	hour := now.Truncate(time.Hour)
	allowed, err := s.store.ConsumeRateLimit(ctx,
		security.HMACKey(s.secret, "admin:notify"), hour, hour.Add(time.Hour), s.settings.AdminMailHourly)
	return err == nil && allowed
}
