package orchestrator

import (
	"errors"
	"fmt"
	"strconv"

	"log/slog"

	"github.com/hytromo/mimosa/internal/configuration"
	"github.com/hytromo/mimosa/internal/logger"
	"github.com/hytromo/mimosa/internal/orchestration/actions"
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

	parsedCommand, err := act.ParseCommand(commandContainer.GetCommandToRun())

	if err != nil {
		fallbackToExecutingCommandIfRemembering(err, dryRun, rememberOptions.Enabled, act, parsedCommand.Command)
		return err
	}

	slog.Debug("Final calculated command hash", "hash", parsedCommand.Hash)

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

	logger.CleanLog.Info(fmt.Sprintf("mimosa-cache-hit: %t", cacheHit))

	// regardless of whether the cache already exists or not, we need to save/update it on disk
	return act.SaveCache(cacheEntry, parsedCommand.TagsByTarget, dryRun)
}

func fallbackToExecutingCommandIfRemembering(err error, dryRun bool, remembering bool, act actions.Actions, commandToRun []string) {
	if !remembering {
		// only if we are remembering we need to fallback to actually running the command
		return
	}

	slog.Error("Falling back to plain command execution", "command", commandToRun, "error", err.Error())

	exitCode := act.RunCommand(dryRun, commandToRun)

	if exitCode != 0 {
		slog.Error("Error running command", "command", commandToRun, "exitCode", exitCode)
	}

	act.ExitProcessWithCode(exitCode)
}
