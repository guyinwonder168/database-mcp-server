// logger.go
package log

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
)

// Logger is the shared structured logger instance.
var (
	Logger *log.Logger
)

// Init initializes the logger with JSON output and file rotation (500kB limit).
func Init(logFile string) error {
	rotator, err := rotatelogs.New(
		logFile+".%Y%m%d%H%M",
		rotatelogs.WithLinkName(logFile),
		rotatelogs.WithRotationSize(500*1024), // 500KiB
		rotatelogs.WithMaxAge(7*24*time.Hour),
	)
	if err != nil {
		return err
	}

	// MCP servers communicate over stdout; avoid corrupting stdio by default.
	// Enable stdout logging only if explicitly requested.
	var writer io.Writer = rotator
	if v := os.Getenv("MCP_LOG_TO_STDOUT"); strings.EqualFold(v, "true") {
		writer = io.MultiWriter(os.Stdout, rotator)
	}

	Logger = log.New(writer, "", 0)
	return nil
}

// sensitiveKeys are exact (lowercased) map keys that must always be redacted.
var sensitiveKeys = map[string]struct{}{
	"password": {},
	"pass":     {},
	"passwd":   {},
	"aes_key":  {},
	"aeskey":   {},
	"secret":   {},
	"api_key":  {},
	"apikey":   {},
	"token":    {},
	"auth":     {},
	"key":      {}, // conservative: single generic key
}

// regex patterns to scrub inline credential substrings inside string values (DSNs, connection strings, etc.)
var inlineRedactPatterns = []*regexp.Regexp{
	// password=... (Postgres style)
	regexp.MustCompile(`(?i)(password\s*=\s*)([^ \t]+)`),
	// user:pass@ in URLs
	regexp.MustCompile(`([a-zA-Z]+://[^:\s/]+:)([^@/]+)(@)`),
	// mysql style ...password=foo&
	regexp.MustCompile(`(?i)(password=)([^&;]+)`),
	// generic key=VALUE (only for obviously secret-looking keys)
	regexp.MustCompile(`(?i)(aes_key|api_key|apikey|token|secret|auth|key)(=|:)([^;,\s]+)`),
}

// sanitizeKV redacts values for sensitive keys and scrubs inline credential patterns within strings.
func sanitizeKV(k string, v interface{}) interface{} {
	lk := strings.ToLower(k)
	if _, ok := sensitiveKeys[lk]; ok {
		return "***REDACTED***"
	}
	// Heuristic: if key contains 'password' or ends with '_password'
	if strings.Contains(lk, "password") {
		return "***REDACTED***"
	}

	switch val := v.(type) {
	case string:
		s := val
		for _, re := range inlineRedactPatterns {
			s = re.ReplaceAllString(s, "$1***REDACTED***$3")
		}
		return s
	case error:
		s := val.Error()
		for _, re := range inlineRedactPatterns {
			s = re.ReplaceAllString(s, "$1***REDACTED***$3")
		}
		return s
	default:
		return v
	}
}

// sanitizeFields clones and sanitizes the provided fields map (non-destructive).
func sanitizeFields(fields map[string]interface{}) map[string]interface{} {
	if fields == nil {
		return nil
	}
	out := make(map[string]interface{}, len(fields))
	for k, v := range fields {
		out[k] = sanitizeKV(k, v)
	}
	return out
}

// JSONLog logs a structured JSON message with automatic credential redaction.
func JSONLog(level, msg string, fields map[string]interface{}) {
	if Logger == nil {
		// fallback to stdout if not initialized (e.g., in tests)
		Logger = log.New(os.Stdout, "", 0)
	}
	entry := map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"level":     level,
		"msg":       msg,
	}

	safe := sanitizeFields(fields)
	for k, v := range safe {
		entry[k] = v
	}

	b, _ := json.Marshal(entry)
	Logger.Println(string(b))
}
