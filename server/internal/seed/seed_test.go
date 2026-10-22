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
