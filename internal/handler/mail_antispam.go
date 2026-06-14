package handler

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/emersion/go-msgauth/dkim"
)

// Antispam: pure-Go heuristics, no external scanner/daemon. Messages scoring at
// or above spamThreshold are filed into the Junk folder. DKIM is verified with
// the already-bundled go-msgauth (best-effort: a DNS-less environment yields no
// verdict and contributes nothing).

const spamThreshold = 5

var spamKeywords = []string{
	"viagra", "cialis", "lottery", "you won", "you've won", "winner",
	"free money", "click here", "act now", "limited time", "risk-free",
	"work from home", "be your own boss", "weight loss", "crypto giveaway",
	"bitcoin doubler", "nigerian prince", "wire transfer", "gift card",
	"verify your account", "suspended account", "confirm your password",
}

var spamLinkRe = regexp.MustCompile(`https?://`)
var spamDangerExt = regexp.MustCompile(`(?i)\.(exe|scr|js|vbs|bat|cmd|com|pif|jar|msi)$`)
var spamDoubleExt = regexp.MustCompile(`(?i)\.(pdf|doc|docx|xls|jpg|png|txt)\.(exe|scr|js|vbs|bat|cmd|zip)$`)

// SpamSignals is the input to scoreSpam (pure, unit-testable).
type SpamSignals struct {
	Subject     string
	Body        string
	Recipients  int
	Attachments []string
	DKIM        string // "pass" | "fail" | "" (unknown)
}

// scoreSpam returns a heuristic spam score and the reasons that contributed.
func scoreSpam(s SpamSignals) (int, []string) {
	score := 0
	var reasons []string
	add := func(n int, why string) { score += n; reasons = append(reasons, why) }

	hay := strings.ToLower(s.Subject + "\n" + s.Body)
	hits := 0
	for _, kw := range spamKeywords {
		if strings.Contains(hay, kw) {
			hits++
		}
	}
	if hits > 0 {
		add(min2(hits*2, 6), "keywords")
	}

	// ALL-CAPS shouty subject
	letters, upper := 0, 0
	for _, r := range s.Subject {
		if r >= 'A' && r <= 'Z' {
			upper++
			letters++
		} else if r >= 'a' && r <= 'z' {
			letters++
		}
	}
	if letters >= 8 && upper*100/maxInt(letters, 1) >= 80 {
		add(2, "shouting subject")
	}

	if n := len(spamLinkRe.FindAllString(s.Body, -1)); n > 10 {
		add(2, "many links")
	}

	for _, fn := range s.Attachments {
		if spamDoubleExt.MatchString(fn) {
			add(4, "double-extension attachment")
			break
		}
	}
	for _, fn := range s.Attachments {
		if spamDangerExt.MatchString(fn) {
			add(3, "executable attachment")
			break
		}
	}

	if s.Recipients > 20 {
		add(1, "bulk recipients")
	}
	if s.DKIM == "fail" {
		add(2, "dkim fail")
	}
	return score, reasons
}

func min2(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// verifyDKIM returns "pass", "fail" or "" (no signature / no verdict, e.g. no DNS).
func verifyDKIM(raw []byte) string {
	vs, err := dkim.Verify(bytes.NewReader(raw))
	if err != nil || len(vs) == 0 {
		return ""
	}
	for _, v := range vs {
		if v.Err == nil {
			return "pass"
		}
	}
	return "fail"
}
