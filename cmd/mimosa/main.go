package main

import (
	"os"

	"github.com/hytromo/mimosa/internal/argsparser"
	"github.com/hytromo/mimosa/internal/logger"
	"github.com/hytromo/mimosa/internal/orchestration/actions"
	"github.com/hytromo/mimosa/internal/orchestration/orchestrator"
	log "github.com/sirupsen/logrus"
)

func main() {
	logger.InitLogging(nil)

	appOptions, err := argsparser.Parse(os.Args)
	if err != nil {
		log.Fatal(err) // if we are unable to even parse our own options, we should exit
	}

	err = orchestrator.Run(appOptions, actions.New())
	if err != nil {
		log.Fatal(err)
	}
}
