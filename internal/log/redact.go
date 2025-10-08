package log

import (
	"regexp"
	"strings"
)

var (
	arnPattern        = regexp.MustCompile(`arn:aws:[a-z0-9\-]+:[a-z0-9\-]*:\d{12}:[^\s]+`)
	accountIDPattern  = regexp.MustCompile(`\d{12}`)
	secretPattern     = regexp.MustCompile(`(?i)(secret|password|token|key)[\s:=]+[^\s]+`)
)

// Redact masks sensitive information in log messages
func Redact(msg string) string {
	// Redact ARNs
	msg = arnPattern.ReplaceAllStringFunc(msg, func(s string) string {
		parts := strings.Split(s, ":")
		if len(parts) >= 5 {
			parts[4] = "***" // Redact account ID
		}
		return strings.Join(parts, ":")
	})

	// Redact standalone account IDs
	msg = accountIDPattern.ReplaceAllString(msg, "************")

	// Redact potential secrets
	msg = secretPattern.ReplaceAllStringFunc(msg, func(s string) string {
		idx := strings.IndexAny(s, ":= ")
		if idx > 0 {
			return s[:idx+1] + "***"
		}
		return "***"
	})

	return msg
}

