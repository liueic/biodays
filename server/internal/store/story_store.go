package store

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"math/big"
	"time"

	"biodays/server/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *Store) GetByID(ctx context.Context, id string) (*model.Story, error) {
	var story model.Story
	err := preloadTags(s.db.WithContext(ctx)).Where("id = ?", id).First(&story).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &story, nil
}

func (s *Store) GetByMonthDay(ctx context.Context, month, day int) ([]model.Story, error) {
	var stories []model.Story
	err := preloadTags(s.db.WithContext(ctx)).
		Where("month = ? AND day = ?", month, day).
		Order("year ASC, id ASC").
		Find(&stories).Error
	return stories, err
}

func (s *Store) List(ctx context.Context, filter ListFilter) ([]model.Story, int64, error) {
	var total int64
	if err := filter.apply(s.db.WithContext(ctx).Model(&model.Story{})).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var stories []model.Story
	query := filter.apply(s.db.WithContext(ctx).Model(&model.Story{}))
	if filter.Summary {
		query = query.Select("stories.id", "stories.month", "stories.day", "stories.year", "stories.title", "stories.lab_item", "stories.summary")
	}
	err := preloadTags(query).
		Order("stories.month ASC, stories.day ASC, stories.year ASC, stories.id ASC").
		Offset((filter.Page - 1) * filter.PageSize).
		Limit(filter.PageSize).
		Find(&stories).Error
	if err != nil {
		return nil, 0, err
	}
	return stories, total, nil
}

func (s *Store) Random(ctx context.Context, exclude []string) (*model.Story, error) {
	story, _, err := s.random(ctx, exclude, nil, rand.Reader)
	return story, err
}

// RandomUnread draws uniformly from unseen IDs, starting a fresh round only when
// none remain. Explicit exclusions (the current story) still apply in a new round.
func (s *Store) RandomUnread(ctx context.Context, exclude, seen []string) (*model.Story, bool, error) {
	return s.random(ctx, exclude, seen, rand.Reader)
}

// RandomUnreadByMonthDay uses dates as metadata, never an ID prefix. Exhausting
// today's candidates does not reset the reader's history for the whole corpus.
func (s *Store) RandomUnreadByMonthDay(ctx context.Context, month, day int, seen []string) (*model.Story, error) {
	query := s.db.Where("month = ? AND day = ?", month, day)
	story, _, err := s.randomFrom(ctx, query, nil, seen, rand.Reader)
	return story, err
}

func (s *Store) random(ctx context.Context, exclude, seen []string, entropy io.Reader) (*model.Story, bool, error) {
	return s.randomFrom(ctx, s.db, exclude, seen, entropy)
}

func (s *Store) randomFrom(ctx context.Context, query *gorm.DB, exclude, seen []string, entropy io.Reader) (*model.Story, bool, error) {
	var ids []string
	err := query.WithContext(ctx).Model(&model.Story{}).Order("id ASC").Pluck("id", &ids).Error
	if err != nil {
		return nil, false, err
	}
	if len(ids) == 0 {
		return nil, false, ErrNotFound
	}
	blocked := make(map[string]bool, len(exclude)+len(seen))
	for _, id := range exclude {
		blocked[id] = true
	}
	for _, id := range seen {
		blocked[id] = true
	}
	candidates := unblockedIDs(ids, blocked)
	reset := len(candidates) == 0 && len(seen) > 0
	if len(candidates) == 0 {
		clear(blocked)
		for _, id := range exclude {
			blocked[id] = true
		}
		candidates = unblockedIDs(ids, blocked)
		// Preserve the existing API's fallback for a one-story / fully excluded dataset.
		if len(candidates) == 0 {
			candidates = ids
		}
	}
	// rand.Int uses rejection sampling, avoiding modulo bias for arbitrary counts.
	index, err := rand.Int(entropy, big.NewInt(int64(len(candidates))))
	if err != nil {
		return nil, false, fmt.Errorf("random story entropy: %w", err)
	}
	story, err := s.GetByID(ctx, candidates[index.Int64()])
	return story, reset, err
}

