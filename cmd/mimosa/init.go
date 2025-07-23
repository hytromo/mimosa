package main

import (
	"os"

	log "github.com/sirupsen/logrus"
)

func initLogging() {
	log.SetFormatter(&log.TextFormatter{
		DisableLevelTruncation: true,
	})

	levelStr := os.Getenv("LOG_LEVEL")
	if levelStr == "" {
		levelStr = "info"
	}

	level, err := log.ParseLevel(levelStr)
	if err != nil {
		log.WithError(err).Warnf("Invalid log level '%s', defaulting to 'info'", levelStr)
		level = log.InfoLevel
	}
	log.SetLevel(level)
}
