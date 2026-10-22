package service_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"biodays/server/internal/model"
	"biodays/server/internal/service"
	"biodays/server/internal/store"

	"github.com/stretchr/testify/require"
)

func newStore(t *testing.T) *store.Store {
	t.Helper()
	name := strings.ReplaceAll(t.Name(), "/", "_")
	st, err := store.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", name))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, st.Close()) })
	return st
}

func seedStory(t *testing.T, st *store.Store, id string, month, day, year int) {
	t.Helper()
	require.NoError(t, st.Upsert(context.Background(), &model.Story{
		ID: id, Month: month, Day: day, Year: year,
		Title: id, Content: "content", GeekQuote: "quote",
	}, []string{"tag"}))
}

func TestParseOffset(t *testing.T) {
	location, err := service.ParseOffset("+08:00")
	require.NoError(t, err)
	_, offset := time.Now().In(location).Zone()
	require.Equal(t, 8*3600, offset)

	location, err = service.ParseOffset("-05:30")
	require.NoError(t, err)
	_, offset = time.Now().In(location).Zone()
	require.Equal(t, -(5*3600 + 30*60), offset)

	location, err = service.ParseOffset("Z")
	require.NoError(t, err)
	require.Equal(t, time.UTC, location)

	for _, invalid := range []string{"8", "+15:00", "+08:60", "08:00", "+8:00", "Asia/Shanghai"} {
		_, err := service.ParseOffset(invalid)
		require.ErrorIs(t, err, service.ErrInvalidOffset, invalid)
	}
}

func TestTodayRespectsTimezone(t *testing.T) {
	st := newStore(t)
	seedStory(t, st, "Q4-02", 10, 16, 1980)
	seedStory(t, st, "Q4-01", 10, 9, 2002)

	// 2026-10-15 20:00 UTC is already 2026-10-16 in +08:00.
	clock := func() time.Time { return time.Date(2026, 10, 15, 20, 0, 0, 0, time.UTC) }
	svc := service.NewStoryServiceWithClock(st, clock)

	story, isFallback, err := svc.Today(context.Background(), "+08:00", service.FallbackRandom)
	require.NoError(t, err)
	require.False(t, isFallback)
	require.Equal(t, "Q4-02", story.ID)

	_, isFallback, err = svc.Today(context.Background(), "+00:00", service.FallbackRandom)
	require.NoError(t, err)
	require.True(t, isFallback)

	_, _, err = svc.Today(context.Background(), "+00:00", service.FallbackNone)
	require.ErrorIs(t, err, service.ErrNotFound)
}

func TestTodayFallsBackToRandom(t *testing.T) {
	st := newStore(t)
	seedStory(t, st, "Q4-02", 10, 16, 1980)

	clock := func() time.Time { return time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC) }
	svc := service.NewStoryServiceWithClock(st, clock)

	story, isFallback, err := svc.Today(context.Background(), "UTC", service.FallbackRandom)
	require.NoError(t, err)
	require.True(t, isFallback)
	require.Equal(t, "Q4-02", story.ID)

	_, _, err = svc.Today(context.Background(), "UTC", service.FallbackNone)
	require.ErrorIs(t, err, service.ErrNotFound)

	_, _, err = svc.Today(context.Background(), "not-a-tz", service.FallbackRandom)
	require.ErrorIs(t, err, service.ErrInvalidOffset)
}

func TestBuildMeta(t *testing.T) {
	meta := service.BuildMeta(104, []int{1, 2, 3, 10, 11, 12})
	require.EqualValues(t, 104, meta.Total)
	require.Equal(t, []string{"Q1", "Q4"}, meta.Quarters)
	require.Equal(t, "1-3月 / 10-12月", meta.Coverage)

	require.Equal(t, "1月 / 3月", service.FormatMonthRanges([]int{3, 1}))
	require.Equal(t, "", service.FormatMonthRanges(nil))
}
