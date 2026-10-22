package model

import "time"

type Story struct {
	ID             string    `gorm:"primaryKey;size:16"`
	Month          int       `gorm:"not null;index:idx_stories_month_day,priority:1"`
	Day            int       `gorm:"not null;index:idx_stories_month_day,priority:2"`
	Year           int       `gorm:"not null"`
	Title          string    `gorm:"not null"`
	LabItem        string    `gorm:"column:lab_item;not null;default:''"`
	Summary        string    `gorm:"not null;default:''"`
	Content        string    `gorm:"not null"`
	GeekQuote      string    `gorm:"column:geek_quote;not null;default:''"`
	SourceType     string    `gorm:"column:source_type;not null;default:''"`
	SourceCitation string    `gorm:"column:source_citation;not null;default:''"`
	SourceURL      string    `gorm:"column:source_url;not null;default:''"`
	Tags           []Tag     `gorm:"many2many:story_tags;constraint:OnDelete:CASCADE"`
	CreatedAt      time.Time `gorm:"not null"`
	UpdatedAt      time.Time `gorm:"not null"`
}

type Tag struct {
	ID   uint   `gorm:"primaryKey"`
	Name string `gorm:"not null;uniqueIndex"`
}

type Source struct {
	Type     string `json:"type"`
	Citation string `json:"citation"`
	URL      string `json:"url"`
}

type StoryDTO struct {
	ID        string   `json:"id" example:"Q1-01"`
	Month     int      `json:"month" example:"1"`
	Day       int      `json:"day" example:"5"`
	Year      int      `json:"year" example:"1961"`
	Title     string   `json:"title"`
	LabItem   string   `json:"lab_item"`
	Tags      []string `json:"tags"`
	Summary   string   `json:"summary"`
	Content   string   `json:"content"`
	GeekQuote string   `json:"geek_quote"`
	Source    Source   `json:"source"`
}

type TagCount struct {
	Name  string `json:"name" example:"仪器极客"`
	Count int64  `json:"count" example:"8"`
}

type Meta struct {
	Total    int64    `json:"total" example:"104"`
	Quarters []string `json:"quarters" example:"Q1,Q4"`
	Coverage string   `json:"coverage" example:"1-3月 / 10-12月"`
}

func (s Story) ToDTO() StoryDTO {
	tags := make([]string, 0, len(s.Tags))
	for _, tag := range s.Tags {
		tags = append(tags, tag.Name)
	}
	return StoryDTO{
		ID:        s.ID,
		Month:     s.Month,
		Day:       s.Day,
		Year:      s.Year,
		Title:     s.Title,
		LabItem:   s.LabItem,
		Tags:      tags,
		Summary:   s.Summary,
		Content:   s.Content,
		GeekQuote: s.GeekQuote,
		Source: Source{
			Type:     s.SourceType,
			Citation: s.SourceCitation,
			URL:      s.SourceURL,
		},
	}
}

func StoriesToDTO(stories []Story) []StoryDTO {
	out := make([]StoryDTO, 0, len(stories))
	for _, story := range stories {
		out = append(out, story.ToDTO())
	}
	return out
}
