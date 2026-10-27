package service_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"biodays/server/internal/mailer"
	"biodays/server/internal/model"
	"biodays/server/internal/service"
	"biodays/server/internal/settings"
	"biodays/server/internal/store"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// mutableSource lets a test flip effective values mid-run, mimicking an
// admin settings save landing between requests.
type mutableSource struct{ values service.EffectiveSettings }

func (m *mutableSource) Effective() service.EffectiveSettings { return m.values }

type fixture struct {
	service *service.FeedbackService
	admin   *service.AdminService
	store   *store.FeedbackStore
	now     time.Time
	values  service.EffectiveSettings
	source  *mutableSource
}

func newFixture(t *testing.T, tweak func(*service.EffectiveSettings)) *fixture {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "feedback.db")
	st, err := store.OpenFeedback(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, st.Close()) })

	values := service.EffectiveSettings{
		Enabled: true, SubscriptionsEnabled: true, SMTPEnabled: true,
		SiteURL: "https://biodays.test", NotifyTo: "admin@biodays.test",
		RateLimits: settings.RateLimitsGroup{
			SubmitHourly: 100, SubmitDaily: 100, ResendHourly: 100, ResendDaily: 100,
			ConfirmHourly: 100, ConfirmDaily: 100, ConfirmDailyGlobal: 100, AdminMailHourly: 100,
		},
	}
	if tweak != nil {
		tweak(&values)
	}
	f := &fixture{store: st, now: time.Date(2026, 10, 24, 10, 0, 0, 0, time.UTC), values: values}
	f.source = &mutableSource{values: values}
	secret := []byte("0123456789abcdef0123456789abcdef")
	f.service = service.NewFeedbackService(st, f.source, secret, func() time.Time { return f.now })
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-horse-123"), bcrypt.MinCost)
	require.NoError(t, err)
	f.admin = service.NewAdminService(st, service.AdminSettings{
		Username: "editor", PasswordHash: string(hash), SessionTTL: 8 * time.Hour, Secret: secret,
	}, func() time.Time { return f.now })
	return f
}

func (f *fixture) submit(t *testing.T, requestID string, tweak func(*service.SubmitInput)) (service.SubmitOutcome, error) {
	t.Helper()
	in := service.SubmitInput{
		RequestID: requestID, Kind: model.FeedbackKindTopic,
		Title: "想了解 GFP 的发现故事", Content: "希望讲讲荧光蛋白怎样从水母研究走向实验室应用。",
	}
	if tweak != nil {
		tweak(&in)
	}
	return f.service.Submit(context.Background(), in, "203.0.113.10")
}

func (f *fixture) outbox(t *testing.T, feedbackID string) []model.MailOutbox {
	t.Helper()
	items, err := f.store.ListOutboxByFeedback(context.Background(), feedbackID)
	require.NoError(t, err)
	return items
}

func (f *fixture) confirmToken(t *testing.T, feedbackID string) string {
	t.Helper()
	for _, item := range f.outbox(t, feedbackID) {
		if item.Type != model.MailTypeSubscriptionConfirm || item.Status != model.MailStatusPending {
			continue
		}
		var data mailer.TemplateData
		require.NoError(t, json.Unmarshal([]byte(item.Payload), &data))
		return tokenFromURL(t, data.ConfirmURL)
	}
	t.Fatalf("no pending confirmation task for %s", feedbackID)
	return ""
}

func (f *fixture) unsubscribeToken(t *testing.T, feedbackID string) string {
	t.Helper()
	for _, item := range f.outbox(t, feedbackID) {
		if item.Type != model.MailTypeUserStatusUpdate {
			continue
		}
		var data mailer.TemplateData
		require.NoError(t, json.Unmarshal([]byte(item.Payload), &data))
		return tokenFromURL(t, data.UnsubscribeURL)
	}
	t.Fatalf("no status task with unsubscribe link for %s", feedbackID)
	return ""
}

func tokenFromURL(t *testing.T, raw string) string {
	t.Helper()
	index := strings.Index(raw, "#token=")
	require.GreaterOrEqual(t, index, 0, raw)
	return raw[index+len("#token="):]
}

