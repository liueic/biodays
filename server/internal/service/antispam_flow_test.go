package service_test

import (
	"path/filepath"
	"testing"
	"time"

	"biodays/server/internal/antispam"
	"biodays/server/internal/service"
	"biodays/server/internal/settings"
	"biodays/server/internal/store"

	"github.com/stretchr/testify/require"
)

func newSpamFixture(t *testing.T, tweak func(*service.EffectiveSettings)) *fixture {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "feedback.db")
	st, err := store.OpenFeedback(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, st.Close()) })
	secret := []byte("0123456789abcdef0123456789abcdef")
	f := &fixture{store: st, now: time.Date(2026, 10, 24, 10, 0, 0, 0, time.UTC)}
	values := service.EffectiveSettings{
		Enabled: true, SubscriptionsEnabled: true, SMTPEnabled: true,
		SiteURL: "https://biodays.test", NotifyTo: "admin@biodays.test",
		RateLimits: settings.RateLimitsGroup{
			SubmitHourly: 100, SubmitDaily: 100, ResendHourly: 100, ResendDaily: 100,
			ConfirmHourly: 100, ConfirmDaily: 100, ConfirmDailyGlobal: 100, AdminMailHourly: 100,
		},
	}
	values.SpamJudge = antispam.NewJudge(st, secret, antispam.Settings{
		Enabled: true, ReviewScore: 1, BlockScore: 30, AIThreshold: 0.8,
		AIDailyLimit: 100, AIMaxChars: 4000,
	}, nil, nil, func() time.Time { return f.now })
	if tweak != nil {
		tweak(&values)
	}
	f.service = service.NewFeedbackService(st, service.StaticSettings(values), secret, func() time.Time { return f.now })
	return f
}

func TestAutoSpamSkipsMail(t *testing.T) {
	f := newSpamFixture(t, nil)
	content := "这是一段正常长度的反馈内容，用于测试重复内容自动隔离。"
	first, err := f.submit(t, "req-spam-0001", func(in *service.SubmitInput) {
		in.Title = "重复内容"
		in.Content = content
	})
	require.NoError(t, err)
	require.Equal(t, "pending", first.Feedback.Status)

	second, err := f.submit(t, "req-spam-0002", func(in *service.SubmitInput) {
		in.Title = "重复内容"
		in.Content = content
	})
	require.NoError(t, err)
	require.Equal(t, "spam", second.Feedback.Status)
	require.Equal(t, "none", second.ConfirmationEmail)
	require.Empty(t, f.outbox(t, second.Feedback.ID))
}

func TestConfirmationThrottleAndAdminCap(t *testing.T) {
	f := newFixture(t, func(values *service.EffectiveSettings) {
		values.RateLimits.ConfirmHourly = 1
		values.RateLimits.ConfirmDaily = 3
		values.RateLimits.AdminMailHourly = 1
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

func TestRateLimitChangeAppliesWithoutRestart(t *testing.T) {
	f := newFixture(t, nil)
	_, err := f.submit(t, "req-hot-0001", nil)
	require.NoError(t, err)

	// Simulate an admin save: the source now serves tighter limits.
	f.source.values.RateLimits.SubmitHourly = 1
	_, err = f.submit(t, "req-hot-0002", func(in *service.SubmitInput) {
		in.Title = "另一条"
	})
	require.ErrorIs(t, err, service.ErrRateLimited)
}
