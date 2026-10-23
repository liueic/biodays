package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"biodays/server/internal/model"
	"biodays/server/internal/store"
)

var ErrInvalidOffset = errors.New("invalid timezone offset")

var ErrNotFound = store.ErrNotFound

const (
	FallbackRandom = "random"
	FallbackNone   = "none"
)

var offsetPattern = regexp.MustCompile(`^([+-])(\d{2}):(\d{2})$`)

type StoryService struct {
	store *store.Store
	now   func() time.Time
}

func NewStoryService(st *store.Store) *StoryService {
	return &StoryService{store: st, now: time.Now}
}

func NewStoryServiceWithClock(st *store.Store, now func() time.Time) *StoryService {
	return &StoryService{store: st, now: now}
}

func ParseOffset(value string) (*time.Location, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || trimmed == "Z" || strings.EqualFold(trimmed, "UTC") {
		return time.UTC, nil
	}
	matches := offsetPattern.FindStringSubmatch(trimmed)
	if matches == nil {
		return nil, fmt.Errorf("%w: %q (expected +08:00 style)", ErrInvalidOffset, value)
	}
	hours, _ := strconv.Atoi(matches[2])
	minutes, _ := strconv.Atoi(matches[3])
	if hours > 14 || minutes > 59 || (hours == 14 && minutes != 0) {
		return nil, fmt.Errorf("%w: %q", ErrInvalidOffset, value)
	}
	offset := hours*3600 + minutes*60
	if matches[1] == "-" {
		offset = -offset
	}
	return time.FixedZone("UTC"+trimmed, offset), nil
}

func (s *StoryService) Today(ctx context.Context, tz, fallback string) (model.StoryDTO, bool, error) {
	return s.TodayUnread(ctx, tz, fallback, nil)
}

// TodayUnread prefers unread stories on this calendar date, regardless of their
// ID format. History resets belong to the global random-reading round only.
func (s *StoryService) TodayUnread(ctx context.Context, tz, fallback string, seen []string) (model.StoryDTO, bool, error) {
	if tz == "" {
		tz = "+08:00"
	}
	location, err := ParseOffset(tz)
	if err != nil {
		return model.StoryDTO{}, false, err
	}
	if fallback == "" {
		fallback = FallbackRandom
	}

	now := s.now().In(location)
	story, err := s.store.RandomUnreadByMonthDay(ctx, int(now.Month()), int(now.Day()), seen)
	if err == nil {
		return story.ToDTO(), false, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return model.StoryDTO{}, false, err
	}
	if fallback == FallbackNone {
		return model.StoryDTO{}, false, store.ErrNotFound
	}

	story, _, err = s.store.RandomUnread(ctx, nil, seen)
	if err != nil {
		return model.StoryDTO{}, false, err
	}
	return story.ToDTO(), true, nil
}

func (s *StoryService) Get(ctx context.Context, id string) (model.StoryDTO, error) {
	story, err := s.store.GetByID(ctx, id)
	if err != nil {
		return model.StoryDTO{}, err
	}
	return story.ToDTO(), nil
}

type ListParams struct {
	Month    *int
	Day      *int
	Tag      string
	Query    string
	Page     int
	PageSize int
	Summary  bool
}

func (s *StoryService) List(ctx context.Context, params ListParams) ([]model.StoryDTO, int64, error) {
	stories, total, err := s.store.List(ctx, store.ListFilter{
		Month:    params.Month,
		Day:      params.Day,
		Tag:      params.Tag,
		Query:    params.Query,
		Page:     params.Page,
		PageSize: params.PageSize,
		Summary:  params.Summary,
	})
	if err != nil {
		return nil, 0, err
	}
	return model.StoriesToDTO(stories), total, nil
}

func (s *StoryService) Random(ctx context.Context, exclude []string) (model.StoryDTO, error) {
	story, err := s.store.Random(ctx, exclude)
	if err != nil {
		return model.StoryDTO{}, err
	}
	return story.ToDTO(), nil
}

func (s *StoryService) RandomUnread(ctx context.Context, exclude, seen []string) (model.StoryDTO, bool, error) {
	story, reset, err := s.store.RandomUnread(ctx, exclude, seen)
	if err != nil {
		return model.StoryDTO{}, false, err
	}
	return story.ToDTO(), reset, nil
}

func (s *StoryService) Tags(ctx context.Context) ([]model.TagCount, error) {
	return s.store.TagCounts(ctx)
}

func (s *StoryService) Meta(ctx context.Context) (model.Meta, error) {
	counts, err := s.store.MonthCounts(ctx)
	if err != nil {
		return model.Meta{}, err
	}
	var total int64
	months := make([]int, 0, len(counts))
	for month, count := range counts {
		months = append(months, month)
		total += count
	}
	sort.Ints(months)
	version, err := s.store.DatasetVersion(ctx)
	if err != nil {
		return model.Meta{}, err
	}
	meta := BuildMeta(total, months)
	meta.MonthCounts = counts
	meta.DatasetVersion = version
	return meta, nil
}

func BuildMeta(total int64, months []int) model.Meta {
	quarters := make([]string, 0, 4)
	seen := make(map[int]bool, 4)
	for _, month := range months {
		quarter := (month-1)/3 + 1
		if !seen[quarter] {
			seen[quarter] = true
			quarters = append(quarters, fmt.Sprintf("Q%d", quarter))
		}
	}
	return model.Meta{
		Total:    total,
		Quarters: quarters,
		Coverage: FormatMonthRanges(months),
	}
}

func FormatMonthRanges(months []int) string {
	if len(months) == 0 {
		return ""
	}
	sorted := make([]int, 0, len(months))
	seen := make(map[int]bool, len(months))
	for _, month := range months {
		if !seen[month] {
			seen[month] = true
			sorted = append(sorted, month)
		}
	}
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j] < sorted[j-1]; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}

	ranges := make([]string, 0, len(sorted))
	start, end := sorted[0], sorted[0]
	flush := func() {
		if start == end {
			ranges = append(ranges, fmt.Sprintf("%d月", start))
			return
		}
		ranges = append(ranges, fmt.Sprintf("%d-%d月", start, end))
	}
	for _, month := range sorted[1:] {
		if month == end+1 {
			end = month
			continue
		}
		flush()
		start, end = month, month
	}
	flush()
	return strings.Join(ranges, " / ")
}
