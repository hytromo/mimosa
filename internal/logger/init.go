package logger

import (
	"os"

	"github.com/sirupsen/logrus"
	"golang.org/x/term"
)

type OnlyMessageFormatter struct{}

func (f *OnlyMessageFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	return []byte(entry.Message + "\n"), nil
}

func isTerminal() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

func InitLogging(customIsTerminal func() bool) {
	if customIsTerminal == nil {
		customIsTerminal = isTerminal
	}
	level, err := logrus.ParseLevel(os.Getenv("LOG_LEVEL"))
	if err != nil {
		level = logrus.InfoLevel
	}

	logrus.SetLevel(level)

	if customIsTerminal() {
		logrus.SetFormatter(&OnlyMessageFormatter{})
	} else {
		logrus.SetFormatter(&logrus.TextFormatter{
			DisableLevelTruncation: true,
		})
	}
}

var CleanLog = &logrus.Logger{
	Out:       os.Stdout,
	Formatter: &OnlyMessageFormatter{},
	Level:     logrus.InfoLevel,
}
