package orchestrator

import (
	"errors"
	"strconv"

	"github.com/hytromo/mimosa/internal/configuration"
	"github.com/hytromo/mimosa/internal/logger"
	"github.com/hytromo/mimosa/internal/orchestration/actions"
	log "github.com/sirupsen/logrus"
)

func HandleRememberOrForgetSubcommands(rememberOptions configuration.RememberSubcommandOptions, forgetOptions configuration.ForgetSubcommandOptions, act actions.Actions) error {
	var commandContainer configuration.CommandContainer
	dryRun := false
	if rememberOptions.Enabled {
		dryRun = rememberOptions.DryRun
		commandContainer = rememberOptions
	} else if forgetOptions.Enabled {
		dryRun = forgetOptions.DryRun
		commandContainer = forgetOptions
	} else {
		return errors.New("no subcommand enabled")
	}

	parsedCommand, err := getCommandHash(commandContainer, act)

	if err != nil {
		fallbackToExecutingCommandIfRemembering(err, dryRun, rememberOptions.Enabled, act, parsedCommand.Command)
		return err
	}

	log.Debugf("Final calculated command hash: %s", parsedCommand.Hash)

	cacheEntry := act.GetCacheEntry(parsedCommand.Hash)

	if forgetOptions.Enabled {
		return act.RemoveCacheEntry(cacheEntry, dryRun)
	}
	// remember branch

	cacheHit := cacheEntry.Exists()

	if cacheHit {
		// retag
		err = act.Retag(cacheEntry, parsedCommand, dryRun)
		if err != nil {
			fallbackToExecutingCommandIfRemembering(err, dryRun, rememberOptions.Enabled, act, parsedCommand.Command)
			return err
		}
	} else {
		// run command
		exitCode := act.RunCommand(dryRun, parsedCommand.Command)

		if exitCode != 0 {
			// not saving cache if command fails
			act.ExitProcessWithCode(exitCode)
			return errors.New("error running command - exit code: " + strconv.Itoa(exitCode))
		}
	}

	logger.CleanLog.Infof("mimosa-cache-hit: %t", cacheHit)

	// regardless of whether the cache already exists or not, we need to save/update it
	return act.SaveCache(cacheEntry, parsedCommand.TagsByTarget, dryRun)
}

func getCommandHash(commandContainer configuration.CommandContainer, act actions.Actions) (configuration.ParsedCommand, error) {
	return act.ParseCommand(commandContainer.GetCommandToRun())
}

func fallbackToExecutingCommandIfRemembering(err error, dryRun bool, remembering bool, act actions.Actions, commandToRun []string) {
	if !remembering {
		// only if we are remembering we need to fallback to actually running the command
		return
	}

	log.Errorf("Falling back to plain command execution: %s due to error: %s", commandToRun, err.Error())

	exitCode := act.RunCommand(dryRun, commandToRun)

	if exitCode != 0 {
		log.Errorf("Error running command: %s with exit code: %d", commandToRun, exitCode)
	}

	act.ExitProcessWithCode(exitCode)
}
