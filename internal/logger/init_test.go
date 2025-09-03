package logger

import (
	"bytes"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestOnlyMessageFormatter(t *testing.T) {
	formatter := &OnlyMessageFormatter{}
	entry := &logrus.Entry{Message: "hello world"}
	out, err := formatter.Format(entry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "hello world\n"
	if string(out) != expected {
		t.Errorf("expected %q, got %q", expected, string(out))
	}
}

func TestInitLogging_Terminal(t *testing.T) {
	origLevel := logrus.GetLevel()
	defer logrus.SetLevel(origLevel)

	called := false
	InitLogging(func() bool {
		called = true
		return true
	}, false)

	if !called {
		t.Error("customIsTerminal was not called")
	}

	// Check formatter type
	if _, ok := logrus.StandardLogger().Formatter.(*OnlyMessageFormatter); !ok {
		t.Errorf("expected OnlyMessageFormatter, got %T", logrus.StandardLogger().Formatter)
	}
}

func TestInitLogging_NotTerminal(t *testing.T) {
	origLevel := logrus.GetLevel()
	defer logrus.SetLevel(origLevel)

	InitLogging(func() bool { return false }, false)

	if _, ok := logrus.StandardLogger().Formatter.(*logrus.TextFormatter); !ok {
		t.Errorf("expected TextFormatter, got %T", logrus.StandardLogger().Formatter)
	}
}

func TestInitLogging_LogLevelEnv(t *testing.T) {
	orig := os.Getenv("LOG_LEVEL")
	defer func() {
		_ = os.Setenv("LOG_LEVEL", orig)
	}()

	_ = os.Setenv("LOG_LEVEL", "debug")
	InitLogging(func() bool { return true }, false)

	if logrus.GetLevel() != logrus.DebugLevel {
		t.Errorf("expected log level debug, got %v", logrus.GetLevel())
	}
}

func TestInitLogging_InvalidLogLevel(t *testing.T) {
	orig := os.Getenv("LOG_LEVEL")
	defer func() {
		_ = os.Setenv("LOG_LEVEL", orig)
	}()

	_ = os.Setenv("LOG_LEVEL", "invalid")
	InitLogging(func() bool { return true }, false)

	if logrus.GetLevel() != logrus.InfoLevel {
		t.Errorf("expected log level info, got %v", logrus.GetLevel())
	}
}

func TestCleanLog(t *testing.T) {
	var buf bytes.Buffer
	logger := &logrus.Logger{
		Out:       &buf,
		Formatter: &OnlyMessageFormatter{},
		Level:     logrus.InfoLevel,
	}
	logger.Info("test message")
	if buf.String() != "test message\n" {
		t.Errorf("expected 'test message\\n', got %q", buf.String())
	}
}

func TestInitLogging_DefaultIsTerminal(t *testing.T) {
	origLevel := logrus.GetLevel()
	defer logrus.SetLevel(origLevel)

	// Call InitLogging with nil to use the default isTerminal
	InitLogging(nil, false)

	// We can't guarantee the formatter type in all environments,
	// but we can check that InitLogging does not panic and sets a valid formatter.
	if logrus.StandardLogger().Formatter == nil {
		t.Error("expected a non-nil formatter")
	}
}
