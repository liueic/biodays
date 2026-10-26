package antispam

import "strings"

var disposableDomains = map[string]struct{}{
	"10minutemail.com":  {},
	"discard.email":     {},
	"getnada.com":       {},
	"guerrillamail.com": {},
	"mailinator.com":    {},
	"sharklasers.com":   {},
	"tempmail.com":      {},
	"temp-mail.org":     {},
	"throwawaymail.com": {},
	"trashmail.com":     {},
	"yopmail.com":       {},
}

var shortenerHosts = map[string]struct{}{
	"bit.ly":      {},
	"buff.ly":     {},
	"cutt.ly":     {},
	"goo.gl":      {},
	"is.gd":       {},
	"ow.ly":       {},
	"rebrand.ly":  {},
	"s.id":        {},
	"t.cn":        {},
	"t.co":        {},
	"tinyurl.com": {},
}

var spamKeywords = []string{
	"加微信", "加vx", "vx:", "qq群", "博彩", "赌博", "时时彩", "刷单",
	"代购", "贷款", "办证", "约炮", "私聊", "telegram", "whatsapp",
	"backlink", "casino", "betting", "crypto", "porn",
}

func IsDisposableDomain(domain string) bool {
	_, ok := disposableDomains[strings.ToLower(strings.TrimSpace(domain))]
	return ok
}

func IsShortenerHost(host string) bool {
	_, ok := shortenerHosts[strings.ToLower(strings.TrimSpace(host))]
	return ok
}

func ContainsSpamKeyword(text string) bool {
	lower := strings.ToLower(text)
	for _, keyword := range spamKeywords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}
