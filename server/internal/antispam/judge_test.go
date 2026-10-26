package antispam

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"biodays/server/internal/model"
	"biodays/server/internal/store"
)

type countingClassifier struct {
	calls   int
	verdict Verdict
}

func (c *countingClassifier) Classify(context.Context, string) (Verdict, error) {
	c.calls++
	return c.verdict, nil
}

func newJudgeFixture(t *testing.T, classifier Classifier) (*Judge, *store.FeedbackStore) {
	t.Helper()
	st, err := store.OpenFeedback(filepath.Join(t.TempDir(), "feedback.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	now := time.Date(2026, 10, 25, 10, 0, 0, 0, time.UTC)
	judge := NewJudge(st, []byte("0123456789abcdef0123456789abcdef"), Settings{
		Enabled: true, ReviewScore: 30, BlockScore: 70,
		AIThreshold: 0.8, AIDailyLimit: 100, AIMaxChars: 4000,
	}, classifier, nil, func() time.Time { return now })
	return judge, st
}

func TestJudgeDoesNotCallAIDirectly(t *testing.T) {
	classifier := &countingClassifier{verdict: Verdict{Label: "spam", Confidence: 0.99}}
	judge, _ := newJudgeFixture(t, classifier)
	decision := judge.Evaluate(context.Background(), Input{
		Title: "想了解 GFP", Content: "希望讲讲荧光蛋白的发现过程。",
		ElapsedSeconds: -1, Now: time.Date(2026, 10, 25, 10, 0, 0, 0, time.UTC),
	})
	if decision.Verdict != model.SpamVerdictAllow || classifier.calls != 0 {
		t.Fatalf("unexpected decision %+v calls=%d", decision, classifier.calls)
	}
}

func TestJudgeCallsAIForReview(t *testing.T) {
	classifier := &countingClassifier{verdict: Verdict{Label: "spam", Confidence: 0.95, Model: "test"}}
	judge, _ := newJudgeFixture(t, classifier)
	decision := judge.Evaluate(context.Background(), Input{
		Title:   "资源推荐",
		Content: "资源：https://a.test https://b.test https://bit.ly/x 这些是相关链接，供参考。", ElapsedSeconds: -1,
		Now: time.Date(2026, 10, 25, 10, 0, 0, 0, time.UTC),
	})
	if classifier.calls != 1 {
		t.Fatalf("expected one AI call, got %d", classifier.calls)
	}
	if decision.Verdict != model.SpamVerdictSpam || decision.Source != model.SpamSourceAI {
		t.Fatalf("unexpected decision %+v", decision)
	}
}

func TestJudgeSkipsAIWhenBudgetExhausted(t *testing.T) {
	classifier := &countingClassifier{verdict: Verdict{Label: "spam", Confidence: 0.95}}
	judge, _ := newJudgeFixture(t, classifier)
	judge.settings.AIDailyLimit = 1
	now := time.Date(2026, 10, 25, 10, 0, 0, 0, time.UTC)
	input := Input{
		Title: "资源推荐", Content: "资源：https://a.test https://b.test https://bit.ly/x 这些是相关链接，供参考。", ElapsedSeconds: -1, Now: now,
	}
	first := judge.Evaluate(context.Background(), input)
	if first.Source != model.SpamSourceAI || classifier.calls != 1 {
		t.Fatalf("unexpected first decision %+v calls=%d", first, classifier.calls)
	}
	second := judge.Evaluate(context.Background(), input)
	if second.Source != model.SpamSourceAIBudget || classifier.calls != 1 {
		t.Fatalf("unexpected second decision %+v calls=%d", second, classifier.calls)
	}
}
