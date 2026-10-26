package service_test

import (
	"context"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"biodays/server/internal/antispam"
	"biodays/server/internal/model"
	"biodays/server/internal/service"
	"biodays/server/internal/store"

	"github.com/stretchr/testify/require"
)

func solveChallenge(salt string, difficulty int) string {
	for i := 0; ; i++ {
		nonce := strconv.Itoa(i)
		if antispam.VerifyPoW(salt, nonce, difficulty) {
			return nonce
		}
	}
}

func newSpamFixture(t *testing.T, tweak func(*service.FeedbackSettings)) *fixture {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "feedback.db")
	st, err := store.OpenFeedback(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, st.Close()) })
	secret := []byte("0123456789abcdef0123456789abcdef")
	f := &fixture{store: st, now: time.Date(2026, 10, 24, 10, 0, 0, 0, time.UTC)}
	settings := service.FeedbackSettings{
		Enabled: true, SubscriptionsEnabled: true, SMTPEnabled: true,
		SiteURL: "https://biodays.test", NotifyTo: "admin@biodays.test",
		SubmitHourly: 100, SubmitDaily: 100, ResendHourly: 100, ResendDaily: 100,
		ConfirmHourly: 100, ConfirmDaily: 100, ConfirmDailyGlobal: 100, AdminMailHourly: 100,
	}
	settings.Spam = service.SpamSettings{
		Enabled: true,
		Judge: antispam.NewJudge(st, secret, antispam.Settings{
			Enabled: true, ReviewScore: 1, BlockScore: 30, AIThreshold: 0.8,
			AIDailyLimit: 100, AIMaxChars: 4000,
		}, nil, nil, func() time.Time { return f.now }),
	}
	if tweak != nil {
		tweak(&settings)
	}
	f.service = service.NewFeedbackService(st, settings, secret, func() time.Time { return f.now })
	return f
}

func (f *fixture) issueChallenge(t *testing.T) (model.ChallengeDTO, string) {
	t.Helper()
	challenge, err := f.service.IssueChallenge(context.Background(), "203.0.113.10")
	require.NoError(t, err)
	f.now = f.now.Add(time.Second)
	return challenge, solveChallenge(challenge.Salt, challenge.Difficulty)
}

func TestPoWChallengeAndIdempotentRetry(t *testing.T) {
	f := newFixture(t, func(settings *service.FeedbackSettings) {
		settings.Verification.PowEnabled = true
		settings.Verification.PowDifficulty = 2
		settings.Verification.PowMinSeconds = 1
	})
	challenge, nonce := f.issueChallenge(t)
	outcome, err := f.submit(t, "req-pow-0001", func(in *service.SubmitInput) {
		in.ChallengeID = challenge.ChallengeID
		in.ChallengeNonce = nonce
	})
	require.NoError(t, err)
	require.True(t, outcome.Created)

	stored, err := f.store.GetFeedback(context.Background(), outcome.Feedback.ID)
	require.NoError(t, err)
	require.Equal(t, model.VerificationMethodPow, stored.VerificationMethod)

	consumed, err := f.store.GetChallenge(context.Background(), challenge.ChallengeID)
	require.NoError(t, err)
	require.NotNil(t, consumed.ConsumedAt)

	retry, err := f.submit(t, "req-pow-0001", nil)
	require.NoError(t, err)
	require.False(t, retry.Created)
	require.Equal(t, outcome.Feedback.ID, retry.Feedback.ID)
}

func TestChallengeReplayRejected(t *testing.T) {
	f := newFixture(t, func(settings *service.FeedbackSettings) {
		settings.Verification.PowEnabled = true
		settings.Verification.PowDifficulty = 2
		settings.Verification.PowMinSeconds = 1
	})
	challenge, nonce := f.issueChallenge(t)
	_, err := f.submit(t, "req-pow-0002", func(in *service.SubmitInput) {
		in.ChallengeID = challenge.ChallengeID
		in.ChallengeNonce = nonce
	})
	require.NoError(t, err)

	_, err = f.submit(t, "req-pow-0003", func(in *service.SubmitInput) {
		in.ChallengeID = challenge.ChallengeID
		in.ChallengeNonce = nonce
	})
	require.ErrorIs(t, err, service.ErrChallengeConsumed)
}

func TestChallengeExpiredAndTooFast(t *testing.T) {
	f := newFixture(t, func(settings *service.FeedbackSettings) {
		settings.Verification.PowEnabled = true
		settings.Verification.PowDifficulty = 2
		settings.Verification.PowMinSeconds = 1
		settings.Verification.PowTTL = time.Minute
	})
	challenge, err := f.service.IssueChallenge(context.Background(), "203.0.113.10")
	require.NoError(t, err)
	nonce := solveChallenge(challenge.Salt, challenge.Difficulty)
	_, err = f.submit(t, "req-pow-0004", func(in *service.SubmitInput) {
		in.ChallengeID = challenge.ChallengeID
		in.ChallengeNonce = nonce
	})
	require.ErrorIs(t, err, service.ErrChallengeTooFast)

	f.now = f.now.Add(2 * time.Minute)
	challenge, err = f.service.IssueChallenge(context.Background(), "203.0.113.10")
	require.NoError(t, err)
	f.now = f.now.Add(2 * time.Minute)
	nonce = solveChallenge(challenge.Salt, challenge.Difficulty)
	_, err = f.submit(t, "req-pow-0005", func(in *service.SubmitInput) {
		in.ChallengeID = challenge.ChallengeID
		in.ChallengeNonce = nonce
	})
	require.ErrorIs(t, err, service.ErrChallengeExpired)
}

func TestAutoSpamSkipsMail(t *testing.T) {
	f := newSpamFixture(t, nil)
	content := "这是一段正常长度的反馈内容，用于测试重复内容自动隔离。"
	first, err := f.submit(t, "req-spam-0001", func(in *service.SubmitInput) {
		in.Title = "重复内容"
		in.Content = content
	})
	require.NoError(t, err)
	require.Equal(t, model.FeedbackStatusPending, first.Feedback.Status)

	second, err := f.submit(t, "req-spam-0002", func(in *service.SubmitInput) {
		in.Title = "重复内容"
		in.Content = content
	})
	require.NoError(t, err)
	require.Equal(t, model.FeedbackStatusSpam, second.Feedback.Status)
	require.Equal(t, "none", second.ConfirmationEmail)
	require.Empty(t, f.outbox(t, second.Feedback.ID))
}

func TestConfirmationThrottleAndAdminCap(t *testing.T) {
	f := newFixture(t, func(settings *service.FeedbackSettings) {
		settings.ConfirmHourly = 1
		settings.ConfirmDaily = 3
		settings.AdminMailHourly = 1
	})
	first, err := f.submit(t, "req-mail-0001", func(in *service.SubmitInput) {
		in.Subscribe = true
		in.Email = "reader@example.com"
	})
	require.NoError(t, err)
	require.Equal(t, "queued", first.ConfirmationEmail)

	second, err := f.submit(t, "req-mail-0002", func(in *service.SubmitInput) {
		in.Title = "另一个话题"
		in.Content = "这是一段不同的反馈内容，用于测试确认邮件节流。"
		in.Subscribe = true
		in.Email = "reader@example.com"
	})
	require.NoError(t, err)
	require.Equal(t, "deferred", second.ConfirmationEmail)

	secondOutbox := f.outbox(t, second.Feedback.ID)
	require.Empty(t, secondOutbox)
}
