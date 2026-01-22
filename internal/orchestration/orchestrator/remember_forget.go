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
	if !rememberOptions.Enabled {
		return errors.New("remember subcommand must be enabled")
	}

	dryRun := rememberOptions.DryRun

	parsedCommand, err := act.ParseCommand(rememberOptions.GetCommandToRun())

	if err != nil {
		fallbackToExecutingCommandIfRemembering(err, dryRun, rememberOptions.Enabled, act, parsedCommand.Command)
		return err
	}

	slog.Debug("Final calculated command hash", "hash", parsedCommand.Hash)

	// Registry-based cache
	exists, cacheTagsByTarget, err := act.CheckRegistryCacheExists(parsedCommand.Hash, parsedCommand.TagsByTarget)
	if err != nil {
		slog.Warn("Error checking registry cache, falling back to command execution", "error", err)
		fallbackToExecutingCommandIfRemembering(err, dryRun, rememberOptions.Enabled, act, parsedCommand.Command)
		return err
	}

	cacheHit := exists

	if cacheHit {
		// Retag from cache tags to requested tags (each pair is cache tag -> new tag in the SAME repository)
		err = act.RetagFromCacheTags(cacheTagsByTarget, dryRun)
		if err != nil {
			fallbackToExecutingCommandIfRemembering(err, dryRun, rememberOptions.Enabled, act, parsedCommand.Command)
			return err
		}
	} else {
		// Run command
		exitCode := act.RunCommand(dryRun, parsedCommand.Command)

		if exitCode != 0 {
			// not saving cache if command fails
			act.ExitProcessWithCode(exitCode)
			return errors.New("error running command - exit code: " + strconv.Itoa(exitCode))
		}

		// After successful build, create cache tags
		err = act.SaveRegistryCacheTags(parsedCommand.Hash, parsedCommand.TagsByTarget, dryRun)
		if err != nil {
			slog.Warn("Failed to save registry cache tags", "error", err)
			// Don't fail the command if cache tag creation fails
		}
	}

	logger.CleanLog.Info(fmt.Sprintf("mimosa-cache-hit: %t", cacheHit))

	return nil
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
