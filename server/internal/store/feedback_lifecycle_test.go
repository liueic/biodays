package store_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"biodays/server/internal/model"
	"biodays/server/internal/store"

	"github.com/stretchr/testify/require"
)

func TestCleanupAppliesRetention(t *testing.T) {
	st, err := store.OpenFeedback(filepath.Join(t.TempDir(), "feedback.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, st.Close()) })
	ctx := context.Background()
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	closed := now.AddDate(0, 0, -91)

	fb := &model.Feedback{
		ID: "fb-cleanup", RequestID: "req-cleanup-1", RequestBodyHash: "hash",
		Kind: model.FeedbackKindIssue, Title: "标题", Content: "一段足够长的内容。",
		Nickname: "旧昵称", Email: "old@example.com", Status: model.FeedbackStatusCompleted,
		Version: 1, SubscriptionStatus: model.SubscriptionConfirmed, ClosedAt: &closed,
		CreatedAt: closed, UpdatedAt: closed, ConfirmedAt: &closed,
	}
	require.NoError(t, st.CreateSubmission(ctx, store.CreateSubmissionInput{
		Feedback: fb,
		Tokens: []model.SubscriptionToken{{
			TokenHash: "old-token", FeedbackID: fb.ID, Purpose: model.TokenPurposeUnsubscribe,
			EmailFingerprint: "fp", ExpiresAt: now.AddDate(1, 0, 0), CreatedAt: closed,
		}},
		Outbox: []model.MailOutbox{{
			ID: "old-sent", Type: model.MailTypeUserStatusUpdate, EventKey: "status:old",
			Recipient: "old@example.com", Template: "user_status_update", Payload: `{"email":"old@example.com"}`,
			Status: model.MailStatusSent, NextAttemptAt: closed, CreatedAt: now, UpdatedAt: now,
		}},
	}))
	expired := &model.AdminSession{
		TokenHash: "expired-session", CredentialVersion: "v", CSRFHash: "c",
		CreatedAt: closed, ExpiresAt: closed,
	}
	require.NoError(t, st.CreateSession(ctx, expired))

	report, err := st.Cleanup(ctx, now)
	require.NoError(t, err)
	require.Equal(t, int64(1), report.Sessions)
	require.Equal(t, int64(1), report.ContactScrubbed)

	cleaned, err := st.GetFeedback(ctx, fb.ID)
	require.NoError(t, err)
	require.Empty(t, cleaned.Email)
	require.Empty(t, cleaned.Nickname)
	require.Equal(t, model.SubscriptionUnsubscribed, cleaned.SubscriptionStatus)
	require.NotNil(t, cleaned.UnsubscribedAt)

	_, err = st.GetToken(ctx, "old-token", model.TokenPurposeUnsubscribe)
	require.ErrorIs(t, err, store.ErrTokenInvalid)

	task, err := st.GetOutbox(ctx, "old-sent")
	require.NoError(t, err)
	require.Empty(t, task.Payload)
}

func TestOutboxLeaseRecoveryAndClaim(t *testing.T) {
	st, err := store.OpenFeedback(filepath.Join(t.TempDir(), "feedback.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, st.Close()) })
	ctx := context.Background()
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	lease := now.Add(-time.Minute)

	fb := &model.Feedback{
		ID: "fb-lease", RequestID: "req-lease-1", RequestBodyHash: "hash",
		Kind: model.FeedbackKindTopic, Title: "标题", Content: "一段足够长的内容。",
		Status: model.FeedbackStatusPending, Version: 1, SubscriptionStatus: model.SubscriptionNone,
		CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, st.CreateSubmission(ctx, store.CreateSubmissionInput{
		Feedback: fb,
		Outbox: []model.MailOutbox{
			{
				ID: "task-lease", Type: model.MailTypeAdminNewFeedback, EventKey: "admin:lease",
				Recipient: "admin@example.com", Template: "admin_new_feedback",
				Status: model.MailStatusSending, NextAttemptAt: now, LeaseUntil: &lease,
				CreatedAt: now, UpdatedAt: now,
			},
			{
				ID: "task-later", Type: model.MailTypeAdminNewFeedback, EventKey: "admin:later",
				Recipient: "admin@example.com", Template: "admin_new_feedback",
				Status: model.MailStatusPending, NextAttemptAt: now.Add(time.Hour),
				CreatedAt: now, UpdatedAt: now,
			},
		},
	}))

	recovered, err := st.RecoverExpiredLeases(ctx, now)
	require.NoError(t, err)
	require.Equal(t, int64(1), recovered)
	task, err := st.GetOutbox(ctx, "task-lease")
	require.NoError(t, err)
	require.Equal(t, model.MailStatusUnknown, task.Status)
	require.Contains(t, task.LastError, "lease expired")

	claimed, err := st.ClaimOutbox(ctx, now, 2*time.Minute, 10)
	require.NoError(t, err)
	require.Empty(t, claimed, "task-later is not due yet")

	now = now.Add(2 * time.Hour)
	claimed, err = st.ClaimOutbox(ctx, now, 2*time.Minute, 10)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	require.Equal(t, "task-later", claimed[0].ID)
	require.Equal(t, model.MailStatusSending, claimed[0].Status)

	claimed, err = st.ClaimOutbox(ctx, now, 2*time.Minute, 10)
	require.NoError(t, err)
	require.Empty(t, claimed, "sending tasks are leased")
}
