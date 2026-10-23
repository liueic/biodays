package seed_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"biodays/server/internal/seed"
	"biodays/server/internal/service"
	"biodays/server/internal/store"

	"github.com/stretchr/testify/require"
)

func TestBundledDataset(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "biodays.db"))
	require.NoError(t, err)
	defer st.Close()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	report, err := seed.Run(context.Background(), st, "../../../data", logger)
	require.NoError(t, err)
	raw, err := os.ReadFile("../../../data/manifest.json")
	require.NoError(t, err)
	var manifest seed.Manifest
	require.NoError(t, json.Unmarshal(raw, &manifest))
	require.Equal(t, len(manifest.Files), report.Files)
	require.Equal(t, manifest.Total, report.Imported)
	require.Equal(t, manifest.Version, report.Version)
	require.Zero(t, report.Skipped)
	require.Zero(t, report.FailedFiles)
	total, months, err := st.Stats(context.Background())
	require.NoError(t, err)
	require.EqualValues(t, report.Imported, total)
	require.Equal(t, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}, months)
	for _, id := range []string{"BIO-C1-01", "BIO-C2-01", "BIO-C3-01", "BIO-C4-01", "BIO-C4-50"} {
		story, err := st.GetByID(context.Background(), id)
		require.NoError(t, err, id)
		require.NotEmpty(t, story.Content, id)
		require.NotEmpty(t, story.Tags, id)
	}
	story, _, err := service.NewStoryService(st).Today(context.Background(), "+08:00", service.FallbackRandom)
	require.NoError(t, err)
	require.NotEmpty(t, story.ID)
	meta, err := service.NewStoryService(st).Meta(context.Background())
	require.NoError(t, err)
	require.Equal(t, manifest.Version, meta.DatasetVersion)
	var counted int64
	for _, count := range meta.MonthCounts {
		counted += count
	}
	require.EqualValues(t, manifest.Total, counted)
}
