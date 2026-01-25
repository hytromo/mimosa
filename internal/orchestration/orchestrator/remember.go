package orchestrator

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"log/slog"

	"github.com/hytromo/mimosa/internal/configuration"
	"github.com/hytromo/mimosa/internal/logger"
	"github.com/hytromo/mimosa/internal/orchestration/actions"
)

// hasPushFlag checks if the command will push to a registry.
// This is true if:
// - --push flag exists
// - --output type=registry,... exists (or -o type=registry,...)
func hasPushFlag(command []string) bool {
	for i, arg := range command {
		if arg == "--push" {
			return true
		}
		// Check for --output type=registry or -o type=registry (space-separated)
		if arg == "--output" || arg == "-o" {
			if i+1 < len(command) && strings.Contains(command[i+1], "type=registry") {
				return true
			}
		}
		// Check for --output=type=registry or -o=type=registry (equals format)
		if strings.HasPrefix(arg, "--output=") || strings.HasPrefix(arg, "-o=") {
			if strings.Contains(arg, "type=registry") {
				return true
			}
		}
	}
	return false
}

func HandleRememberOrForgetSubcommands(rememberOptions configuration.RememberSubcommandOptions, forgetOptions configuration.ForgetSubcommandOptions, act actions.Actions) error {
	if !rememberOptions.Enabled {
		return errors.New("remember subcommand must be enabled")
	}

	dryRun := rememberOptions.DryRun
	commandToRun := rememberOptions.GetCommandToRun()

	if !hasPushFlag(commandToRun) {
		// unsafe to continue without a --push flag, because command success does not guarantee that the tags were pushed to the registry
		err := errors.New("--push flag not found, skipping caching behavior and running command directly")
		fallbackToSimpleCommandExecution(err, dryRun, act, commandToRun)
		return err
	}

	parsedCommand, err := act.ParseCommand(commandToRun)

	if err != nil {
		fallbackToSimpleCommandExecution(err, dryRun, act, parsedCommand.Command)
		return err
	}

	slog.Debug("Final calculated command hash", "hash", parsedCommand.Hash)

	// Registry-based cache
	exists, cacheTagsByTarget, err := act.CheckRegistryCacheExists(parsedCommand.Hash, parsedCommand.TagsByTarget)
	if err != nil {
		slog.Warn("Error checking registry cache, falling back to command execution", "error", err)
		fallbackToSimpleCommandExecution(err, dryRun, act, parsedCommand.Command)
		return err
	}

	cacheHit := exists

	if cacheHit {
		// Retag from cache tags to requested tags (each pair is cache tag -> new tag in the SAME repository)
		err = act.RetagFromCacheTags(cacheTagsByTarget, dryRun)
		if err != nil {
			fallbackToSimpleCommandExecution(err, dryRun, act, parsedCommand.Command)
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

func fallbackToSimpleCommandExecution(err error, dryRun bool, act actions.Actions, commandToRun []string) {
	slog.Error("Falling back to plain command execution", "command", commandToRun, "error", err.Error())

	exitCode := act.RunCommand(dryRun, commandToRun)

	if exitCode != 0 {
		slog.Error("Error running command", "command", commandToRun, "exitCode", exitCode)
	}

	act.ExitProcessWithCode(exitCode)
}
