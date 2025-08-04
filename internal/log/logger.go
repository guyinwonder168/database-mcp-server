// logger.go
package log

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"time"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
)

var (
	Logger *log.Logger
	inited bool
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
	multi := io.MultiWriter(os.Stdout, rotator)
	Logger = log.New(multi, "", 0)
	inited = true
	return nil
}

// JSONLog logs a structured JSON message.
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
	for k, v := range fields {
		entry[k] = v
	}
	b, _ := json.Marshal(entry)
	Logger.Println(string(b))
}
