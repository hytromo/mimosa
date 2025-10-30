package docker

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"log/slog"

	"github.com/hytromo/mimosa/internal/configuration"
	argparse "github.com/hytromo/mimosa/internal/docker/arg_parse"
	fileresolution "github.com/hytromo/mimosa/internal/docker/file_resolution"
	"github.com/hytromo/mimosa/internal/hasher"
	"github.com/samber/lo"
)

type ParsedBuildCommand struct {
	FinalTag               string
	ContextPath            string   // Absolute path to the build context dir
	CmdWithoutTagArguments []string // The docker build command without any tag-related arguments that could influence the hash
	DockerfilePath         string   // Absolute path to the Dockerfile used
	DockerignorePath       string   // Absolute path to the dockerignore file used, if any
	Executable             string   // the docker executable
	Args                   []string // the raw docker command arguments
	RegistryDomain         string   // the full domain name of the registry, e.g. docker.io - extracted from the tag
}

const (
	tagFlagEq       = "--tag="
	tagShortFlagEq  = "-t="
	fileFlagEq      = "--file="
	fileShortFlagEq = "-f="
)

func extractBuildFlags(args []string) (allTags []string, additionalBuildContexts map[string]string, dockerfilePath string, err error) {
	allTags = []string{}
	dockerfilePath = ""
	additionalBuildContexts = make(map[string]string) // context name -> context path/value
	for i := 1; i < len(args); i++ {
		if args[i] == "--tag" || args[i] == "-t" {
			if i+1 < len(args) {
				allTags = append(allTags, args[i+1])
				i++ // skip next
			}
		} else if len(args[i]) > len(tagFlagEq)-1 && args[i][:len(tagFlagEq)] == tagFlagEq {
			allTags = append(allTags, args[i][len(tagFlagEq):])
		} else if len(args[i]) > len(tagShortFlagEq)-1 && args[i][:len(tagShortFlagEq)] == tagShortFlagEq {
			allTags = append(allTags, args[i][len(tagShortFlagEq):])
		} else if args[i] == "--file" || args[i] == "-f" {
			if i+1 < len(args) {
				dockerfilePath = args[i+1]
				i++ // skip next
			}
		} else if len(args[i]) > len(fileFlagEq)-1 && args[i][:len(fileFlagEq)] == fileFlagEq {
			dockerfilePath = args[i][len(fileFlagEq):]
		} else if len(args[i]) > len(fileShortFlagEq)-1 && args[i][:len(fileShortFlagEq)] == fileShortFlagEq {
			dockerfilePath = args[i][len(fileShortFlagEq):]
		} else if args[i] == "--build-context" {
			if i+1 < len(args) {
				// Handle: --build-context name=VALUE
				contextArg := args[i+1]
				if strings.Contains(contextArg, "=") {
					parts := strings.SplitN(contextArg, "=", 2)
					if len(parts) == 2 {
						additionalBuildContexts[parts[0]] = parts[1]
					}
				}
				i++ // skip the context argument
			}
		} else if strings.HasPrefix(args[i], "--build-context=") {
			// Handle: --build-context=name=VALUE
			contextArg := args[i][len("--build-context="):]
			if strings.Contains(contextArg, "=") {
				parts := strings.SplitN(contextArg, "=", 2)
				if len(parts) == 2 {
					additionalBuildContexts[parts[0]] = parts[1]
				}
			}
		}
	}

	if len(allTags) == 0 {
		return nil, nil, "", fmt.Errorf("cannot find image tag using the -t or --tag option")
	}

	return
}