func TestAnonymousSubmissionAndDeduplication(t *testing.T) {
	f := newFixture(t, nil)
	outcome, err := f.submit(t, "req-anon-0001", nil)
	require.NoError(t, err)
	require.True(t, outcome.Created)
	require.Equal(t, model.FeedbackStatusPending, outcome.Feedback.Status)
	require.Equal(t, model.SubscriptionNone, outcome.Feedback.SubscriptionStatus)
	require.Empty(t, outcome.Feedback.Email)

	tasks := f.outbox(t, outcome.Feedback.ID)
	require.Len(t, tasks, 1)
	require.Equal(t, model.MailTypeAdminNewFeedback, tasks[0].Type)
	require.Equal(t, "admin@biodays.test", tasks[0].Recipient)
	require.NotEmpty(t, tasks[0].Payload)

	retry, err := f.submit(t, "req-anon-0001", nil)
	require.NoError(t, err)
	require.False(t, retry.Created)
	require.Equal(t, outcome.Feedback.ID, retry.Feedback.ID)

	_, err = f.submit(t, "req-anon-0001", func(in *service.SubmitInput) {
		in.Title = "另一个标题"
	})
	require.ErrorIs(t, err, service.ErrIdempotencyConflict)
}

func TestMultipleFeedbacksShareDatabase(t *testing.T) {
	f := newFixture(t, nil)
	first, err := f.submit(t, "req-second-0001", nil)
	require.NoError(t, err)
	second, err := f.submit(t, "req-second-0002", func(in *service.SubmitInput) {
		in.Title = "第二条反馈"
	})
	require.NoError(t, err)
	require.NotEqual(t, first.Feedback.ID, second.Feedback.ID)

	list, total, err := f.service.ListFeedback(context.Background(), store.FeedbackListFilter{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, list, 2)
}

func TestSubmissionValidation(t *testing.T) {
	f := newFixture(t, nil)
	cases := []struct {
		name  string
		tweak func(*service.SubmitInput)
		field string
	}{
		{"subscribe without email", func(in *service.SubmitInput) { in.Subscribe = true }, "email"},
		{"short content", func(in *service.SubmitInput) { in.Content = "太短" }, "content"},
		{"long title", func(in *service.SubmitInput) { in.Title = strings.Repeat("字", 81) }, "title"},
		{"long nickname", func(in *service.SubmitInput) { in.Nickname = strings.Repeat("名", 41); in.Email = "a@example.com" }, "nickname"},
		{"bad url", func(in *service.SubmitInput) { in.SourceURL = "javascript:alert(1)" }, "source_url"},
		{"bad email", func(in *service.SubmitInput) { in.Email = "not-an-email" }, "email"},
		{"bad kind", func(in *service.SubmitInput) { in.Kind = "unknown" }, "kind"},
		{"honeypot", func(in *service.SubmitInput) { in.Honeypot = "http://spam" }, "website"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := f.submit(t, "req-validate-"+strings.ReplaceAll(tc.name, " ", "-"), tc.tweak)
			var validation *service.ValidationError
			require.ErrorAs(t, err, &validation)
			require.Equal(t, tc.field, validation.Field)
		})
	}

	noSMTP := newFixture(t, func(s *service.EffectiveSettings) { s.SMTPEnabled = false })
	_, err := noSMTP.submit(t, "req-no-smtp", func(in *service.SubmitInput) {
		in.Email = "reader@example.com"
		in.Subscribe = true
	})
	var validation *service.ValidationError
	require.ErrorAs(t, err, &validation)
	require.Equal(t, "subscribe_updates", validation.Field)
}

