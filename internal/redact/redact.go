// Package redact strips token and key shapes from a handoff note before it is stored.
package redact

import "regexp"

// Replacement is the literal stored in place of a matched secret.
const Replacement = "[REDACTED]"

// Patterns match secret shapes only. A bare word such as "token", a filesystem
// path, a commit hash, or ordinary prose must stay intact.
var patterns = []*regexp.Regexp{
	regexp.MustCompile(`(?s)-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----.*?-----END [A-Z0-9 ]*PRIVATE KEY-----`),
	regexp.MustCompile(`(?i)\b(?:api[_-]?key|token|secret|password|authorization)\b\s*[:=]\s*["']?[^\s"']+`),
	regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{8,}\b`),
	regexp.MustCompile(`\bxai-[A-Za-z0-9_-]{8,}\b`),
	regexp.MustCompile(`\bxox[abprs]-[A-Za-z0-9-]{10,}\b`),
	regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{8,}\.eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\b`),
	regexp.MustCompile(`\bghp_[A-Za-z0-9]{20,}\b`),
	regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{20,}\b`),
	regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`),
	regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._-]{8,}\b`),
}

// Note returns text with token and key patterns replaced.
func Note(text string) string {
	out := text
	for _, pattern := range patterns {
		out = pattern.ReplaceAllString(out, Replacement)
	}
	return out
}
