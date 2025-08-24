package actions

import (
	"os"
	"os/exec"
	"strings"

	log "github.com/sirupsen/logrus"
)

func (a *Actioner) RunCommand(dryRun bool, command []string) int {
	if dryRun {
		log.Infoln("> DRY RUN: command would be run:", strings.Join(command, " "))
		return 0
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
