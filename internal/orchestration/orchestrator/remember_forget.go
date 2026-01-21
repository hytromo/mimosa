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

	if forgetOptions.Enabled {
		// For forget, use local cache (registry cache doesn't support forget)
		cacheEntry := act.GetCacheEntry(parsedCommand.Hash)
		return act.RemoveCacheEntry(cacheEntry, dryRun)
	}

	// remember branch
	// Determine cache location - default to registry if not specified
	cacheLocation := rememberOptions.CacheLocation
	if cacheLocation == "" {
		cacheLocation = configuration.CacheLocationRegistry
	}

	var cacheHit bool

	if cacheLocation == configuration.CacheLocationRegistry {
		// Registry-based cache
		exists, cacheTagsByTarget, err := act.CheckRegistryCacheExists(parsedCommand.Hash, parsedCommand.TagsByTarget)
		if err != nil {
			slog.Warn("Error checking registry cache, falling back to command execution", "error", err)
			fallbackToExecutingCommandIfRemembering(err, dryRun, rememberOptions.Enabled, act, parsedCommand.Command)
			return err
		}

		cacheHit = exists

		if cacheHit {
			// Retag from cache tags to requested tags
			err = act.RetagFromCacheTags(cacheTagsByTarget, parsedCommand.TagsByTarget, dryRun)
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
	} else {
		// Local cache (existing behavior)
		cacheEntry := act.GetCacheEntry(parsedCommand.Hash)
		cacheHit = cacheEntry.Exists()

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

		// Save/update local cache
		err = act.SaveCache(cacheEntry, parsedCommand.TagsByTarget, dryRun)
		if err != nil {
			slog.Warn("Failed to save local cache", "error", err)
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
