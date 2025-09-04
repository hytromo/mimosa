package actions

import (
	"os"
	"os/exec"
	"strings"

	"log/slog"
)

func (a *Actioner) RunCommand(dryRun bool, command []string) int {
	if dryRun {
		slog.Info("> DRY RUN: command would be run", "command", strings.Join(command, " "))
		return 0
	}

	if len(command) == 0 {
		slog.Error("Command is nil or empty")
		return 1
	}

	if command[0] == "" {
		slog.Error("Command name is empty")
		return 1
	}

	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(interface{ ExitStatus() int }); ok {
				// trying to exit the same using the same exit status like docker
				return status.ExitStatus()
			}
		}
		return 1
	}

	return 0
}

func (a *Actioner) ExitProcessWithCode(code int) {
	os.Exit(code)
}
