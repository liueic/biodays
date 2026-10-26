package antispam

import (
	"strings"
	"testing"

	"biodays/server/internal/model"
)

func TestScoreCleanSubmissionAllows(t *testing.T) {
	result := Score(ScoreInput{
		Title:          "想了解 GFP 的发现故事",
		Content:        "希望讲讲荧光蛋白怎样从水母研究走向实验室应用。",
		ElapsedSeconds: -1,
	}, 30, 70)
	if result.Verdict != model.SpamVerdictAllow {
		t.Fatalf("expected allow, got %s (%v)", result.Verdict, result.Signals)
	}
}

func TestScoreManyLinksReviews(t *testing.T) {
	result := Score(ScoreInput{
		Title:   "资源",
		Content: "https://a.test https://b.test https://c.test https://d.test https://e.test https://f.test",
	}, 30, 70)
	if result.Score < 30 {
		t.Fatalf("expected review score, got %d", result.Score)
	}
}

func TestScoreDuplicateContentReviews(t *testing.T) {
	result := Score(ScoreInput{
		Title:          "重复内容",
		Content:        "这是一段正常长度的重复反馈内容，用于测试重复检测。",
		DuplicateCount: 3,
	}, 30, 70)
	if result.Verdict != model.SpamVerdictReview {
		t.Fatalf("expected review, got %s (%v)", result.Verdict, result.Signals)
	}
}

func TestScoreSignalsAreBoundedAndUnique(t *testing.T) {
	result := Score(ScoreInput{
		Title:          strings.Repeat("A", 40),
		Content:        "加微信 13800138000 https://bit.ly/x\u200b",
		Email:          "x@mailinator.com",
		ElapsedSeconds: 1,
		DuplicateCount: 5,
	}, 30, 70)
	if result.Score > 100 {
		t.Fatalf("score must be capped at 100, got %d", result.Score)
	}
	seen := map[string]struct{}{}
	for _, signal := range result.Signals {
		if _, ok := seen[signal]; ok {
			t.Fatalf("duplicate signal %q", signal)
		}
		seen[signal] = struct{}{}
	}
	if result.Verdict != model.SpamVerdictSpam {
		t.Fatalf("expected spam, got %s", result.Verdict)
	}
}
