package mailer_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"biodays/server/internal/mailer"
	"biodays/server/internal/model"
	"biodays/server/internal/security"
	"biodays/server/internal/store"

	"github.com/stretchr/testify/require"
)

type fakeSender struct {
	mu   sync.Mutex
	msgs []mailer.Message
	err  error
}

func (f *fakeSender) Send(_ context.Context, msg mailer.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.msgs = append(f.msgs, msg)
	return f.err
}

func (f *fakeSender) messages() []mailer.Message {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]mailer.Message(nil), f.msgs...)
}

type workerFixture struct {
	worker *mailer.Worker
	sender *fakeSender
	store  *store.FeedbackStore
	now    time.Time
}

func newWorkerFixture(t *testing.T) *workerFixture {
	t.Helper()
	st, err := store.OpenFeedback(filepath.Join(t.TempDir(), "feedback.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, st.Close()) })
	f := &workerFixture{
		sender: &fakeSender{},
		store:  st,
		now:    time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC),
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	f.worker = mailer.NewWorker(st, f.sender, mailer.NewRenderer("biodays", "https://biodays.test"),
		logger, func() time.Time { return f.now }, []byte("0123456789abcdef0123456789abcdef"), "admin@biodays.test")
	return f
}

func (f *workerFixture) feedback(t *testing.T, status, subscription string) *model.Feedback {
	t.Helper()
	now := f.now
	fb := &model.Feedback{
		ID: "fb-test-0001", RequestID: "req-worker-0001", RequestBodyHash: "hash",
		Kind: model.FeedbackKindTopic, Title: "标题", Content: "一段足够长的内容用于测试。",
		Email: "reader@example.com", Status: status, Version: 1,
		SubscriptionStatus: subscription, CreatedAt: now, UpdatedAt: now,
	}
	if subscription == model.SubscriptionConfirmed {
		fb.ConfirmedAt = &now
	}
	return fb
}

func (f *workerFixture) create(t *testing.T, fb *model.Feedback, tokens []model.SubscriptionToken, outboxes []model.MailOutbox) {
	t.Helper()
	require.NoError(t, f.store.CreateSubmission(context.Background(), store.CreateSubmissionInput{
		Feedback: fb, Tokens: tokens, Outbox: outboxes,
	}))
}

func payload(t *testing.T, data mailer.TemplateData) string {
	t.Helper()
	encoded, err := json.Marshal(data)
	require.NoError(t, err)
	return string(encoded)
}

func TestWorkerSendsAndClearsPayload(t *testing.T) {
	f := newWorkerFixture(t)
	fb := f.feedback(t, model.FeedbackStatusPending, model.SubscriptionNone)
	outbox := model.MailOutbox{
		ID: "task-admin-1", Type: model.MailTypeAdminNewFeedback, EventKey: "admin:1",
		Recipient: "admin@biodays.test", Template: "admin_new_feedback",
		Payload: payload(t, mailer.TemplateData{FeedbackID: fb.ID, Title: fb.Title, Content: fb.Content}),
		Status:  model.MailStatusPending, NextAttemptAt: f.now,
	}
	f.create(t, fb, nil, []model.MailOutbox{outbox})

	sent, err := f.worker.ProcessOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, sent)
	require.Len(t, f.sender.messages(), 1)

	stored, err := f.store.GetOutbox(context.Background(), "task-admin-1")
	require.NoError(t, err)
	require.Equal(t, model.MailStatusSent, stored.Status)
	require.Empty(t, stored.Payload)
	require.NotNil(t, stored.SMTPAcceptedAt)
}

func TestWorkerRetriesTemporaryErrors(t *testing.T) {
	f := newWorkerFixture(t)
	f.sender.err = &mailer.DeliveryError{Err: errors.New("connection reset")}
	fb := f.feedback(t, model.FeedbackStatusPending, model.SubscriptionNone)
	outbox := model.MailOutbox{
		ID: "task-temp-1", Type: model.MailTypeAdminNewFeedback, EventKey: "admin:temp",
		Recipient: "admin@biodays.test", Template: "admin_new_feedback",
		Payload: payload(t, mailer.TemplateData{FeedbackID: fb.ID}), Status: model.MailStatusPending,
		NextAttemptAt: f.now,
	}
	f.create(t, fb, nil, []model.MailOutbox{outbox})

	_, err := f.worker.ProcessOnce(context.Background())
	require.NoError(t, err)
	stored, err := f.store.GetOutbox(context.Background(), "task-temp-1")
	require.NoError(t, err)
	require.Equal(t, model.MailStatusFailed, stored.Status)
	require.Equal(t, 1, stored.Attempts)
	require.Equal(t, f.now.Add(time.Minute), stored.NextAttemptAt)
}

func TestWorkerStopsAfterFiveAttempts(t *testing.T) {
	f := newWorkerFixture(t)
	fb := f.feedback(t, model.FeedbackStatusPending, model.SubscriptionNone)
	outbox := model.MailOutbox{
		ID: "task-exhaust-1", Type: model.MailTypeAdminNewFeedback, EventKey: "admin:exhaust",
		Recipient: "admin@biodays.test", Template: "admin_new_feedback",
		Payload: payload(t, mailer.TemplateData{FeedbackID: fb.ID}), Status: model.MailStatusPending,
		Attempts: 4, NextAttemptAt: f.now,
	}
	f.create(t, fb, nil, []model.MailOutbox{outbox})
	f.sender.err = &mailer.DeliveryError{Err: errors.New("still down")}

	_, err := f.worker.ProcessOnce(context.Background())
	require.NoError(t, err)
	stored, err := f.store.GetOutbox(context.Background(), "task-exhaust-1")
	require.NoError(t, err)
	require.Equal(t, model.MailStatusFailed, stored.Status)
	require.Contains(t, stored.LastError, "retries exhausted")
}

func TestWorkerPermanentAndAmbiguousErrors(t *testing.T) {
	f := newWorkerFixture(t)
	f.sender.err = &mailer.DeliveryError{Permanent: true, Err: errors.New("550 rejected")}
	fb := f.feedback(t, model.FeedbackStatusPending, model.SubscriptionNone)
	permanent := model.MailOutbox{
		ID: "task-perm-1", Type: model.MailTypeAdminNewFeedback, EventKey: "admin:perm",
		Recipient: "admin@biodays.test", Template: "admin_new_feedback",
		Payload: payload(t, mailer.TemplateData{FeedbackID: fb.ID}), Status: model.MailStatusPending,
		NextAttemptAt: f.now,
	}
	f.create(t, fb, nil, []model.MailOutbox{permanent})
	_, err := f.worker.ProcessOnce(context.Background())
	require.NoError(t, err)
	stored, err := f.store.GetOutbox(context.Background(), "task-perm-1")
	require.NoError(t, err)
	require.Equal(t, model.MailStatusFailed, stored.Status)

	f.sender.err = &mailer.DeliveryError{Ambiguous: true, Err: errors.New("connection lost after DATA")}
	fb2 := f.feedback(t, model.FeedbackStatusPending, model.SubscriptionNone)
	fb2.ID = "fb-test-0002"
	fb2.RequestID = "req-worker-0002"
	ambiguous := model.MailOutbox{
		ID: "task-amb-1", Type: model.MailTypeAdminNewFeedback, EventKey: "admin:amb",
		Recipient: "admin@biodays.test", Template: "admin_new_feedback",
		Payload: payload(t, mailer.TemplateData{FeedbackID: fb2.ID}), Status: model.MailStatusPending,
		NextAttemptAt: f.now,
	}
	f.create(t, fb2, nil, []model.MailOutbox{ambiguous})
	_, err = f.worker.ProcessOnce(context.Background())
	require.NoError(t, err)
	stored, err = f.store.GetOutbox(context.Background(), "task-amb-1")
	require.NoError(t, err)
	require.Equal(t, model.MailStatusUnknown, stored.Status)
}

func TestWorkerCancelsAfterUnsubscribe(t *testing.T) {
	f := newWorkerFixture(t)
	fb := f.feedback(t, model.FeedbackStatusPending, model.SubscriptionPendingConfirmation)
	raw, err := security.RandomToken()
	require.NoError(t, err)
	token := model.SubscriptionToken{
		TokenHash: security.HashToken(raw), FeedbackID: fb.ID, Purpose: model.TokenPurposeConfirm,
		EmailFingerprint: "fingerprint", ExpiresAt: f.now.Add(time.Hour), CreatedAt: f.now,
	}
	unsubRaw, err := security.RandomToken()
	require.NoError(t, err)
	unsub := model.SubscriptionToken{
		TokenHash: security.HashToken(unsubRaw), FeedbackID: fb.ID, Purpose: model.TokenPurposeUnsubscribe,
		EmailFingerprint: "fingerprint", ExpiresAt: f.now.Add(100 * time.Hour), CreatedAt: f.now,
	}
	outbox := model.MailOutbox{
		ID: "task-confirm-1", Type: model.MailTypeSubscriptionConfirm, EventKey: "confirm:cancel",
		Recipient: fb.Email, Template: "subscription_confirm",
		Payload: payload(t, mailer.TemplateData{FeedbackID: fb.ID, ConfirmURL: mailer.ConfirmURL("https://biodays.test", raw), TokenHash: token.TokenHash}),
		Status:  model.MailStatusPending, NextAttemptAt: f.now,
	}
	f.create(t, fb, []model.SubscriptionToken{token, unsub}, []model.MailOutbox{outbox})

	_, _, err = f.store.Unsubscribe(context.Background(), unsub.TokenHash, f.now)
	require.NoError(t, err)

	_, err = f.worker.ProcessOnce(context.Background())
	require.NoError(t, err)
	require.Empty(t, f.sender.messages())
	stored, err := f.store.GetOutbox(context.Background(), "task-confirm-1")
	require.NoError(t, err)
	require.Equal(t, model.MailStatusCancelled, stored.Status)
}

func TestWorkerRegeneratesExpiredConfirmationLink(t *testing.T) {
	f := newWorkerFixture(t)
	fb := f.feedback(t, model.FeedbackStatusPending, model.SubscriptionPendingConfirmation)
	oldRaw, err := security.RandomToken()
	require.NoError(t, err)
	token := model.SubscriptionToken{
		TokenHash: security.HashToken(oldRaw), FeedbackID: fb.ID, Purpose: model.TokenPurposeConfirm,
		EmailFingerprint: "fingerprint", ExpiresAt: f.now.Add(-time.Hour), CreatedAt: f.now.Add(-25 * time.Hour),
	}
	outbox := model.MailOutbox{
		ID: "task-confirm-expired", Type: model.MailTypeSubscriptionConfirm, EventKey: "confirm:expired",
		Recipient: fb.Email, Template: "subscription_confirm",
		Payload: payload(t, mailer.TemplateData{FeedbackID: fb.ID, ConfirmURL: mailer.ConfirmURL("https://biodays.test", oldRaw), TokenHash: token.TokenHash}),
		Status:  model.MailStatusPending, NextAttemptAt: f.now,
	}
	f.create(t, fb, []model.SubscriptionToken{token}, []model.MailOutbox{outbox})

	_, err = f.worker.ProcessOnce(context.Background())
	require.NoError(t, err)
	require.Len(t, f.sender.messages(), 1)
	msg := f.sender.messages()[0]
	index := strings.Index(msg.Text, "#token=")
	require.GreaterOrEqual(t, index, 0)
	newRaw := strings.SplitN(msg.Text[index+len("#token="):], "\n", 2)[0]
	require.NotEqual(t, oldRaw, newRaw)

	validated, err := f.store.GetToken(context.Background(), security.HashToken(newRaw), model.TokenPurposeConfirm)
	require.NoError(t, err)
	require.True(t, validated.ExpiresAt.After(f.now))
}

func TestWorkerSkipsStatusMailWhenNoLongerEligible(t *testing.T) {
	f := newWorkerFixture(t)
	fb := f.feedback(t, model.FeedbackStatusPending, model.SubscriptionConfirmed)
	outbox := model.MailOutbox{
		ID: "task-status-1", Type: model.MailTypeUserStatusUpdate, EventKey: "status:1",
		Recipient: fb.Email, Template: "user_status_update",
		Payload: payload(t, mailer.TemplateData{FeedbackID: fb.ID, Status: model.FeedbackStatusAccepted, UnsubscribeURL: "https://biodays.test/feedback/unsubscribe#token=x"}),
		Status:  model.MailStatusPending, NextAttemptAt: f.now,
	}
	f.create(t, fb, nil, []model.MailOutbox{outbox})

	_, err := f.worker.ProcessOnce(context.Background())
	require.NoError(t, err)
	require.Empty(t, f.sender.messages())
	stored, err := f.store.GetOutbox(context.Background(), "task-status-1")
	require.NoError(t, err)
	require.Equal(t, model.MailStatusCancelled, stored.Status)
}

func TestRendererDoesNotLeakContentIntoConfirmation(t *testing.T) {
	renderer := mailer.NewRenderer("biodays", "https://biodays.test")
	msg, err := renderer.Render(model.MailTypeSubscriptionConfirm, mailer.TemplateData{
		FeedbackID: "abc", ConfirmURL: "https://biodays.test/feedback/confirm#token=x",
		Content: "未验证的自由文本不应出现",
	})
	require.NoError(t, err)
	require.NotContains(t, msg.Text, "未验证的自由文本")
	require.NotContains(t, msg.HTML, "未验证的自由文本")
	require.Contains(t, msg.Text, "#token=x")
}
