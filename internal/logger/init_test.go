package logger

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"testing"
	"time"
)

func TestOnlyMessageHandler(t *testing.T) {
	var buf bytes.Buffer
	handler := &OnlyMessageHandler{
		writer: &buf,
		level:  slog.LevelInfo,
	}

	record := slog.NewRecord(time.Now(), slog.LevelInfo, "hello world", 0)
	err := handler.Handle(context.Background(), record)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "hello world\n"
	if buf.String() != expected {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}
}

func TestInitLogging_Terminal(t *testing.T) {
	called := false
	InitLogging(func() bool {
		called = true
		return true
	}, false)

	if !called {
		t.Error("customIsTerminal was not called")
	}

	// We can't easily test the handler type without exposing it,
	// but we can verify that InitLogging doesn't panic
}

func TestInitLogging_NotTerminal(t *testing.T) {
	InitLogging(func() bool { return false }, false)

	// We can't easily test the handler type without exposing it,
	// but we can verify that InitLogging doesn't panic
}

func TestInitLogging_LogLevelEnv(t *testing.T) {
	orig := os.Getenv("LOG_LEVEL")
	defer func() {
		_ = os.Setenv("LOG_LEVEL", orig)
	}()

	_ = os.Setenv("LOG_LEVEL", "debug")
	InitLogging(func() bool { return true }, false)

	// We can't easily test the log level without exposing the handler,
	// but we can verify that InitLogging doesn't panic
}

func TestInitLogging_InvalidLogLevel(t *testing.T) {
	orig := os.Getenv("LOG_LEVEL")
	defer func() {
		_ = os.Setenv("LOG_LEVEL", orig)
	}()

	_ = os.Setenv("LOG_LEVEL", "invalid")
	InitLogging(func() bool { return true }, false)

	// We can't easily test the log level without exposing the handler,
	// but we can verify that InitLogging doesn't panic
}

func TestCleanLog(t *testing.T) {
	var buf bytes.Buffer
	handler := &OnlyMessageHandler{
		writer: &buf,
		level:  slog.LevelInfo,
	}
	logger := slog.New(handler)
	logger.Info("test message")
	if buf.String() != "test message\n" {
		t.Errorf("expected 'test message\\n', got %q", buf.String())
	}
}

func TestInitLogging_DefaultIsTerminal(t *testing.T) {
	// Call InitLogging with nil to use the default isTerminal
	InitLogging(nil, false)

	// We can't guarantee the handler type in all environments,
	// but we can check that InitLogging does not panic
}
