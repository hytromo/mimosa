package actions

import (
	"errors"

	"github.com/hytromo/mimosa/internal/configuration"
	"github.com/hytromo/mimosa/internal/docker"
)

func (a *Actioner) ParseCommand(command []string) (configuration.ParsedCommand, error) {
	parsedCommand := configuration.ParsedCommand{
		// still set the original command so that it can be run if needed
		Command: command,
	}

	// "docker build ." is the smallest possible command
	if len(command) < 3 {
		return parsedCommand, errors.New("command is too short")
	}

	if command[0] != "docker" {
		return parsedCommand, errors.New("command must start with 'docker'")
	}

	if command[1] == "build" {
		return docker.ParseBuildCommand(command)
	}

	if command[1] != "buildx" {
		return parsedCommand, errors.New("sub-command must either be 'build' or 'buildx'")
	}

	// "docker buildx bake/build ." is the smallest possible command for buildx
	if len(command) < 4 {
		return parsedCommand, errors.New("command is too short")
	}

	switch command[2] {
	case "build":
		return docker.ParseBuildCommand(command)
	case "bake":
		return docker.ParseBakeCommand(command)
	default:
		return parsedCommand, errors.New("sub-command must either be 'build' or 'bake'")
	}

}
