package antispam

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	emailPattern = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	phonePattern = regexp.MustCompile(`(?:\+?86[-\s]?)?1[3-9][0-9]{9}`)
	urlPattern   = regexp.MustCompile(`(?i)https?://[^\s<>"']+`)
)

// Redact removes contact details and link parameters before text leaves the
// process for AI classification. Titles are included only as context.
func Redact(title, content string, maxChars int) string {
	text := strings.TrimSpace(title) + "\n" + strings.TrimSpace(content)
	text = urlPattern.ReplaceAllStringFunc(text, func(raw string) string {
		parsed, err := url.Parse(raw)
		if err != nil || parsed.Host == "" {
			return "[链接]"
		}
		return "[链接:" + strings.ToLower(parsed.Host) + "]"
	})
	text = emailPattern.ReplaceAllString(text, "[邮箱]")
	text = phonePattern.ReplaceAllString(text, "[电话]")
	text = strings.NewReplacer(
		"\u200b", "", "\u200c", "", "\u200d", "", "\u2060", "", "\ufeff", "",
	).Replace(text)
	text = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return ' '
		}
		return r
	}, text)
	text = strings.TrimSpace(text)
	if maxChars > 0 && utf8.RuneCountInString(text) > maxChars {
		runes := []rune(text)
		text = string(runes[:maxChars])
	}
	return text
}

func ContentHash(title, content string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(title) + "\x00" + strings.TrimSpace(content)))
	return hex.EncodeToString(sum[:])
}

func EmailDomain(email string) string {
	email = strings.TrimSpace(email)
	at := strings.LastIndex(email, "@")
	if at < 0 || at == len(email)-1 {
		return ""
	}
	return strings.ToLower(email[at+1:])
}

func countURLs(text string) int {
	return len(urlPattern.FindAllString(text, -1))
}

func containsZeroWidth(text string) bool {
	return strings.ContainsAny(text, "\u200b\u200c\u200d\u2060\ufeff")
}