func unblockedIDs(ids []string, blocked map[string]bool) []string {
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		if !blocked[id] {
			result = append(result, id)
		}
	}
	return result
}

func (s *Store) TagCounts(ctx context.Context) ([]model.TagCount, error) {
	var counts []model.TagCount
	err := s.db.WithContext(ctx).
		Table("tags").
		Select("tags.name AS name, COUNT(story_tags.story_id) AS count").
		Joins("JOIN story_tags ON story_tags.tag_id = tags.id").
		Group("tags.id, tags.name").
		Order("count DESC, tags.name ASC").
		Scan(&counts).Error
	return counts, err
}

func (s *Store) Stats(ctx context.Context) (int64, []int, error) {
	var total int64
	if err := s.db.WithContext(ctx).Model(&model.Story{}).Count(&total).Error; err != nil {
		return 0, nil, err
	}
	var months []int
	if err := s.db.WithContext(ctx).Model(&model.Story{}).
		Distinct().
		Order("month ASC").
		Pluck("month", &months).Error; err != nil {
		return 0, nil, err
	}
	return total, months, nil
}

func (s *Store) MonthCounts(ctx context.Context) (map[int]int64, error) {
	var rows []struct {
		Month int
		Count int64
	}
	err := s.db.WithContext(ctx).Model(&model.Story{}).
		Select("month, COUNT(*) AS count").Group("month").Scan(&rows).Error
	counts := make(map[int]int64, len(rows))
	for _, row := range rows {
		counts[row.Month] = row.Count
	}
	return counts, err
}

func (s *Store) SetDatasetVersion(ctx context.Context, version string) error {
	return s.db.WithContext(ctx).Save(&model.Dataset{ID: 1, Version: version}).Error
}

func (s *Store) DatasetVersion(ctx context.Context) (string, error) {
	var dataset model.Dataset
	err := s.db.WithContext(ctx).First(&dataset, 1).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	return dataset.Version, err
}

func (s *Store) Upsert(ctx context.Context, story *model.Story, tagNames []string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		story.Tags = nil
		if story.CreatedAt.IsZero() {
			story.CreatedAt = now
		}
		story.UpdatedAt = now

		err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"month", "day", "year", "title", "lab_item", "summary", "content",
				"geek_quote", "source_type", "source_citation", "source_url", "updated_at",
			}),
		}).Omit("Tags").Create(story).Error
		if err != nil {
			return err
		}

		tags := make([]model.Tag, 0, len(tagNames))
		for _, name := range tagNames {
			if name == "" {
				continue
			}
			tag := model.Tag{Name: name}
			if err := tx.Where("name = ?", name).FirstOrCreate(&tag).Error; err != nil {
				return err
			}
			tags = append(tags, tag)
		}

		ref := model.Story{ID: story.ID}
		return tx.Model(&ref).Association("Tags").Replace(tags)
	})
}

func (s *Store) CleanupOrphanTags(ctx context.Context) error {
	return s.db.WithContext(ctx).Exec(
		"DELETE FROM tags WHERE id NOT IN (SELECT DISTINCT tag_id FROM story_tags)",
	).Error
}

// DeleteStory removes a story and its tag associations. It reports whether the
// id existed; orphaned tags are left for CleanupOrphanTags.
func (s *Store) DeleteStory(ctx context.Context, id string) (bool, error) {
	deleted := false
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		story := model.Story{ID: id}
		if err := tx.Model(&story).Association("Tags").Clear(); err != nil {
			return err
		}
		result := tx.Delete(&model.Story{}, "id = ?", id)
		if result.Error != nil {
			return result.Error
		}
		deleted = result.RowsAffected > 0
		return nil
	})
	return deleted, err
}
