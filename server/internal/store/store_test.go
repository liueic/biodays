package store_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"biodays/server/internal/model"
	"biodays/server/internal/store"

	"github.com/stretchr/testify/require"
)

var storeSeq int

func newStore(t *testing.T) *store.Store {
	t.Helper()
	storeSeq++
	name := fmt.Sprintf("%s_%d", strings.ReplaceAll(t.Name(), "/", "_"), storeSeq)
	st, err := store.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", name))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, st.Close()) })
	return st
}

func upsert(t *testing.T, st *store.Store, story model.Story, tags ...string) {
	t.Helper()
	require.NoError(t, st.Upsert(context.Background(), &story, tags))
}

func TestPing(t *testing.T) {
	st := newStore(t)
	require.NoError(t, st.Ping(context.Background()))
}

func fixtures(t *testing.T) *store.Store {
	t.Helper()
	st := newStore(t)
	upsert(t, st, model.Story{
		ID: "Q1-01", Month: 1, Day: 5, Year: 1961,
		Title: "移液器", Summary: "嘴吸时代终结", Content: "content", GeekQuote: "quote", LabItem: "移液枪",
		SourceType: "patent", SourceCitation: "DE 1090449", SourceURL: "https://example.com/1",
	}, "仪器极客", "工业设计")
	upsert(t, st, model.Story{
		ID: "Q4-01", Month: 10, Day: 9, Year: 2002,
		Title: "质谱诺奖", Summary: "软电离", Content: "content", GeekQuote: "quote", LabItem: "质谱仪",
		SourceType: "nobel | paper", SourceCitation: "Science 246", SourceURL: "https://example.com/2",
	}, "仪器极客", "质谱与组学")
	upsert(t, st, model.Story{
		ID: "Q4-02", Month: 10, Day: 16, Year: 1980,
		Title: "液氮研钵", Summary: "RNase 冻结", Content: "content", GeekQuote: "quote", LabItem: "研钵",
		SourceType: "paper", SourceCitation: "Biochemistry 18", SourceURL: "https://example.com/3",
	}, "样品前处理")
	return st
}

func TestGetByID(t *testing.T) {
	st := fixtures(t)
	story, err := st.GetByID(context.Background(), "Q1-01")
	require.NoError(t, err)
	require.Equal(t, "移液器", story.Title)
	require.Len(t, story.Tags, 2)
	require.Equal(t, "仪器极客", story.Tags[0].Name)

	_, err = st.GetByID(context.Background(), "Q2-99")
	require.ErrorIs(t, err, store.ErrNotFound)
}

func TestGetByMonthDay(t *testing.T) {
	st := fixtures(t)
	stories, err := st.GetByMonthDay(context.Background(), 10, 1)
	require.NoError(t, err)
	require.Empty(t, stories)

	upsert(t, st, model.Story{
		ID: "Q4-03", Month: 10, Day: 1, Year: 1990, Title: "晚", Content: "c", GeekQuote: "q",
	})
	upsert(t, st, model.Story{
		ID: "Q4-04", Month: 10, Day: 1, Year: 1970, Title: "早", Content: "c", GeekQuote: "q",
	})
	stories, err = st.GetByMonthDay(context.Background(), 10, 1)
	require.NoError(t, err)
	require.Len(t, stories, 2)
	require.Equal(t, "Q4-04", stories[0].ID, "sorted by year")

	month, day := 10, 9
	got, total, err := st.List(context.Background(), store.ListFilter{Month: &month, Day: &day, Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, "Q4-01", got[0].ID)
}

func TestListFiltersAndPagination(t *testing.T) {
	st := fixtures(t)
	ctx := context.Background()

	month := 10
	stories, total, err := st.List(ctx, store.ListFilter{Month: &month, Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Len(t, stories, 2)

	stories, total, err = st.List(ctx, store.ListFilter{Tag: "仪器极客", Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Len(t, stories, 2)

	stories, total, err = st.List(ctx, store.ListFilter{Query: "RNase", Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, "Q4-02", stories[0].ID)

	stories, total, err = st.List(ctx, store.ListFilter{Query: "不存在的关键词", Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.EqualValues(t, 0, total)
	require.Empty(t, stories)

	page1, total, err := st.List(ctx, store.ListFilter{Page: 1, PageSize: 2})
	require.NoError(t, err)
	require.EqualValues(t, 3, total)
	require.Len(t, page1, 2)
	page2, _, err := st.List(ctx, store.ListFilter{Page: 2, PageSize: 2})
	require.NoError(t, err)
	require.Len(t, page2, 1)
	require.Equal(t, "Q4-02", page2[0].ID)
}

func TestListOrdersByMonthDayYear(t *testing.T) {
	st := fixtures(t)
	stories, _, err := st.List(context.Background(), store.ListFilter{Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.Equal(t, []string{"Q1-01", "Q4-01", "Q4-02"}, []string{stories[0].ID, stories[1].ID, stories[2].ID})
}

func TestRandomWithExclude(t *testing.T) {
	st := fixtures(t)
	ctx := context.Background()

	story, err := st.Random(ctx, []string{"Q1-01", "Q4-02"})
	require.NoError(t, err)
	require.Equal(t, "Q4-01", story.ID)

	story, err = st.Random(ctx, []string{"Q1-01", "Q4-01", "Q4-02"})
	require.NoError(t, err)
	require.NotNil(t, story, "falls back to ignoring a full exclude list")

	empty := newStore(t)
	_, err = empty.Random(ctx, nil)
	require.ErrorIs(t, err, store.ErrNotFound)
}

func TestTagCounts(t *testing.T) {
	st := fixtures(t)
	counts, err := st.TagCounts(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, counts)
	require.Equal(t, "仪器极客", counts[0].Name)
	require.EqualValues(t, 2, counts[0].Count)
}

func TestStats(t *testing.T) {
	st := fixtures(t)
	total, months, err := st.Stats(context.Background())
	require.NoError(t, err)
	require.EqualValues(t, 3, total)
	require.Equal(t, []int{1, 10}, months)
}

func TestUpsertIsIdempotent(t *testing.T) {
	st := newStore(t)
	ctx := context.Background()
	story := model.Story{
		ID: "Q1-01", Month: 1, Day: 5, Year: 1961, Title: "v1", Content: "c", GeekQuote: "q",
	}
	require.NoError(t, st.Upsert(ctx, &story, []string{"A", "B"}))

	first, err := st.GetByID(ctx, "Q1-01")
	require.NoError(t, err)

	story2 := model.Story{
		ID: "Q1-01", Month: 1, Day: 5, Year: 1961, Title: "v2", Content: "c2", GeekQuote: "q2",
	}
	require.NoError(t, st.Upsert(ctx, &story2, []string{"B", "C"}))
	require.NoError(t, st.Upsert(ctx, &story2, []string{"B", "C"}))

	total, _, err := st.Stats(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)

	second, err := st.GetByID(ctx, "Q1-01")
	require.NoError(t, err)
	require.Equal(t, "v2", second.Title)
	require.Equal(t, []string{"B", "C"}, []string{second.Tags[0].Name, second.Tags[1].Name})
	require.WithinDuration(t, first.CreatedAt, second.CreatedAt, 0)

	counts, err := st.TagCounts(ctx)
	require.NoError(t, err)
	for _, count := range counts {
		require.NotEqual(t, "A", count.Name, "orphan tag should be cleaned up")
	}
}