func TestSubscriptionConfirmationAndStatusNotification(t *testing.T) {
	f := newFixture(t, nil)
	outcome, err := f.submit(t, "req-sub-0001", func(in *service.SubmitInput) {
		in.Email = "Reader@Example.COM"
		in.Subscribe = true
	})
	require.NoError(t, err)
	require.Equal(t, model.SubscriptionPendingConfirmation, outcome.Feedback.SubscriptionStatus)
	require.Equal(t, "Reader@example.com", outcome.Feedback.Email)

	tasks := f.outbox(t, outcome.Feedback.ID)
	require.Len(t, tasks, 2)
	var confirm *model.MailOutbox
	for i := range tasks {
		if tasks[i].Type == model.MailTypeSubscriptionConfirm {
			confirm = &tasks[i]
		}
	}
	require.NotNil(t, confirm)
	require.NotContains(t, confirm.Payload, "希望讲讲荧光蛋白") // no unverified free text in confirm mail

	result, err := f.service.Confirm(context.Background(), f.confirmToken(t, outcome.Feedback.ID))
	require.NoError(t, err)
	require.Equal(t, "confirmed", result.Outcome)
	require.Equal(t, model.SubscriptionConfirmed, result.SubscriptionStatus)

	again, err := f.service.Confirm(context.Background(), f.confirmToken(t, outcome.Feedback.ID))
	require.NoError(t, err)
	require.Contains(t, []string{"already_confirmed", "confirmed"}, again.Outcome)
	require.Equal(t, model.SubscriptionConfirmed, again.SubscriptionStatus)

	detail, err := f.service.FeedbackDetail(context.Background(), outcome.Feedback.ID)
	require.NoError(t, err)
	update, err := f.service.UpdateFeedback(context.Background(), "editor", service.AdminUpdateInput{
		ID: detail.ID, Version: detail.Version, Status: model.FeedbackStatusAccepted,
		PublicReply: stringPtr("已采纳，会整理成故事。"), NotifySubscriber: true,
	})
	require.NoError(t, err)
	require.True(t, update.Notified)

	var statusTasks int
	for _, task := range f.outbox(t, outcome.Feedback.ID) {
		if task.Type == model.MailTypeUserStatusUpdate {
			statusTasks++
			require.Equal(t, "Reader@example.com", task.Recipient)
			require.NotEmpty(t, task.Payload)
		}
	}
	require.Equal(t, 1, statusTasks)
}

func stringPtr(value string) *string { return &value }

func TestConfirmAfterResultQueuesOnce(t *testing.T) {
	f := newFixture(t, nil)
	outcome, err := f.submit(t, "req-catchup-0001", func(in *service.SubmitInput) {
		in.Email = "reader@example.com"
		in.Subscribe = true
	})
	require.NoError(t, err)
	detail, err := f.service.FeedbackDetail(context.Background(), outcome.Feedback.ID)
	require.NoError(t, err)
	_, err = f.service.UpdateFeedback(context.Background(), "editor", service.AdminUpdateInput{
		ID: detail.ID, Version: detail.Version, Status: model.FeedbackStatusCompleted,
		PublicReply: stringPtr("已经修正，谢谢。"),
	})
	require.NoError(t, err)

	_, err = f.service.Confirm(context.Background(), f.confirmToken(t, outcome.Feedback.ID))
	require.NoError(t, err)
	count := 0
	for _, task := range f.outbox(t, outcome.Feedback.ID) {
		if task.Type == model.MailTypeUserStatusUpdate {
			count++
		}
	}
	require.Equal(t, 1, count)
}

func TestNoteOnlyUpdateAndVersionConflict(t *testing.T) {
	f := newFixture(t, nil)
	outcome, err := f.submit(t, "req-note-0001", func(in *service.SubmitInput) {
		in.Email = "reader@example.com"
		in.Subscribe = true
	})
	require.NoError(t, err)
	_, err = f.service.Confirm(context.Background(), f.confirmToken(t, outcome.Feedback.ID))
	require.NoError(t, err)
	detail, err := f.service.FeedbackDetail(context.Background(), outcome.Feedback.ID)
	require.NoError(t, err)

	before := len(f.outbox(t, outcome.Feedback.ID))
	_, err = f.service.UpdateFeedback(context.Background(), "editor", service.AdminUpdateInput{
		ID: detail.ID, Version: detail.Version, InternalNote: stringPtr("仅内部可见"), NotifySubscriber: true,
	})
	require.NoError(t, err)
	require.Len(t, f.outbox(t, outcome.Feedback.ID), before)
	detail2, err := f.service.FeedbackDetail(context.Background(), outcome.Feedback.ID)
	require.NoError(t, err)
	require.Equal(t, "仅内部可见", detail2.InternalNote)
	require.Len(t, detail2.Events, 1) // only the creation event

	_, err = f.service.UpdateFeedback(context.Background(), "editor", service.AdminUpdateInput{
		ID: detail.ID, Version: detail.Version, Status: model.FeedbackStatusRejected, NotifySubscriber: true,
	})
	require.ErrorIs(t, err, service.ErrVersionConflict)
}