// assumes the context path does not start with "-"
func findContextPath(dockerBuildArgs []string) (string, error) {
	var previousArgument string

	// skip docker build/docker buildx build args
	hasBuildx := slices.Contains(dockerBuildArgs, "buildx")
	firstIndex := 2
	if hasBuildx {
		firstIndex = 3
	}

	for i := firstIndex; i < len(dockerBuildArgs); i++ {
		arg := dockerBuildArgs[i]

		// If the current argument starts with '-', it's a flag / normal argument (could be --file, -t, --no-cache, etc.)
		if strings.HasPrefix(arg, "-") {
			previousArgument = arg // save this so as to see if the next arg is its value
			continue
		}

		// If the previous argument was a flag (and didn't include '='), assume this is its value
		if strings.HasPrefix(previousArgument, "-") && !strings.Contains(previousArgument, "=") {
			// This argument is being used as the value of the previous flag, so skip it
			previousArgument = arg
			continue
		}

		// If we reach here, the argument:
		// - doesn't start with '-'
		// - isn't the value of a previous flag
		// So we assume it's the build context (e.g. ".", "./dir", etc.)
		return arg, nil
	}

	// If no suitable argument was found, return an error
	return "", fmt.Errorf("context path not found")
}

func buildCommandWithoutTagArguments(dockerBuildCmd []string) []string {
	var cmdWithoutTagArguments []string
	for i := 0; i < len(dockerBuildCmd); i++ {
		if dockerBuildCmd[i] == "--tag" || dockerBuildCmd[i] == "-t" {
			i++ // skip this and the next argument (--tag/-t <TAG>)
			continue
		} else if strings.HasPrefix(dockerBuildCmd[i], tagFlagEq) || strings.HasPrefix(dockerBuildCmd[i], tagShortFlagEq) {
			continue // skip this argument (--tag/-t=<TAG>)
		}

		// non-tag argument - add to the command
		cmdWithoutTagArguments = append(cmdWithoutTagArguments, dockerBuildCmd[i])
	}
	return cmdWithoutTagArguments
}

func ParseBuildCommand(dockerBuildCmd []string) (parsedCommand configuration.ParsedCommand, err error) {
	slog.Debug("Parsing command", "command", dockerBuildCmd)
	parsedCommand.Command = dockerBuildCmd

	if len(dockerBuildCmd) < 2 {
		return parsedCommand, fmt.Errorf("not enough arguments for a docker build command")
	}

	// Use argsparser logic to check docker and build/buildx
	executable := dockerBuildCmd[0]
	if executable != "docker" {
		return parsedCommand, fmt.Errorf("only 'docker' executable is supported for caching, got: %s", executable)
	}
	args := dockerBuildCmd[1:]
	if len(args) < 1 {
		return parsedCommand, fmt.Errorf("missing docker subcommand")
	}
	firstArg := args[0]
	if firstArg != "build" && firstArg != "buildx" {
		return parsedCommand, fmt.Errorf("only image building is supported")
	}

	allTags, allBuildContexts, dockerfilePath, err := extractBuildFlags(args)

	// dockerfilePath is relative to CWD

	if err != nil {
		return parsedCommand, err
	}

	contextPath, err := findContextPath(dockerBuildCmd)
	if err != nil {
		return parsedCommand, err
	}

	// Get absolute path for contextPath
	absCtxPath, err := filepath.Abs(contextPath)
	if err != nil {
		return parsedCommand, err
	}
	contextPath = absCtxPath

	allRegistryDomains := []string{}
	for _, tag := range allTags {
		allRegistryDomains = append(allRegistryDomains, argparse.ExtractRegistryDomain(tag))
	}

	cwd, err := os.Getwd()

	if err != nil {
		return parsedCommand, err
	}

	dockerfilePath = fileresolution.ResolveAbsoluteDockerfilePath(cwd, dockerfilePath)
	dockerignorePath := fileresolution.ResolveAbsoluteDockerIgnorePath(contextPath, dockerfilePath)

	// add the context in all the build contexts:
	allBuildContexts[configuration.MainBuildContextName] = contextPath

	parsedCommand.Hash = hasher.HashBuildCommand(hasher.DockerBuildCommand{
		DockerfilePath:         dockerfilePath,
		DockerignorePath:       dockerignorePath,
		BuildContexts:          allBuildContexts,
		AllRegistryDomains:     lo.Uniq(allRegistryDomains),
		CmdWithoutTagArguments: buildCommandWithoutTagArguments(dockerBuildCmd),
	})
	parsedCommand.TagsByTarget = map[string][]string{
		"default": allTags,
	}

	return parsedCommand, nil
}
