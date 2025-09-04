package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"

	"golang.org/x/term"
)

// Global log level for checking if debug logging is enabled
var globalLogLevel slog.Level = slog.LevelInfo

func isTerminal() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

func InitLogging(customIsTerminal func() bool, forceDebug bool) {
	if customIsTerminal == nil {
		customIsTerminal = isTerminal
	}

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

	// Store the global log level
	globalLogLevel = level

	var handler slog.Handler
	if customIsTerminal() {
		// For terminal output, use a simple text handler that only shows messages
		handler = &OnlyMessageHandler{
			writer: os.Stdout,
			level:  level,
		}
	} else {
		// For non-terminal output, use JSON handler for structured logging
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: level,
		})
	}

	slog.SetDefault(slog.New(handler))
}

// OnlyMessageHandler is a custom slog handler that only outputs the message
type OnlyMessageHandler struct {
	writer io.Writer
	level  slog.Level
}

// SetWriter sets the writer for the handler (useful for testing)
func (h *OnlyMessageHandler) SetWriter(writer io.Writer) {
	h.writer = writer
}

// GetWriter returns the current writer (useful for testing)
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
	// For simplicity, we don't support attributes in the only-message handler
	return h
}

func (h *OnlyMessageHandler) WithGroup(name string) slog.Handler {
	// For simplicity, we don't support groups in the only-message handler
	return h
}

// CleanLog is a logger that only outputs messages without any formatting
var CleanLog = slog.New(&OnlyMessageHandler{
	writer: os.Stdout,
	level:  slog.LevelInfo,
})

// GetCleanLogHandler returns the handler used by CleanLog (useful for testing)
func GetCleanLogHandler() *OnlyMessageHandler {
	if handler, ok := CleanLog.Handler().(*OnlyMessageHandler); ok {
		return handler
	}
	return nil
}

// IsLevelEnabled checks if the given level is enabled (equivalent to logrus IsLevelEnabled)
func IsLevelEnabled(level slog.Level) bool {
	return level >= globalLogLevel
}

// IsDebugEnabled checks if debug level is enabled
func IsDebugEnabled() bool {
	return IsLevelEnabled(slog.LevelDebug)
}