func TestSpamAndUnsubscribeDoNotSend(t *testing.T) {
	f := newFixture(t, nil)
	outcome, err := f.submit(t, "req-spam-0001", func(in *service.SubmitInput) {
		in.Email = "reader@example.com"
		in.Subscribe = true
	})
	require.NoError(t, err)
	_, err = f.service.Confirm(context.Background(), f.confirmToken(t, outcome.Feedback.ID))
	require.NoError(t, err)
	detail, err := f.service.FeedbackDetail(context.Background(), outcome.Feedback.ID)
	require.NoError(t, err)

	result, err := f.service.UpdateFeedback(context.Background(), "editor", service.AdminUpdateInput{
		ID: detail.ID, Version: detail.Version, Status: model.FeedbackStatusSpam, NotifySubscriber: true,
	})
	require.NoError(t, err)
	require.False(t, result.Notified)
	require.Contains(t, result.NotifySkippedNote, "垃圾信息")

	// Move back to a notifiable state, notify, then unsubscribe.
	detail, err = f.service.FeedbackDetail(context.Background(), outcome.Feedback.ID)
	require.NoError(t, err)
	_, err = f.service.UpdateFeedback(context.Background(), "editor", service.AdminUpdateInput{
		ID: detail.ID, Version: detail.Version, Status: model.FeedbackStatusAccepted, NotifySubscriber: true,
	})
	require.NoError(t, err)
	raw := f.unsubscribeToken(t, outcome.Feedback.ID)
	result2, err := f.service.Unsubscribe(context.Background(), raw)
	require.NoError(t, err)
	require.Equal(t, model.SubscriptionUnsubscribed, result2.SubscriptionStatus)

	for _, task := range f.outbox(t, outcome.Feedback.ID) {
		if task.Type != model.MailTypeUserStatusUpdate {
			continue
		}
		require.Equal(t, model.MailStatusCancelled, task.Status)
	}
}

func TestExpiredAndInvalidTokens(t *testing.T) {
	f := newFixture(t, nil)
	outcome, err := f.submit(t, "req-expire-0001", func(in *service.SubmitInput) {
		in.Email = "reader@example.com"
		in.Subscribe = true
	})
	require.NoError(t, err)
	raw := f.confirmToken(t, outcome.Feedback.ID)
	f.now = f.now.Add(25 * time.Hour)
	_, err = f.service.Confirm(context.Background(), raw)
	require.ErrorIs(t, err, service.ErrTokenExpired)

	_, err = f.service.Confirm(context.Background(), "definitely-not-a-token")
	require.ErrorIs(t, err, service.ErrInvalidToken)
	_, err = f.service.Unsubscribe(context.Background(), "definitely-not-a-token")
	require.ErrorIs(t, err, service.ErrInvalidToken)
}

func TestSubmitRateLimit(t *testing.T) {
	f := newFixture(t, func(s *service.EffectiveSettings) {
		s.RateLimits.SubmitHourly = 1
		s.RateLimits.SubmitDaily = 10
	})
	_, err := f.submit(t, "req-limit-0001", nil)
	require.NoError(t, err)
	_, err = f.submit(t, "req-limit-0002", nil)
	require.ErrorIs(t, err, service.ErrRateLimited)
}

func TestDeleteCleansUp(t *testing.T) {
	f := newFixture(t, nil)
	outcome, err := f.submit(t, "req-delete-0001", func(in *service.SubmitInput) {
		in.Email = "reader@example.com"
		in.Subscribe = true
	})
	require.NoError(t, err)
	require.NoError(t, f.service.DeleteFeedback(context.Background(), "editor", outcome.Feedback.ID))
	_, err = f.service.FeedbackDetail(context.Background(), outcome.Feedback.ID)
	require.ErrorIs(t, err, service.ErrFeedbackNotFound)
	require.Empty(t, f.outbox(t, outcome.Feedback.ID))

	// Deleting twice is a stable 404.
	require.ErrorIs(t, f.service.DeleteFeedback(context.Background(), "editor", outcome.Feedback.ID), service.ErrFeedbackNotFound)
}

