package antispam

import (
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"biodays/server/internal/model"
)

type ScoreInput struct {
	Title          string
	Content        string
	SourceURL      string
	Email          string
	ElapsedSeconds int
	DuplicateCount int64
	DomainCount    int64
}

type ScoreResult struct {
	Score   int
	Verdict string
	Signals []string
}

func Score(in ScoreInput, review, block int) ScoreResult {
	if review <= 0 {
		review = 30
	}
	if block <= review {
		block = 70
	}
	score := 0
	signals := make([]string, 0, 10)
	added := make(map[string]struct{}, 10)
	add := func(weight int, signal string) {
		if _, exists := added[signal]; exists {
			return
		}
		added[signal] = struct{}{}
		signals = append(signals, signal)
		score += weight
	}

	combined := in.Title + "\n" + in.Content
	links := countURLs(combined)
	if in.SourceURL != "" {
		links++
	}
	switch {
	case links > 5:
		add(35, "many_links")
	case links > 2:
		add(20, "several_links")
	}

	shortener := false
	for _, raw := range urlPattern.FindAllString(combined, -1) {
		parsed, err := url.Parse(raw)
		if err == nil && IsShortenerHost(parsed.Hostname()) {
			shortener = true
			break
		}
	}
	if shortener {
		add(30, "shortener_link")
	}

	if containsZeroWidth(combined) {
		add(20, "zero_width")
	}
	if containsControl(combined) {
		add(30, "control_chars")
	}
	if in.DuplicateCount >= 3 {
		add(60, "duplicate_content")
	} else if in.DuplicateCount >= 1 {
		add(40, "duplicate_content")
	}
	if in.DomainCount >= 5 {
		add(15, "email_domain_burst")
	}
	domain := EmailDomain(in.Email)
	if domain != "" && IsDisposableDomain(domain) {
		add(25, "disposable_email")
	}
	if ContainsSpamKeyword(combined) {
		add(15, "spam_keyword")
	}
	if urlOnly(combined) {
		add(20, "url_only")
	}
	if excessiveCaps(combined) {
		add(10, "excessive_caps")
	}
	if repeatedRune(combined) {
		add(10, "repeated_chars")
	}

	if score > 100 {
		score = 100
	}
	verdict := model.SpamVerdictAllow
	switch {
	case score >= block:
		verdict = model.SpamVerdictSpam
	case score >= review:
		verdict = model.SpamVerdictReview
	}
	if len(signals) == 0 {
		signals = []string{}
	}
	return ScoreResult{Score: score, Verdict: verdict, Signals: signals}
}

func containsControl(text string) bool {
	for _, r := range text {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return true
		}
	}
	return false
}

func urlOnly(text string) bool {
	withoutURLs := strings.TrimSpace(urlPattern.ReplaceAllString(text, ""))
	return withoutURLs != "" && utf8.RuneCountInString(withoutURLs) < 10 && urlPattern.MatchString(text)
}

func excessiveCaps(text string) bool {
	letters := 0
	upper := 0
	for _, r := range text {
		if unicode.IsLetter(r) {
			letters++
			if unicode.IsUpper(r) {
				upper++
			}
		}
	}
	return letters > 20 && float64(upper)/float64(letters) > 0.8
}

func repeatedRune(text string) bool {
	run := 0
	var previous rune
	for _, r := range text {
		if r == previous {
			run++
		} else {
			previous = r
			run = 1
		}
		if run > 20 {
			return true
		}
	}
	return false
}
