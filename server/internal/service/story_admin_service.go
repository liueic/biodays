package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"biodays/server/internal/model"
	"biodays/server/internal/seed"
	"biodays/server/internal/store"
)

// MaxStoryBatch caps how many stories one admin request may upsert.
const MaxStoryBatch = 200

var ErrStoryBatchTooLarge = fmt.Errorf("story batch exceeds %d items", MaxStoryBatch)

// StoryAdminService is the write path behind /admin/stories. Validation is the
// seed package's rule set, so admin-published stories match imported ones.
type StoryAdminService struct {
	stories *store.Store
	now     func() time.Time
}

func NewStoryAdminService(st *store.Store, now func() time.Time) *StoryAdminService {
	if now == nil {
		now = time.Now
	}
	return &StoryAdminService{stories: st, now: now}
}

type StoryUpsertItem struct {
	ID       string   `json:"id" example:"Q1-01"`
	Created  bool     `json:"created"`
	Problems []string `json:"problems,omitempty"`
}

type StoryBatchResult struct {
	Items    []StoryUpsertItem `json:"items"`
	Accepted int               `json:"accepted"`
	Rejected int               `json:"rejected"`
}

// Upsert publishes one story, creating it when the id is new.
func (s *StoryAdminService) Upsert(ctx context.Context, entry seed.StoryInput) (StoryUpsertItem, error) {
	item, _, err := s.upsertOne(ctx, entry)
	return item, err
}

// UpsertBatch publishes up to MaxStoryBatch stories and reports each item;
// invalid entries are rejected individually without failing the batch.
func (s *StoryAdminService) UpsertBatch(ctx context.Context, entries []seed.StoryInput) (StoryBatchResult, error) {
	if len(entries) > MaxStoryBatch {
		return StoryBatchResult{}, ErrStoryBatchTooLarge
	}
	result := StoryBatchResult{Items: make([]StoryUpsertItem, 0, len(entries))}
	for _, entry := range entries {
		item, accepted, err := s.upsertOne(ctx, entry)
		if err != nil {
			return StoryBatchResult{}, err
		}
		if accepted {
			result.Accepted++
		} else {
			result.Rejected++
		}
		result.Items = append(result.Items, item)
	}
	return result, nil
}

func (s *StoryAdminService) upsertOne(ctx context.Context, entry seed.StoryInput) (StoryUpsertItem, bool, error) {
	if problems := seed.ValidateStory(entry); len(problems) > 0 {
		return StoryUpsertItem{ID: entry.ID, Problems: problems}, false, nil
	}
	id := strings.TrimSpace(entry.ID)
	created := false
	fromDataset := false
	if existing, err := s.stories.GetByID(ctx, id); err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			return StoryUpsertItem{}, false, err
		}
		created = true
	} else {
		fromDataset = existing.FromDataset
	}
	story, tags := seed.StoryToModel(entry)
	if err := s.stories.Upsert(ctx, story, tags, fromDataset, true); err != nil {
		return StoryUpsertItem{}, false, err
	}
	// An explicit re-publish clears a deletion tombstone: the administrator
	// wants this story back, even if a later dataset import removed it.
	if err := s.stories.RemoveTombstone(ctx, id); err != nil {
		return StoryUpsertItem{}, false, err
	}
	if err := s.stories.CleanupOrphanTags(ctx); err != nil {
		return StoryUpsertItem{}, false, err
	}
	return StoryUpsertItem{ID: story.ID, Created: created}, true, nil
}

// StoryPatchSource is the optional citation block of a patch.
type StoryPatchSource struct {
	Type     *string `json:"type"`
	Citation *string `json:"citation"`
	URL      *string `json:"url"`
}

// StoryPatch mirrors StoryInput with optional fields; a present field replaces
// the stored value, an absent one keeps it.
type StoryPatch struct {
	Month     *int              `json:"month"`
	Day       *int              `json:"day"`
	Year      *int              `json:"year"`
	Title     *string           `json:"title"`
	LabItem   *string           `json:"lab_item"`
	Tags      *[]string         `json:"tags"`
	Summary   *string           `json:"summary"`
	Content   *string           `json:"content"`
	GeekQuote *string           `json:"geek_quote"`
	Source    *StoryPatchSource `json:"source"`
}

// Update applies a partial patch and returns the merged story. When validation
// fails the problems describe the merged result, not the patch alone.
func (s *StoryAdminService) Update(ctx context.Context, id string, patch StoryPatch) (model.StoryDTO, []string, error) {
	existing, err := s.stories.GetByID(ctx, id)
	if err != nil {
		return model.StoryDTO{}, nil, err
	}
	merged := seed.StoryInput{
		ID:    existing.ID,
		Month: existing.Month,
		Day:   existing.Day,
		Year:  existing.Year,
		Title: existing.Title, LabItem: existing.LabItem,
		Summary: existing.Summary, Content: existing.Content, GeekQuote: existing.GeekQuote,
		Source: seed.SourceInput{
			Type:     existing.SourceType,
			Citation: existing.SourceCitation,
			URL:      existing.SourceURL,
		},
	}
	for _, tag := range existing.Tags {
		merged.Tags = append(merged.Tags, tag.Name)
	}
	if patch.Month != nil {
		merged.Month = *patch.Month
	}
	if patch.Day != nil {
		merged.Day = *patch.Day
	}
	if patch.Year != nil {
		merged.Year = *patch.Year
	}
	if patch.Title != nil {
		merged.Title = *patch.Title
	}
	if patch.LabItem != nil {
		merged.LabItem = *patch.LabItem
	}
	if patch.Tags != nil {
		merged.Tags = *patch.Tags
	}
	if patch.Summary != nil {
		merged.Summary = *patch.Summary
	}
	if patch.Content != nil {
		merged.Content = *patch.Content
	}
	if patch.GeekQuote != nil {
		merged.GeekQuote = *patch.GeekQuote
	}
	if patch.Source != nil {
		if patch.Source.Type != nil {
			merged.Source.Type = *patch.Source.Type
		}
		if patch.Source.Citation != nil {
			merged.Source.Citation = *patch.Source.Citation
		}
		if patch.Source.URL != nil {
			merged.Source.URL = *patch.Source.URL
		}
	}
	if problems := seed.ValidateStory(merged); len(problems) > 0 {
		return model.StoryDTO{}, problems, nil
	}
	story, tags := seed.StoryToModel(merged)
	if err := s.stories.Upsert(ctx, story, tags, existing.FromDataset, true); err != nil {
		return model.StoryDTO{}, nil, err
	}
	if err := s.stories.CleanupOrphanTags(ctx); err != nil {
		return model.StoryDTO{}, nil, err
	}
	updated, err := s.stories.GetByID(ctx, id)
	if err != nil {
		return model.StoryDTO{}, nil, err
	}
	return updated.ToDTO(), nil, nil
}

// Delete removes a story and cleans up tags left orphaned by the removal.
// Bundled (dataset) stories leave a tombstone so incremental corpus imports
// do not resurrect them.
func (s *StoryAdminService) Delete(ctx context.Context, id string) error {
	existing, err := s.stories.GetByID(ctx, id)
	if err != nil {
		return err
	}
	deleted, err := s.stories.DeleteStory(ctx, id)
	if err != nil {
		return err
	}
	if !deleted {
		return store.ErrNotFound
	}
	if existing.FromDataset {
		if err := s.stories.AddTombstone(ctx, id, s.now()); err != nil {
			return err
		}
	}
	return s.stories.CleanupOrphanTags(ctx)
}
