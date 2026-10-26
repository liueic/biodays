package seed_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"biodays/server/internal/model"
	"biodays/server/internal/seed"
	"biodays/server/internal/store"

	"github.com/stretchr/testify/require"
)

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
}

func storyJSON(id string, month int, title string) string {
	return fmt.Sprintf(`{
		"id": %q, "month": %d, "day": 5, "year": 1961,
		"title": %q, "lab_item": "移液枪", "tags": ["仪器极客", "仪器极客", " 工业设计 "],
		"summary": "summary", "content": "content", "geek_quote": "quote",
		"source": {"type": "patent", "citation": "citation", "url": "https://example.com"}
	}`, id, month, title)
}

func TestRunImportsValidEntriesAndSkipsInvalid(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "stories_q1.json", "["+
		storyJSON("Q1-01", 1, "one")+","+
		storyJSON("Q1-02", 13, "bad month")+","+
		storyJSON("Q1-03", 2, "three")+"]")
	writeFile(t, dir, "stories_q4.json", "["+
		storyJSON("Q1-01", 1, "duplicate")+","+
		storyJSON("Q4-01", 10, "four")+"]")
	writeFile(t, dir, "stories_broken.json", "{not json")

	name := strings.ReplaceAll(t.Name(), "/", "_")
	st, err := store.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", name))
	require.NoError(t, err)
	defer st.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	report, err := seed.Run(context.Background(), st, dir, logger)
	require.NoError(t, err)
	require.Equal(t, 3, report.Files)
	require.Equal(t, 3, report.Imported)
	require.Equal(t, 2, report.Skipped)
	require.Equal(t, 1, report.FailedFiles)

	total, _, err := st.Stats(context.Background())
	require.NoError(t, err)
	require.EqualValues(t, 3, total)

	story, err := st.GetByID(context.Background(), "Q1-01")
	require.NoError(t, err)
	require.Equal(t, "one", story.Title, "first valid duplicate wins")
	require.Len(t, story.Tags, 2, "duplicate and whitespace tags are normalised")

	report, err = seed.Run(context.Background(), st, dir, logger)
	require.NoError(t, err)
	require.Equal(t, 3, report.Imported)
	total, _, err = st.Stats(context.Background())
	require.NoError(t, err)
	require.EqualValues(t, 3, total, "second run is idempotent")
}

func TestRunRequiresData(t *testing.T) {
	dir := t.TempDir()
	name := strings.ReplaceAll(t.Name(), "/", "_")
	st, err := store.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", name))
	require.NoError(t, err)
	defer st.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	_, err = seed.Run(context.Background(), st, dir, logger)
	require.ErrorContains(t, err, "no stories_*.json")
}

// incrementStore opens an isolated in-memory store for incremental tests.
func incrementStore(t *testing.T) *store.Store {
	t.Helper()
	name := strings.ReplaceAll(t.Name(), "/", "_")
	st, err := store.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", name))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, st.Close()) })
	return st
}

func TestRunIncrementalImportsOnlyWhenDatasetChanges(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "stories_q1.json", "["+storyJSON("Q1-01", 1, "one")+"]")
	st := incrementStore(t)
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	report, err := seed.RunIncremental(ctx, st, dir, logger)
	require.NoError(t, err)
	require.Equal(t, 1, report.Imported)
	require.False(t, report.Unchanged)

	// Same fingerprint: nothing runs.
	report, err = seed.RunIncremental(ctx, st, dir, logger)
	require.NoError(t, err)
	require.True(t, report.Unchanged)
	require.Equal(t, 0, report.Imported)

	// A changed corpus imports the update.
	writeFile(t, dir, "stories_q1.json", "["+storyJSON("Q1-01", 1, "one v2")+"]")
	report, err = seed.RunIncremental(ctx, st, dir, logger)
	require.NoError(t, err)
	require.Equal(t, 1, report.Imported)
	story, err := st.GetByID(ctx, "Q1-01")
	require.NoError(t, err)
	require.Equal(t, "one v2", story.Title)
}

func TestRunIncrementalPreservesAdminEditsAndTombstones(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "stories_q1.json", "["+
		storyJSON("Q1-01", 1, "original")+","+
		storyJSON("Q1-02", 1, "keep me")+","+
		storyJSON("Q1-03", 1, "to delete")+"]")
	st := incrementStore(t)
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	_, err := seed.RunIncremental(ctx, st, dir, logger)
	require.NoError(t, err)

	// Simulate admin edits: modify Q1-01 through the admin write path and
	// delete Q1-03 (a dataset story, which must leave a tombstone).
	edited := modelStory("Q1-01", "admin title")
	require.NoError(t, st.Upsert(ctx, edited, []string{"仪器极客"}, true, true))
	_, err = st.DeleteStory(ctx, "Q1-03")
	require.NoError(t, err)
	require.NoError(t, st.AddTombstone(ctx, "Q1-03", time.Now()))

	// Ship a new corpus version: Q1-01 changed upstream, Q1-03 is still there.
	writeFile(t, dir, "stories_q1.json", "["+
		storyJSON("Q1-01", 1, "upstream title")+","+
		storyJSON("Q1-02", 1, "keep me")+","+
		storyJSON("Q1-03", 1, "to delete")+","+
		storyJSON("Q1-04", 1, "brand new")+"]")
	report, err := seed.RunIncremental(ctx, st, dir, logger)
	require.NoError(t, err)
	require.Equal(t, 2, report.Preserved, "edited + tombstoned stories are counted")
	require.Equal(t, 2, report.Imported, "unchanged story re-upserted and new story added")

	story, err := st.GetByID(ctx, "Q1-01")
	require.NoError(t, err)
	require.Equal(t, "admin title", story.Title, "admin edit survives the import")
	require.True(t, story.AdminEdited)

	_, err = st.GetByID(ctx, "Q1-03")
	require.ErrorIs(t, err, store.ErrNotFound, "deleted story is not resurrected")

	_, err = st.GetByID(ctx, "Q1-04")
	require.NoError(t, err)

	// The manual Run resets to the canon: edits reverted, deleted story back.
	report, err = seed.Run(ctx, st, dir, logger)
	require.NoError(t, err)
	require.Equal(t, 4, report.Imported)
	story, err = st.GetByID(ctx, "Q1-01")
	require.NoError(t, err)
	require.Equal(t, "upstream title", story.Title)
	require.False(t, story.AdminEdited)
	_, err = st.GetByID(ctx, "Q1-03")
	require.NoError(t, err, "manual seed clears tombstones")
}

func modelStory(id, title string) *model.Story {
	return &model.Story{
		ID: id, Month: 1, Day: 5, Year: 1961,
		Title: title, Content: "content", GeekQuote: "quote",
	}
}
