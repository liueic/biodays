package store

import (
	"context"
	"os"
	"testing"
	"time"

	"biodays/server/internal/model"

	"github.com/stretchr/testify/require"
)

// The PostgreSQL integration test runs only when BIODAYS_TEST_PG_DSN points at
// a disposable database, e.g.
//   BIODAYS_TEST_PG_DSN='postgres://biodays:biodays@127.0.0.1:5432/biodays_test?sslmode=disable' go test ./internal/store/
// It drops and recreates every biodays table in that database.

func TestPostgresIntegration(t *testing.T) {
	dsn := os.Getenv("BIODAYS_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("BIODAYS_TEST_PG_DSN not set; skipping PostgreSQL integration test")
	}
	ctx := context.Background()

	st, err := OpenPostgres(dsn)
	require.NoError(t, err)
	require.NoError(t, st.db.Migrator().DropTable(
		&model.Story{}, &model.Tag{}, &model.Dataset{},
		&model.Feedback{}, &model.FeedbackEvent{}, &model.AdminSession{},
		&model.SubscriptionToken{}, &model.MailOutbox{}, &model.AbuseBucket{},
		&model.FeedbackChallenge{}, &model.FeedbackDeletion{},
	))
	require.NoError(t, st.Close())

	// Reopening re-runs AutoMigrate, which creates the schema from scratch.
	st, err = OpenPostgres(dsn)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, st.Close()) })
	feedback, err := OpenFeedbackPostgres(dsn)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, feedback.Close()) })

	t.Run("StoryUpsertGetDelete", func(t *testing.T) {
		story := &model.Story{
			ID: "PG-01", Month: 1, Day: 5, Year: 1961,
			Title: "移液器", Content: "content", GeekQuote: "quote",
			SourceType: "patent",
		}
		require.NoError(t, st.Upsert(ctx, story, []string{"仪器极客"}))

		loaded, err := st.GetByID(ctx, "PG-01")
		require.NoError(t, err)
		require.Equal(t, "移液器", loaded.Title)
		require.Len(t, loaded.Tags, 1)

		story.Title = "更新后"
		require.NoError(t, st.Upsert(ctx, story, []string{"仪器极客", "工业设计"}))
		loaded, err = st.GetByID(ctx, "PG-01")
		require.NoError(t, err)
		require.Equal(t, "更新后", loaded.Title)
		require.Len(t, loaded.Tags, 2)

		stories, total, err := st.List(ctx, ListFilter{Page: 1, PageSize: 20, Query: "更新"})
		require.NoError(t, err)
		require.EqualValues(t, 1, total)
		require.Len(t, stories, 1)

		deleted, err := st.DeleteStory(ctx, "PG-01")
		require.NoError(t, err)
		require.True(t, deleted)
		require.NoError(t, st.CleanupOrphanTags(ctx))
	})

	t.Run("RateLimitUpsert", func(t *testing.T) {
		window := time.Now().UTC().Truncate(time.Hour)
		for i := 0; i < 3; i++ {
			allowed, err := feedback.ConsumeRateLimit(ctx, "pg-rate-key", window, window.Add(time.Hour), 2)
			require.NoError(t, err)
			require.Equal(t, i < 2, allowed)
		}
	})

	t.Run("SubmissionTransactionWithChallenge", func(t *testing.T) {
		now := time.Now().UTC()
		challenge := &model.FeedbackChallenge{
			ID: "pg-challenge", IPHash: "ip", Salt: "salt", Signature: "signature",
			Difficulty: 2, MinSeconds: 1, IssuedAt: now, ExpiresAt: now.Add(10 * time.Minute),
		}
		require.NoError(t, feedback.CreateChallenge(ctx, challenge))
		require.NoError(t, feedback.CreateSubmission(ctx, CreateSubmissionInput{
			Feedback: &model.Feedback{
				ID: "pg-fb-1", RequestID: "pg-req-1", RequestBodyHash: "hash",
				Kind: model.FeedbackKindTopic, Title: "标题", Content: "一段足够长的内容。",
				Status: model.FeedbackStatusPending, Version: 1,
				SubscriptionStatus: model.SubscriptionNone, CreatedAt: now, UpdatedAt: now,
			},
			ChallengeID: challenge.ID, Now: now,
		}))
		consumed, err := feedback.GetChallenge(ctx, challenge.ID)
		require.NoError(t, err)
		require.NotNil(t, consumed.ConsumedAt)

		err = feedback.CreateSubmission(ctx, CreateSubmissionInput{
			Feedback: &model.Feedback{
				ID: "pg-fb-2", RequestID: "pg-req-2", RequestBodyHash: "hash",
				Kind: model.FeedbackKindTopic, Title: "标题", Content: "一段足够长的内容。",
				Status: model.FeedbackStatusPending, Version: 1,
				SubscriptionStatus: model.SubscriptionNone, CreatedAt: now, UpdatedAt: now,
			},
			ChallengeID: challenge.ID, Now: now,
		})
		require.ErrorIs(t, err, ErrChallengeConsumed)
	})

	t.Run("BackupUnsupported", func(t *testing.T) {
		require.ErrorIs(t, st.BackupTo(ctx, "/tmp/should-not-exist.db"), ErrBackupUnsupported)
		require.ErrorIs(t, feedback.BackupTo(ctx, "/tmp/should-not-exist.db"), ErrBackupUnsupported)
	})
}
