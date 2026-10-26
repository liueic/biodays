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

func newChallengeStore(t *testing.T) *store.FeedbackStore {
	t.Helper()
	st, err := store.OpenFeedback(filepath.Join(t.TempDir(), "feedback.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, st.Close()) })
	return st
}

func testFeedback(id, requestID string, now time.Time) *model.Feedback {
	return &model.Feedback{
		ID: id, RequestID: requestID, RequestBodyHash: "hash-" + id,
		Kind: model.FeedbackKindTopic, Title: "标题", Content: "一段足够长的内容。",
		Status: model.FeedbackStatusPending, Version: 1, SubscriptionStatus: model.SubscriptionNone,
		CreatedAt: now, UpdatedAt: now,
	}
}

func TestChallengeIsConsumedAtomically(t *testing.T) {
	st := newChallengeStore(t)
	ctx := context.Background()
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	challenge := &model.FeedbackChallenge{
		ID: "challenge-1", IPHash: "ip", Salt: "salt", Signature: "signature",
		Difficulty: 2, MinSeconds: 1, IssuedAt: now, ExpiresAt: now.Add(10 * time.Minute),
	}
	require.NoError(t, st.CreateChallenge(ctx, challenge))

	require.NoError(t, st.CreateSubmission(ctx, store.CreateSubmissionInput{
		Feedback: testFeedback("fb-1", "req-1", now), ChallengeID: challenge.ID, Now: now,
	}))
	consumed, err := st.GetChallenge(ctx, challenge.ID)
	require.NoError(t, err)
	require.NotNil(t, consumed.ConsumedAt)
	require.Equal(t, "fb-1", consumed.FeedbackID)

	err = st.CreateSubmission(ctx, store.CreateSubmissionInput{
		Feedback: testFeedback("fb-2", "req-2", now), ChallengeID: challenge.ID, Now: now,
	})
	require.ErrorIs(t, err, store.ErrChallengeConsumed)
	_, err = st.GetFeedbackByRequestID(ctx, "req-2")
	require.ErrorIs(t, err, store.ErrFeedbackNotFound)
}

func TestChallengeRollbackDoesNotConsume(t *testing.T) {
	st := newChallengeStore(t)
	ctx := context.Background()
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	challenge := &model.FeedbackChallenge{
		ID: "challenge-2", IPHash: "ip", Salt: "salt", Signature: "signature",
		Difficulty: 2, MinSeconds: 1, IssuedAt: now, ExpiresAt: now.Add(10 * time.Minute),
	}
	require.NoError(t, st.CreateChallenge(ctx, challenge))

	first := model.MailOutbox{
		ID: "task-a", Type: model.MailTypeAdminNewFeedback, EventKey: "duplicate-key",
		Recipient: "admin@example.com", Template: "admin_new_feedback",
		Status: model.MailStatusPending, NextAttemptAt: now,
	}
	second := first
	second.ID = "task-b"
	err := st.CreateSubmission(ctx, store.CreateSubmissionInput{
		Feedback: testFeedback("fb-3", "req-3", now), ChallengeID: challenge.ID, Now: now,
		Outbox: []model.MailOutbox{first, second},
	})
	require.Error(t, err)

	unchanged, err := st.GetChallenge(ctx, challenge.ID)
	require.NoError(t, err)
	require.Nil(t, unchanged.ConsumedAt)
}

func TestCleanupRemovesExpiredChallenges(t *testing.T) {
	st := newChallengeStore(t)
	ctx := context.Background()
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	require.NoError(t, st.CreateChallenge(ctx, &model.FeedbackChallenge{
		ID: "challenge-expired", IPHash: "ip", Salt: "salt", Signature: "signature",
		Difficulty: 2, MinSeconds: 1, IssuedAt: now.Add(-time.Hour), ExpiresAt: now.Add(-time.Minute),
	}))
	report, err := st.Cleanup(ctx, now)
	require.NoError(t, err)
	require.EqualValues(t, 1, report.Challenges)
	_, err = st.GetChallenge(ctx, "challenge-expired")
	require.ErrorIs(t, err, store.ErrChallengeNotFound)
}
