package store

import (
	"context"
	"errors"
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
	query := preloadTags(s.db.WithContext(ctx))
	if len(exclude) > 0 {
		query = query.Where("id NOT IN ?", exclude)
	}

	var story model.Story
	err := query.Order("RANDOM()").First(&story).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if len(exclude) > 0 {
			return s.Random(ctx, nil)
		}
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &story, nil
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
