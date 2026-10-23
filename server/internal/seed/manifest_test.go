package seed

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

func TestManifestRejectsStaleDataAndWrongCounts(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "stories_test.json")
	require.NoError(t, os.WriteFile(file, []byte(`[{"id":"test"}]`), 0600))
	version, _, err := fingerprint([]string{file}, dir)
	require.NoError(t, err)
	for _, test := range []struct {
		name     string
		manifest Manifest
		valid    bool
	}{
		{"valid", Manifest{version, 1, map[string]int{"stories_test.json": 1}}, true},
		{"stale hash", Manifest{"sha256:old", 1, map[string]int{"stories_test.json": 1}}, false},
		{"wrong count", Manifest{version, 2, map[string]int{"stories_test.json": 2}}, false},
		{"wrong name", Manifest{version, 1, map[string]int{"stories_other.json": 1}}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			raw, err := json.Marshal(test.manifest)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest.json"), raw, 0600))
			_, _, err = fingerprint([]string{file}, dir)
			if test.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestValidateCalendarDate(t *testing.T) {
	entry := inputStory{ID: "date", Year: 2025, Month: 2, Day: 29, Title: "title", Content: "body", GeekQuote: "quote"}
	require.Contains(t, validate(entry), "invalid calendar date")
	entry.Year = 2024
	require.Empty(t, validate(entry))
}
