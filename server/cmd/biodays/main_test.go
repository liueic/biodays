package main

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"biodays/server/internal/config"
	"biodays/server/internal/model"
	"biodays/server/internal/store"

	"github.com/stretchr/testify/require"
)

func TestStartupRejectsEmptyDatabase(t *testing.T) {
	for _, dataDir := range []string{"", t.TempDir()} {
		st, err := store.Open(filepath.Join(t.TempDir(), "empty.db"))
		require.NoError(t, err)
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		err = autoSeed(context.Background(), st, config.Config{DataDir: dataDir}, logger)
		require.ErrorContains(t, err, "database has no stories")
		require.NoError(t, st.Close())
	}
}

func TestStartupUsesBundledDatabaseWithoutSeedDirectory(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "bundled.db"))
	require.NoError(t, err)
	defer st.Close()
	require.NoError(t, st.Upsert(context.Background(), &model.Story{
		ID: "Q1-01", Month: 1, Day: 5, Year: 1961, Title: "bundled story",
	}, nil))
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	require.NoError(t, autoSeed(context.Background(), st, config.Config{}, logger))
}
