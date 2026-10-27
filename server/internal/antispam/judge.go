package antispam

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"biodays/server/internal/model"
	"biodays/server/internal/security"
)

// JudgeStore is the slice of FeedbackStore the Judge needs, as an interface
// so the settings manager can rebuild Judges without import cycles.
type JudgeStore interface {
	CountFeedbackByContentHash(ctx context.Context, hash string, since time.Time) (int64, error)
	CountFeedbackByEmailDomain(ctx context.Context, domain string, since time.Time) (int64, error)
	ConsumeRateLimit(ctx context.Context, keyHash string, windowStart, expiresAt time.Time, limit int) (bool, error)
}

type Settings struct {
	Enabled      bool
	ReviewScore  int
	BlockScore   int
	AIThreshold  float64
	AIDailyLimit int
	AIMaxChars   int
}

type Input struct {
	Title          string
	Content        string
	SourceURL      string
	Email          string
	ClientIP       string
	ElapsedSeconds int
	Now            time.Time
}

type Decision struct {
	Score      int
	Verdict    string
	Signals    []string
	Source     string
	Model      string
	Confidence float64
}

type Judge struct {
	store      JudgeStore
	secret     []byte
	settings   Settings
	classifier Classifier
	logger     *slog.Logger
	now        func() time.Time
}

func NewJudge(st JudgeStore, secret []byte, settings Settings, classifier Classifier, logger *slog.Logger, now func() time.Time) *Judge {
	if now == nil {
		now = time.Now
	}
	return &Judge{store: st, secret: secret, settings: settings, classifier: classifier, logger: logger, now: now}
}

func (j *Judge) AIEnabled() bool {
	return j != nil && j.classifier != nil
}

func (j *Judge) Evaluate(ctx context.Context, in Input) Decision {
	allow := Decision{Verdict: model.SpamVerdictAllow, Signals: []string{}}
	if j == nil || !j.settings.Enabled {
		return allow
	}
	now := in.Now.UTC()
	if now.IsZero() {
		now = j.now().UTC()
	}
	hash := ContentHash(in.Title, in.Content)
	duplicates := int64(0)
	if hash != "" {
		if count, err := j.store.CountFeedbackByContentHash(ctx, hash, now.AddDate(0, 0, -30)); err == nil {
			duplicates = count
		}
	}
	domain := EmailDomain(in.Email)
	domainCount := int64(0)
	if domain != "" {
		if count, err := j.store.CountFeedbackByEmailDomain(ctx, domain, now.Add(-time.Hour)); err == nil {
			domainCount = count
		}
	}
	scored := Score(ScoreInput{
		Title: in.Title, Content: in.Content, SourceURL: in.SourceURL, Email: in.Email,
		ElapsedSeconds: in.ElapsedSeconds, DuplicateCount: duplicates, DomainCount: domainCount,
	}, j.settings.ReviewScore, j.settings.BlockScore)
	decision := Decision{
		Score: scored.Score, Verdict: scored.Verdict, Signals: scored.Signals,
		Source: model.SpamSourceHeuristic,
	}
	if decision.Verdict != model.SpamVerdictReview || j.classifier == nil {
		j.log(decision)
		return decision
	}

	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	limit := j.settings.AIDailyLimit
	if limit <= 0 {
		limit = 500
	}
	allowed, err := j.store.ConsumeRateLimit(ctx, security.HMACKey(j.secret, "ai:spam:day"),
		day, day.Add(24*time.Hour), limit)
	if err != nil || !allowed {
		decision.Source = model.SpamSourceAIBudget
		j.log(decision)
		return decision
	}

	maxChars := j.settings.AIMaxChars
	if maxChars <= 0 {
		maxChars = 4000
	}
	text := Redact(in.Title, in.Content, maxChars)
	verdict, err := j.classifier.Classify(ctx, text)
	if err != nil {
		decision.Source = model.SpamSourceAIError
		j.log(decision)
		return decision
	}
	decision.Source = model.SpamSourceAI
	decision.Model = verdict.Model
	decision.Confidence = verdict.Confidence
	threshold := j.settings.AIThreshold
	if threshold <= 0 {
		threshold = 0.8
	}
	if verdict.Label == "spam" && verdict.Confidence >= threshold {
		decision.Verdict = model.SpamVerdictSpam
	} else if verdict.Label == "legit" && verdict.Confidence >= threshold {
		decision.Verdict = model.SpamVerdictAllow
	}
	j.log(decision)
	return decision
}

func (j *Judge) log(decision Decision) {
	if j.logger == nil || (decision.Verdict == model.SpamVerdictAllow && decision.Source == model.SpamSourceHeuristic) {
		return
	}
	j.logger.Info("feedback spam decision",
		"verdict", decision.Verdict,
		"score", decision.Score,
		"signals", strings.Join(decision.Signals, ","),
		"source", decision.Source,
		"model", decision.Model,
		"confidence", decision.Confidence,
	)
}
