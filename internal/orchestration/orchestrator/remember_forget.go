package orchestrator

import (
	"errors"
	"strconv"

	"github.com/hytromo/mimosa/internal/configuration"
	"github.com/hytromo/mimosa/internal/orchestration/actions"
	log "github.com/sirupsen/logrus"
)

func handleRememberOrForgetSubcommands(appOptions configuration.AppOptions, act actions.Actions) error {
	parsedCommand, err := getCommandHash(appOptions, act)

	if err != nil {
		fallbackToExecutingCommandIfRemembering(err, appOptions.Remember.Enabled, act, parsedCommand.Command)
		return err
	}

	cacheEntry := act.GetCacheEntry(parsedCommand.Hash)

	if appOptions.Forget.Enabled {
		return act.RemoveCacheEntry(cacheEntry, appOptions.Forget.DryRun)
	}

	// remember branch
	if cacheEntry.Exists() {
		// retag
		err = act.Retag(cacheEntry, parsedCommand, appOptions.Remember.DryRun)
		if err != nil {
			fallbackToExecutingCommandIfRemembering(err, appOptions.Remember.Enabled, act, parsedCommand.Command)
			return err
		}
	} else {
		// run command
		exitCode := act.RunCommand(parsedCommand.Command)

		if exitCode != 0 {
			act.ExitProcessWithCode(exitCode)
			return errors.New("error running command - exit code: " + strconv.Itoa(exitCode))
		}

		act.ExitProcessWithCode(exitCode) // not saving cache if command fails
		return nil
	}

	// regardless of whether the cache already exists or not, we need to save/update it
	return act.SaveCache(cacheEntry, parsedCommand.TagsByTarget, appOptions.Remember.DryRun)
}

func getCommandHash(appOptions configuration.AppOptions, act actions.Actions) (configuration.ParsedCommand, error) {
	var commandContainer configuration.CommandContainer

	if appOptions.Remember.Enabled {
		commandContainer = appOptions.Remember
	} else {
		commandContainer = appOptions.Forget
	}

	commandToRun := commandContainer.GetCommandToRun()

	return act.ParseCommand(commandToRun)
}

func fallbackToExecutingCommandIfRemembering(err error, remebering bool, act actions.Actions, commandToRun []string) {
	if !remebering {
		// only if we are remembering we need to fallback to actually running the command
		return
	}

	log.Errorf("Falling back to command execution: %s due to error: %s", commandToRun, err.Error())

	exitCode := act.RunCommand(commandToRun)

	if exitCode != 0 {
		log.Errorf("Error running command: %s with exit code: %d", commandToRun, exitCode)
	}

	act.ExitProcessWithCode(exitCode)
}
