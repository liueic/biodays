package service

import (
	"context"
	"strconv"
	"strings"
	"time"

	"biodays/server/internal/antispam"
	"biodays/server/internal/model"
	"biodays/server/internal/security"
	"biodays/server/internal/store"
)

func (s *FeedbackService) IssueChallenge(ctx context.Context, clientIP string) (model.ChallengeDTO, error) {
	settings := s.settings.Verification
	if !settings.PowEnabled {
		return model.ChallengeDTO{}, ErrChallengeDisabled
	}
	now := s.now().UTC()
	if err := s.consumeIssueLimits(ctx, clientIP, now); err != nil {
		return model.ChallengeDTO{}, err
	}
	id, err := security.RandomHex(16)
	if err != nil {
		return model.ChallengeDTO{}, err
	}
	salt, err := security.RandomHex(16)
	if err != nil {
		return model.ChallengeDTO{}, err
	}
	expires := now.Add(settings.PowTTL)
	ipHash := security.HMACKey(s.secret, "challenge:ip", clientIP)
	challenge := model.FeedbackChallenge{
		ID:         id,
		IPHash:     ipHash,
		Salt:       salt,
		Signature:  challengeSignature(s.secret, id, salt, settings.PowDifficulty, expires, ipHash),
		Difficulty: settings.PowDifficulty,
		MinSeconds: settings.PowMinSeconds,
		IssuedAt:   now,
		ExpiresAt:  expires,
	}
	if err := s.store.CreateChallenge(ctx, &challenge); err != nil {
		return model.ChallengeDTO{}, err
	}
	return model.ChallengeDTO{
		ChallengeID:      id,
		Salt:             salt,
		Difficulty:       settings.PowDifficulty,
		MinSubmitSeconds: settings.PowMinSeconds,
		IssuedAt:         now,
		ExpiresAt:        expires,
	}, nil
}

func (s *FeedbackService) consumeIssueLimits(ctx context.Context, clientIP string, now time.Time) error {
	key := security.HMACKey(s.secret, "challenge:issue:ip", clientIP)
	minute := now.Truncate(time.Minute)
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	for _, window := range []struct {
		start time.Time
		ttl   time.Duration
		limit int
	}{
		{minute, time.Minute, 20},
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

func (s *FeedbackService) verifyPoW(ctx context.Context, challengeID, nonce, clientIP string, now time.Time) (time.Time, error) {
	challengeID = strings.TrimSpace(challengeID)
	if challengeID == "" {
		return time.Time{}, ErrChallengeRequired
	}
	challenge, err := s.store.GetChallenge(ctx, challengeID)
	if err != nil {
		if err == store.ErrChallengeNotFound {
			return time.Time{}, ErrChallengeInvalid
		}
		return time.Time{}, err
	}
	if challenge.ConsumedAt != nil {
		return time.Time{}, ErrChallengeConsumed
	}
	if !challenge.ExpiresAt.After(now) {
		return time.Time{}, ErrChallengeExpired
	}
	ipHash := security.HMACKey(s.secret, "challenge:ip", clientIP)
	if !security.ConstantTimeEqual(challenge.IPHash, ipHash) {
		return time.Time{}, ErrChallengeInvalid
	}
	expected := challengeSignature(s.secret, challenge.ID, challenge.Salt, challenge.Difficulty, challenge.ExpiresAt, challenge.IPHash)
	if !security.ConstantTimeEqual(challenge.Signature, expected) {
		return time.Time{}, ErrChallengeInvalid
	}
	if now.Before(challenge.IssuedAt) || now.Sub(challenge.IssuedAt) < time.Duration(challenge.MinSeconds)*time.Second {
		return time.Time{}, ErrChallengeTooFast
	}
	if !antispam.VerifyPoW(challenge.Salt, strings.TrimSpace(nonce), challenge.Difficulty) {
		return time.Time{}, ErrChallengeFailed
	}
	return challenge.IssuedAt, nil
}

func challengeSignature(secret []byte, id, salt string, difficulty int, expires time.Time, ipHash string) string {
	return security.HMACKey(secret, "challenge:v1", id, salt,
		strconv.Itoa(difficulty), strconv.FormatInt(expires.Unix(), 10), ipHash)
}
