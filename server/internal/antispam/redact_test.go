package antispam

import (
	"strings"
	"testing"
)

func TestRedactRemovesContactDetailsAndURLParameters(t *testing.T) {
	text := Redact("联系我", "邮箱 a@example.com 电话 13800138000 链接 https://example.com/path?token=secret\u200b", 4000)
	for _, forbidden := range []string{"a@example.com", "13800138000", "token=secret", "\u200b"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("redacted text still contains %q: %s", forbidden, text)
		}
	}
	for _, expected := range []string{"[邮箱]", "[电话]", "[链接:example.com]"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected %q in %s", expected, text)
		}
	}
}

func TestRedactTruncatesByRunes(t *testing.T) {
	text := Redact("标题", strings.Repeat("生", 20), 5)
	if len([]rune(text)) > 5 {
		t.Fatalf("expected at most 5 runes, got %d", len([]rune(text)))
	}
}

func TestContentHashAndEmailDomain(t *testing.T) {
	if ContentHash("a", "b") == ContentHash("a", "c") {
		t.Fatal("different content must have different hashes")
	}
	if got := EmailDomain("User@Example.COM"); got != "example.com" {
		t.Fatalf("unexpected domain %q", got)
	}
}
