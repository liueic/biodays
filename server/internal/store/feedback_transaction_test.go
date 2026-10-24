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

func TestCreateSubmissionRollsBackOnOutboxFailure(t *testing.T) {
	st, err := store.OpenFeedback(filepath.Join(t.TempDir(), "feedback.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, st.Close()) })

	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	fb := &model.Feedback{
		ID: "fb-rollback", RequestID: "req-rollback-1", RequestBodyHash: "hash",
		Kind: model.FeedbackKindTopic, Title: "标题", Content: "一段足够长的内容。",
		Status: model.FeedbackStatusPending, Version: 1, SubscriptionStatus: model.SubscriptionNone,
		CreatedAt: now, UpdatedAt: now,
	}
	duplicate := model.MailOutbox{
		ID: "task-a", Type: model.MailTypeAdminNewFeedback, EventKey: "same-key",
		Recipient: "admin@example.com", Template: "admin_new_feedback",
		Status: model.MailStatusPending, NextAttemptAt: now,
	}
	second := duplicate
	second.ID = "task-b"

	err = st.CreateSubmission(context.Background(), store.CreateSubmissionInput{
		Feedback: fb,
		Tokens: []model.SubscriptionToken{{
			TokenHash: "token-hash", FeedbackID: fb.ID, Purpose: model.TokenPurposeConfirm,
			EmailFingerprint: "fp", ExpiresAt: now.Add(time.Hour), CreatedAt: now,
		}},
		Outbox: []model.MailOutbox{duplicate, second},
	})
	require.Error(t, err)

	_, err = st.GetFeedbackByRequestID(context.Background(), fb.RequestID)
	require.ErrorIs(t, err, store.ErrFeedbackNotFound)
	_, err = st.GetToken(context.Background(), "token-hash", model.TokenPurposeConfirm)
	require.ErrorIs(t, err, store.ErrTokenInvalid)
	_, total, err := st.ListOutbox(context.Background(), store.OutboxFilter{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Zero(t, total)
}
