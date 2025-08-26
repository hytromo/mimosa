package docker

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/hytromo/mimosa/internal/configuration"
	"github.com/hytromo/mimosa/internal/hasher"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
)

type ParsedBuildCommand struct {
	FinalTag              string
	ContextPath           string   // Absolute path to the build context dir
	CmdWithTagPlaceholder []string // The docker build command with the tag replaced by "TAG" - useful for stable caching with tag differences
	DockerfilePath        string   // Absolute path to the Dockerfile used
	DockerignorePath      string   // Absolute path to the dockerignore file used, if any
	Executable            string   // the docker executable
	Args                  []string // the raw docker command arguments
	RegistryDomain        string   // the full domain name of the registry, e.g. docker.io - extracted from the tag
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
	additionalBuildContexts = make(map[string]string)
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
				additionalBuildContexts[args[i+1]] = args[i+2]
				i++ // skip next
			}
		}
	}

	if len(allTags) == 0 {
		return nil, nil, "", fmt.Errorf("cannot find image tag using the -t or --tag option")
	}

	return
}

// assumes the context path does not start with "-"
func findContextPath(args []string) (string, error) {
	var previousArgument string

	for i := 1; i < len(args); i++ {
		arg := args[i]

		if (arg == "buildx" || arg == "build") && i < 2 {
			continue
		}

		// If the current argument starts with '-', it's a flag / normal argument (could be --file, -t, --no-cache, etc.)
		if strings.HasPrefix(arg, "-") {
			previousArgument = arg // save this so as to see if the next arg is its value
			continue
		}

		// If the previous argument was a flag (and didn't include '='), assume this is its value
		if strings.HasPrefix(previousArgument, "-") && !strings.Contains(previousArgument, "=") {
			// This argument is being used as the value of the previous flag, so skip it
			previousArgument = "" // Reset previous to avoid confusion on next iteration
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

func resolveDockerfilePath(cwd, extractFromCommandDokcerfilePath string) string {
	if extractFromCommandDokcerfilePath == "" {
		extractFromCommandDokcerfilePath = "Dockerfile"
	}

	path, err := filepath.Abs(filepath.Join(cwd, extractFromCommandDokcerfilePath))

	if err == nil {
		return path
	}

	return filepath.Join(cwd, "Dockerfile")
}

func findDockerignorePath(contextPath, dockerfilePath string) string {
	dockerfileDir := filepath.Dir(dockerfilePath)
	dockerfileBase := filepath.Base(dockerfilePath)
	dockerignoreCandidate := filepath.Join(dockerfileDir, dockerfileBase+".dockerignore")
	if fi, err := os.Stat(dockerignoreCandidate); err == nil && !fi.IsDir() {
		if abs, err := filepath.Abs(dockerignoreCandidate); err == nil {
			return abs
		}
	}
	contextDockerignore := filepath.Join(contextPath, ".dockerignore")
	if fi, err := os.Stat(contextDockerignore); err == nil && !fi.IsDir() {
		if abs, err := filepath.Abs(contextDockerignore); err == nil {
			return abs
		}
	}
	return ""
}

func buildCmdWithTagsPlaceholder(dockerBuildCmd []string) []string {
	cmdWithTagPlaceholder := make([]string, len(dockerBuildCmd))
	copy(cmdWithTagPlaceholder, dockerBuildCmd)
	for i := 0; i < len(cmdWithTagPlaceholder); i++ {
		if cmdWithTagPlaceholder[i] == "--tag" || cmdWithTagPlaceholder[i] == "-t" {
			if i+1 < len(cmdWithTagPlaceholder) {
				cmdWithTagPlaceholder[i+1] = "TAG"
				break
			}
		} else if len(cmdWithTagPlaceholder[i]) > len(tagFlagEq)-1 && cmdWithTagPlaceholder[i][:len(tagFlagEq)] == tagFlagEq {
			cmdWithTagPlaceholder[i] = tagFlagEq + "TAG"
			break
		} else if len(cmdWithTagPlaceholder[i]) > len(tagShortFlagEq)-1 && cmdWithTagPlaceholder[i][:len(tagShortFlagEq)] == tagShortFlagEq {
			cmdWithTagPlaceholder[i] = tagShortFlagEq + "TAG"
			break
		}
	}
	return cmdWithTagPlaceholder
}

func extractRegistryDomain(tag string) string {
	// Split image reference into domain and remainder
	// Format: [domain/][user/]repo[:tag|@digest]
	slashParts := strings.SplitN(tag, "/", 2)

	if len(slashParts) == 1 {
		// No domain, so it's Docker Hub
		return "docker.io"
	}

	first := slashParts[0]

	// Check if the first segment looks like a domain or IP
	if strings.Contains(first, ".") || strings.Contains(first, ":") || net.ParseIP(first) != nil {
		return first
	}

	// Otherwise, it's a Docker Hub namespace like "library/ubuntu"
	return "docker.io"
}

func ParseBuildCommand(dockerBuildCmd []string) (configuration.ParsedCommand, error) {
	log.Debugln("Parsing command:", dockerBuildCmd)
	if len(dockerBuildCmd) < 2 {
		return configuration.ParsedCommand{}, fmt.Errorf("not enough arguments for a docker build command")
	}

	// Use argsparser logic to check docker and build/buildx
	executable := dockerBuildCmd[0]
	if executable != "docker" {
		return configuration.ParsedCommand{}, fmt.Errorf("only 'docker' executable is supported for caching, got: %s", executable)
	}
	args := dockerBuildCmd[1:]
	if len(args) < 1 {
		return configuration.ParsedCommand{}, fmt.Errorf("missing docker subcommand")
	}
	firstArg := args[0]
	if firstArg != "build" && firstArg != "buildx" {
		return configuration.ParsedCommand{}, fmt.Errorf("only image building is supported")
	}

	allTags, allBuildContexts, dockerfilePath, err := extractBuildFlags(args)
	if err != nil {
		return configuration.ParsedCommand{}, err
	}

	contextPath, err := findContextPath(args)
	if err != nil {
		return configuration.ParsedCommand{}, err
	}

	// Get absolute path for contextPath
	absPath, err := filepath.Abs(contextPath)
	if err == nil {
		contextPath = absPath
	}

	allRegistryDomains := []string{}
	for _, tag := range allTags {
		allRegistryDomains = append(allRegistryDomains, extractRegistryDomain(tag))
	}

	dockerfilePath = resolveDockerfilePath(contextPath, dockerfilePath)
	dockerignorePath := findDockerignorePath(contextPath, dockerfilePath)

	cmdWithTagPlaceholder := buildCmdWithTagsPlaceholder(dockerBuildCmd)

	// add the context in all the build contexts:
	allBuildContexts[configuration.MainBuildContextName] = contextPath

	return configuration.ParsedCommand{
		Command: dockerBuildCmd,
		TagsByTarget: map[string][]string{
			"default": allTags,
		},
		Hash: hasher.HashBuildCommand(hasher.DockerBuildCommand{
			DockerfilePath:        dockerfilePath,
			DockerignorePath:      dockerignorePath,
			BuildContexts:         allBuildContexts,
			AllRegistryDomains:    lo.Uniq(allRegistryDomains),
			CmdWithTagPlaceholder: cmdWithTagPlaceholder,
		}),
	}, nil
}