func TestResendRotatesTokenAndSupersedesTask(t *testing.T) {
	f := newFixture(t, nil)
	outcome, err := f.submit(t, "req-resend-0001", func(in *service.SubmitInput) {
		in.Email = "reader@example.com"
		in.Subscribe = true
	})
	require.NoError(t, err)
	first := f.confirmToken(t, outcome.Feedback.ID)

	require.NoError(t, f.service.ResendConfirmation(context.Background(), outcome.Feedback.ID, "reader@example.com", "203.0.113.10"))
	second := f.confirmToken(t, outcome.Feedback.ID)
	require.NotEqual(t, first, second)

	pending := 0
	for _, task := range f.outbox(t, outcome.Feedback.ID) {
		if task.Type == model.MailTypeSubscriptionConfirm && task.Status == model.MailStatusPending {
			pending++
		}
	}
	require.Equal(t, 1, pending)

	_, err = f.service.Confirm(context.Background(), first)
	require.ErrorIs(t, err, service.ErrInvalidToken)
	_, err = f.service.Confirm(context.Background(), second)
	require.NoError(t, err)

	// Unknown email and mismatched feedback are silent no-ops.
	require.NoError(t, f.service.ResendConfirmation(context.Background(), outcome.Feedback.ID, "other@example.com", "203.0.113.10"))
}

func TestAdminLoginSessionsAndThrottle(t *testing.T) {
	f := newFixture(t, nil)
	ctx := context.Background()

	identity, err := f.admin.Login(ctx, "editor", "correct-horse-123", "198.51.100.1")
	require.NoError(t, err)
	require.NotEmpty(t, identity.RawToken)
	require.NotEmpty(t, identity.CSRF)

	authenticated, err := f.admin.Authenticate(ctx, identity.RawToken)
	require.NoError(t, err)
	require.True(t, f.admin.VerifyCSRF(authenticated, identity.CSRF))
	require.False(t, f.admin.VerifyCSRF(authenticated, "wrong"))

	f.now = f.now.Add(9 * time.Hour)
	_, err = f.admin.Authenticate(ctx, identity.RawToken)
	require.ErrorIs(t, err, service.ErrUnauthenticated)

	for i := 0; i < 5; i++ {
		_, err := f.admin.Login(ctx, "editor", "wrong-password", "198.51.100.2")
		require.ErrorIs(t, err, service.ErrInvalidCredentials)
	}
	_, err = f.admin.Login(ctx, "editor", "correct-horse-123", "198.51.100.2")
	require.ErrorIs(t, err, service.ErrLoginRateLimited)
}

func TestCredentialRotationInvalidatesSessions(t *testing.T) {
	f := newFixture(t, nil)
	ctx := context.Background()
	identity, err := f.admin.Login(ctx, "editor", "correct-horse-123", "198.51.100.4")
	require.NoError(t, err)

	hash, err := bcrypt.GenerateFromPassword([]byte("another-password"), bcrypt.MinCost)
	require.NoError(t, err)
	rotated := service.NewAdminService(f.store, service.AdminSettings{
		Username: "editor", PasswordHash: string(hash), SessionTTL: 8 * time.Hour,
		Secret: []byte("0123456789abcdef0123456789abcdef"),
	}, func() time.Time { return f.now })
	require.NoError(t, rotated.Startup(ctx))

	_, err = f.admin.Authenticate(ctx, identity.RawToken)
	require.ErrorIs(t, err, service.ErrUnauthenticated)
	_, err = rotated.Authenticate(ctx, identity.RawToken)
	require.ErrorIs(t, err, service.ErrUnauthenticated)
}

func TestAdminPasswordConstraints(t *testing.T) {
	_, err := service.HashPassword("short")
	require.Error(t, err)
	_, err = service.HashPassword(strings.Repeat("a", 73))
	require.ErrorContains(t, err, "72")
	hash, err := service.HashPassword("long-enough-password")
	require.NoError(t, err)
	require.NoError(t, bcrypt.CompareHashAndPassword([]byte(hash), []byte("long-enough-password")))
}
