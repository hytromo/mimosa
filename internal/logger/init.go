package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
)

// Global log level for checking if debug logging is enabled
var globalLogLevel slog.Level = slog.LevelInfo

func InitLogging(forceDebug bool) {
	var level slog.Level
	if forceDebug {
		level = slog.LevelDebug
	} else {
		envLevel := strings.ToLower(os.Getenv("LOG_LEVEL"))
		switch envLevel {
		case "debug":
			level = slog.LevelDebug
		case "info":
			level = slog.LevelInfo
		case "warn", "warning":
			level = slog.LevelWarn
		case "error":
			level = slog.LevelError
		default:
			level = slog.LevelInfo
		}
	}

	slog.SetLogLoggerLevel(level)

	// Store the global log level
	globalLogLevel = level
}

// OnlyMessageHandler is a custom slog handler that only outputs the message
type OnlyMessageHandler struct {
	writer io.Writer
	level  slog.Level
}

func (h *OnlyMessageHandler) SetWriter(writer io.Writer) {
	h.writer = writer
}

func (h *OnlyMessageHandler) GetWriter() io.Writer {
	return h.writer
}

func (h *OnlyMessageHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *OnlyMessageHandler) Handle(ctx context.Context, record slog.Record) error {
	_, err := h.writer.Write([]byte(record.Message + "\n"))
	return err
}

func (h *OnlyMessageHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}

func (h *OnlyMessageHandler) WithGroup(name string) slog.Handler {
	return h
}

// CleanLog is a logger that only outputs messages without any formatting
var CleanLog = slog.New(&OnlyMessageHandler{
	writer: os.Stdout,
	level:  slog.LevelInfo,
})

// IsLevelEnabled checks if the given level is enabled (equivalent to logrus IsLevelEnabled)
func IsLevelEnabled(level slog.Level) bool {
	return level >= globalLogLevel
}

// IsDebugEnabled checks if debug level is enabled
func IsDebugEnabled() bool {
	return IsLevelEnabled(slog.LevelDebug)
}
