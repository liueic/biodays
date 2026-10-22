package seed

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"biodays/server/internal/model"
	"biodays/server/internal/store"
)

type Report struct {
	Files    int
	Imported int
	Skipped  int
}

type inputSource struct {
	Type     string `json:"type"`
	Citation string `json:"citation"`
	URL      string `json:"url"`
}

type inputStory struct {
	ID        string      `json:"id"`
	Month     int         `json:"month"`
	Day       int         `json:"day"`
	Year      int         `json:"year"`
	Title     string      `json:"title"`
	LabItem   string      `json:"lab_item"`
	Tags      []string    `json:"tags"`
	Summary   string      `json:"summary"`
	Content   string      `json:"content"`
	GeekQuote string      `json:"geek_quote"`
	Source    inputSource `json:"source"`
}

func Run(ctx context.Context, st *store.Store, dataDir string, logger *slog.Logger) (Report, error) {
	pattern := filepath.Join(dataDir, "stories_*.json")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return Report{}, fmt.Errorf("glob %s: %w", pattern, err)
	}
	if len(files) == 0 {
		return Report{}, fmt.Errorf("no stories_*.json found in %s", dataDir)
	}
	sort.Strings(files)

	report := Report{Files: len(files)}
	seenIDs := make(map[string]string, 208)

	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			logger.Warn("seed: cannot read file", "file", file, "error", err)
			continue
		}
		var entries []inputStory
		if err := json.Unmarshal(raw, &entries); err != nil {
			logger.Warn("seed: cannot parse file", "file", file, "error", err)
			continue
		}
		for index, entry := range entries {
			label := entry.ID
			if label == "" {
				label = fmt.Sprintf("%s[%d]", filepath.Base(file), index)
			}
			if problems := validate(entry); len(problems) > 0 {
				report.Skipped++
				logger.Warn("seed: skipped invalid story", "story", label, "problems", problems)
				continue
			}
			entry.ID = strings.TrimSpace(entry.ID)
			if previous, duplicate := seenIDs[entry.ID]; duplicate {
				report.Skipped++
				logger.Warn("seed: skipped duplicate id", "story", entry.ID, "first_seen_in", previous)
				continue
			}

			story, tags := toModel(entry)
			if err := st.Upsert(ctx, story, tags); err != nil {
				return report, fmt.Errorf("upsert %s: %w", entry.ID, err)
			}
			seenIDs[entry.ID] = filepath.Base(file)
			report.Imported++
		}
	}

	if err := st.CleanupOrphanTags(ctx); err != nil {
		return report, fmt.Errorf("cleanup orphan tags: %w", err)
	}
	return report, nil
}

func validate(entry inputStory) []string {
	var problems []string
	if strings.TrimSpace(entry.ID) == "" {
		problems = append(problems, "id is empty")
	} else if len(entry.ID) > 16 {
		problems = append(problems, "id longer than 16 characters")
	}
	if entry.Month < 1 || entry.Month > 12 {
		problems = append(problems, fmt.Sprintf("month %d out of range 1-12", entry.Month))
	}
	if entry.Day < 1 || entry.Day > 31 {
		problems = append(problems, fmt.Sprintf("day %d out of range 1-31", entry.Day))
	}
	if entry.Year <= 0 {
		problems = append(problems, "year must be positive")
	}
	if strings.TrimSpace(entry.Title) == "" {
		problems = append(problems, "title is empty")
	}
	if strings.TrimSpace(entry.Content) == "" {
		problems = append(problems, "content is empty")
	}
	if strings.TrimSpace(entry.GeekQuote) == "" {
		problems = append(problems, "geek_quote is empty")
	}
	return problems
}

func toModel(entry inputStory) (*model.Story, []string) {
	tags := make([]string, 0, len(entry.Tags))
	seen := make(map[string]bool, len(entry.Tags))
	for _, tag := range entry.Tags {
		name := strings.TrimSpace(tag)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		tags = append(tags, name)
	}
	story := &model.Story{
		ID:             strings.TrimSpace(entry.ID),
		Month:          entry.Month,
		Day:            entry.Day,
		Year:           entry.Year,
		Title:          strings.TrimSpace(entry.Title),
		LabItem:        strings.TrimSpace(entry.LabItem),
		Summary:        strings.TrimSpace(entry.Summary),
		Content:        strings.TrimSpace(entry.Content),
		GeekQuote:      strings.TrimSpace(entry.GeekQuote),
		SourceType:     strings.TrimSpace(entry.Source.Type),
		SourceCitation: strings.TrimSpace(entry.Source.Citation),
		SourceURL:      strings.TrimSpace(entry.Source.URL),
	}
	return story, tags
}
