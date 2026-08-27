// Package security provides log redaction and correlation-ID
// helpers shared across the HTTP and worker layers.
package security

import "regexp"

var sensitivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)"card_number"\s*:\s*"[^"]*"`),
	regexp.MustCompile(`(?i)"cvv"\s*:\s*"[^"]*"`),
}

// Redact removes values that must never reach logs.
func Redact(s string) string {
	for _, p := range sensitivePatterns {
		s = p.ReplaceAllString(s, `"<redacted>"`)
	}
	return s
}
