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
		&model.FeedbackDeletion{}, &model.SettingsAudit{},
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
		require.NoError(t, st.Upsert(ctx, story, []string{"仪器极客"}, true, false))

		loaded, err := st.GetByID(ctx, "PG-01")
		require.NoError(t, err)
		require.Equal(t, "移液器", loaded.Title)
		require.Len(t, loaded.Tags, 1)

		story.Title = "更新后"
		require.NoError(t, st.Upsert(ctx, story, []string{"仪器极客", "工业设计"}, true, false))
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

	t.Run("SettingsDocumentRoundTrip", func(t *testing.T) {
		now := time.Now().UTC()
		require.NoError(t, feedback.SetAppSetting(ctx, "runtime_settings", `{"version":1}`, now))
		value, err := feedback.GetAppSetting(ctx, "runtime_settings")
		require.NoError(t, err)
		require.Equal(t, `{"version":1}`, value)

		record := &model.SettingsAudit{Revision: 1, Actor: "editor", Changes: `{"smtp.enabled":{"old":false,"new":true}}`, CreatedAt: now}
		require.NoError(t, feedback.SaveSettingsDocument(ctx, "runtime_settings", `{"version":1,"revision":1}`, record, now))
		value, err = feedback.GetAppSetting(ctx, "runtime_settings")
		require.NoError(t, err)
		require.Contains(t, value, `"revision":1`)
		_, total, err := feedback.ListSettingsAudit(ctx, 1, 20)
		require.NoError(t, err)
		require.EqualValues(t, 1, total)
	})

	t.Run("BackupUnsupported", func(t *testing.T) {
		require.ErrorIs(t, st.BackupTo(ctx, "/tmp/should-not-exist.db"), ErrBackupUnsupported)
		require.ErrorIs(t, feedback.BackupTo(ctx, "/tmp/should-not-exist.db"), ErrBackupUnsupported)
	})
}
