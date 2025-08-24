package actions

import (
	"errors"

	"github.com/hytromo/mimosa/internal/configuration"
	"github.com/hytromo/mimosa/internal/docker"
)

func (a *Actioner) ParseCommand(command []string) (configuration.ParsedCommand, error) {

	// "docker build ." is the smallest possible command
	if len(command) < 3 {
		return configuration.ParsedCommand{}, errors.New("command is too short")
	}

	if command[0] != "docker" {
		return configuration.ParsedCommand{}, errors.New("command must start with 'docker'")
	}

	if command[1] == "build" {
		return docker.ParseBuildCommand(command)
	}

	if command[1] != "buildx" {
		return configuration.ParsedCommand{}, errors.New("sub-command must either be 'build' or 'buildx'")
	}

	// "docker buildx bake/build ." is the smallest possible command for buildx
	if len(command) < 4 {
		return configuration.ParsedCommand{}, errors.New("command is too short")
	}

	if command[2] == "build" {
		return docker.ParseBuildCommand(command)
	} else if command[2] == "bake" {
		return docker.ParseBakeCommand(command)
	}

	return configuration.ParsedCommand{}, errors.New("sub-command must either be 'build' or 'bake'")

}
