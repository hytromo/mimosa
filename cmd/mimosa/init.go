package main

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

func initLogging() {
	if isTerminal() {
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
