package log

import (
	"fmt"
	"io"
	stdlog "log"
	"regexp"
	"strings"
)

var (
	arnPattern       = regexp.MustCompile(`arn:aws:[a-z0-9\-]+:[a-z0-9\-]*:\d{12}:[^\s]+`)
	accountIDPattern = regexp.MustCompile(`\d{12}`)
	secretPattern    = regexp.MustCompile(`(?i)(secret|password|token|key)[\s:=]+[^\s]+`)
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

// RedactWriter wraps an io.Writer and redacts sensitive data before writing.
type RedactWriter struct {
	Out io.Writer
}

func (w *RedactWriter) Write(p []byte) (int, error) {
	redacted := Redact(string(p))
	n, err := w.Out.Write([]byte(redacted))
	if err != nil {
		return n, err
	}
	return len(p), nil
}

// NewRedactedLogger returns a *log.Logger that redacts sensitive data
// (AWS account IDs, ARNs, secrets) from all log output.
func NewRedactedLogger(out io.Writer, prefix string, flag int) *stdlog.Logger {
	return stdlog.New(&RedactWriter{Out: out}, prefix, flag)
}

// Printf logs a formatted message with redaction applied.
func Printf(format string, v ...interface{}) {
	stdlog.Output(2, Redact(fmt.Sprintf(format, v...)))
}

// Println logs a message with redaction applied.
func Println(v ...interface{}) {
	stdlog.Output(2, Redact(fmt.Sprintln(v...)))
}

// Fatalf logs a fatal message with redaction applied.
func Fatalf(format string, v ...interface{}) {
	stdlog.Output(2, Redact(fmt.Sprintf(format, v...)))
	panic(Redact(fmt.Sprintf(format, v...)))
}
