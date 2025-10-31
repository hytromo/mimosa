package logger

import (
	"bytes"
	"context"
	"log/slog"
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
