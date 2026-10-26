package store

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"testing"

	"biodays/server/internal/model"
	"github.com/stretchr/testify/require"
)

func randomFixture(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, st.Close()) })
	for _, id := range []string{"A", "B", "C"} {
		require.NoError(t, st.Upsert(context.Background(), &model.Story{ID: id, Title: id, Content: "content"}, []string{"tag"}, false, false))
	}
	return st
}

func TestRandomUsesFreshEntropyAndAllCandidates(t *testing.T) {
	st := randomFixture(t)
	entropy := bytes.NewReader([]byte{2, 0, 1})
	for _, want := range []string{"C", "A", "B"} {
		story, reset, err := st.random(context.Background(), nil, nil, entropy)
		require.NoError(t, err)
		require.False(t, reset)
		require.Equal(t, want, story.ID)
		require.Len(t, story.Tags, 1)
	}
	_, _, err := st.random(context.Background(), nil, nil, entropy)
	require.ErrorIs(t, err, io.EOF, "entropy failure must not fall back to a predictable choice")
}

func TestRandomRejectsOutOfRangeEntropy(t *testing.T) {
	st := randomFixture(t)
	// For 3 candidates, 3 must be rejected, rather than biased toward A with 3 %% 3.
	story, _, err := st.random(context.Background(), nil, nil, bytes.NewReader([]byte{3, 2}))
	require.NoError(t, err)
	require.Equal(t, "C", story.ID)
}

func TestRandomUnreadRoundResetPreservesCurrentExclusion(t *testing.T) {
	st := randomFixture(t)
	story, reset, err := st.random(context.Background(), []string{"C"}, []string{"A"}, bytes.NewReader(nil))
	require.NoError(t, err)
	require.False(t, reset)
	require.Equal(t, "B", story.ID)
	story, reset, err = st.random(context.Background(), []string{"C"}, []string{"A", "B", "C"}, bytes.NewReader([]byte{1}))
	require.NoError(t, err)
	require.True(t, reset)
	require.Equal(t, "B", story.ID)
}
