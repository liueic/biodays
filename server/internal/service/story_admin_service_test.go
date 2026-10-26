package service_test

import (
	"context"
	"testing"
	"time"

	"biodays/server/internal/seed"
	"biodays/server/internal/service"
	"biodays/server/internal/store"

	"github.com/stretchr/testify/require"
)

func validStory(id string) seed.StoryInput {
	return seed.StoryInput{
		ID: id, Month: 1, Day: 5, Year: 1961,
		Title: "移液器", Content: "长正文", GeekQuote: "quote",
		Tags: []string{"仪器极客"}, Summary: "摘要", LabItem: "移液枪",
		Source: seed.SourceInput{Type: "patent", Citation: "DE 1090449", URL: "https://example.com/1"},
	}
}

func newStoryAdmin(t *testing.T) (*service.StoryAdminService, *store.Store) {
	t.Helper()
	st := newStore(t)
	return service.NewStoryAdminService(st, time.Now), st
}

func TestStoryAdminUpsertCreatesThenUpdates(t *testing.T) {
	svc, st := newStoryAdmin(t)
	ctx := context.Background()

	item, err := svc.Upsert(ctx, validStory("NEW-01"))
	require.NoError(t, err)
	require.True(t, item.Created)
	require.Empty(t, item.Problems)

	updated := validStory("NEW-01")
	updated.Title = "改过的标题"
	item, err = svc.Upsert(ctx, updated)
	require.NoError(t, err)
	require.False(t, item.Created)

	stories, total, err := st.List(ctx, store.ListFilter{Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, "改过的标题", stories[0].Title)
}

func TestStoryAdminUpsertRejectsInvalid(t *testing.T) {
	svc, _ := newStoryAdmin(t)
	entry := validStory("BAD-01")
	entry.Month = 13
	item, err := svc.Upsert(context.Background(), entry)
	require.NoError(t, err)
	require.False(t, item.Created)
	require.Contains(t, item.Problems, "month 13 out of range 1-12")
}

func TestStoryAdminBatchReportsPerItem(t *testing.T) {
	svc, _ := newStoryAdmin(t)
	ctx := context.Background()

	good := validStory("B-01")
	bad := validStory("B-02")
	bad.Month = 2
	bad.Day = 30 // invalid calendar date

	result, err := svc.UpsertBatch(ctx, []seed.StoryInput{good, bad, validStory("B-03")})
	require.NoError(t, err)
	require.Equal(t, 2, result.Accepted)
	require.Equal(t, 1, result.Rejected)
	require.Len(t, result.Items, 3)
	require.Empty(t, result.Items[0].Problems)
	require.NotEmpty(t, result.Items[1].Problems)

	_, err = svc.UpsertBatch(ctx, make([]seed.StoryInput, service.MaxStoryBatch+1))
	require.ErrorIs(t, err, service.ErrStoryBatchTooLarge)
}

func TestStoryAdminPatchKeepsUnspecifiedFields(t *testing.T) {
	svc, _ := newStoryAdmin(t)
	ctx := context.Background()
	_, err := svc.Upsert(ctx, validStory("P-01"))
	require.NoError(t, err)

	newTitle := "只改标题"
	story, problems, err := svc.Update(ctx, "P-01", service.StoryPatch{Title: &newTitle})
	require.NoError(t, err)
	require.Empty(t, problems)
	require.Equal(t, newTitle, story.Title)
	require.Equal(t, 1, story.Month)
	require.Equal(t, "长正文", story.Content)
	require.Equal(t, []string{"仪器极客"}, story.Tags)
	require.Equal(t, "patent", story.Source.Type)

	newURL := "https://example.com/updated"
	story, problems, err = svc.Update(ctx, "P-01", service.StoryPatch{
		Source: &service.StoryPatchSource{URL: &newURL},
	})
	require.NoError(t, err)
	require.Empty(t, problems)
	require.Equal(t, newURL, story.Source.URL)
	require.Equal(t, "DE 1090449", story.Source.Citation)

	empty := ""
	_, problems, err = svc.Update(ctx, "P-01", service.StoryPatch{Title: &empty})
	require.NoError(t, err)
	require.Contains(t, problems, "title is empty")

	_, _, err = svc.Update(ctx, "missing", service.StoryPatch{})
	require.ErrorIs(t, err, store.ErrNotFound)
}

func TestStoryAdminDelete(t *testing.T) {
	svc, st := newStoryAdmin(t)
	ctx := context.Background()
	_, err := svc.Upsert(ctx, validStory("D-01"))
	require.NoError(t, err)

	require.NoError(t, svc.Delete(ctx, "D-01"))
	require.ErrorIs(t, svc.Delete(ctx, "D-01"), store.ErrNotFound)

	// The shared tag should be cleaned up with the last story gone.
	tags, err := st.TagCounts(ctx)
	require.NoError(t, err)
	require.Empty(t, tags)
}
