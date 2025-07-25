package docker

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
	buildArgFlagEq  = "--build-arg="
)

func extractBuildFlags(args []string) (finalTag, dockerfilePath string, err error) {
	finalTag = ""
	dockerfilePath = ""
	for i := 1; i < len(args); i++ {
		if args[i] == "--tag" || args[i] == "-t" {
			if i+1 < len(args) {
				finalTag = args[i+1]
				i++ // skip next
			}
		} else if len(args[i]) > len(tagFlagEq)-1 && args[i][:len(tagFlagEq)] == tagFlagEq {
			finalTag = args[i][len(tagFlagEq):]
		} else if len(args[i]) > len(tagShortFlagEq)-1 && args[i][:len(tagShortFlagEq)] == tagShortFlagEq {
			finalTag = args[i][len(tagShortFlagEq):]
		} else if args[i] == "--file" || args[i] == "-f" {
			if i+1 < len(args) {
				dockerfilePath = args[i+1]
				i++ // skip next
			}
		} else if len(args[i]) > len(fileFlagEq)-1 && args[i][:len(fileFlagEq)] == fileFlagEq {
			dockerfilePath = args[i][len(fileFlagEq):]
		} else if len(args[i]) > len(fileShortFlagEq)-1 && args[i][:len(fileShortFlagEq)] == fileShortFlagEq {
			dockerfilePath = args[i][len(fileShortFlagEq):]
		}
	}
	return
}

func findContextPath(args []string) (string, error) {
	for i := len(args) - 1; i >= 1; i-- {
		if len(args[i]) == 0 || args[i][0] == '-' {
			continue
		}
		return args[i], nil
	}
	return "", fmt.Errorf("cannot find docker build context path")
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

func buildCmdWithTagPlaceholder(dockerBuildCmd []string) []string {
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

func ParseBuildCommand(dockerBuildCmd []string) (ParsedBuildCommand, error) {
	if len(dockerBuildCmd) < 2 {
		return ParsedBuildCommand{}, fmt.Errorf("not enough arguments for a docker build command")
	}

	// Use argsparser logic to check docker and build/buildx
	executable := dockerBuildCmd[0]
	if executable != "docker" {
		return ParsedBuildCommand{}, fmt.Errorf("only 'docker' executable is supported for caching, got: %s", executable)
	}
	args := dockerBuildCmd[1:]
	if len(args) < 1 {
		return ParsedBuildCommand{}, fmt.Errorf("missing docker subcommand")
	}
	firstArg := args[0]
	if firstArg != "build" && firstArg != "buildx" {
		return ParsedBuildCommand{}, fmt.Errorf("only image building is supported")
	}

	finalTag, dockerfilePath, _ := extractBuildFlags(args)
	registryDomain := extractRegistryDomain(finalTag)
	contextPath, err := findContextPath(args)
	if err != nil {
		return ParsedBuildCommand{}, err
	}

	// Get absolute path for contextPath
	absPath, err := filepath.Abs(contextPath)
	if err == nil {
		contextPath = absPath
	}

	dockerfilePath = resolveDockerfilePath(contextPath, dockerfilePath)
	dockerignorePath := findDockerignorePath(contextPath, dockerfilePath)

	if finalTag == "" {
		return ParsedBuildCommand{}, fmt.Errorf("cannot find image tag using the -t or --tag option")
	}

	cmdWithTagPlaceholder := buildCmdWithTagPlaceholder(dockerBuildCmd)

	return ParsedBuildCommand{
		FinalTag:              finalTag,
		ContextPath:           contextPath,
		CmdWithTagPlaceholder: cmdWithTagPlaceholder,
		DockerfilePath:        dockerfilePath,
		DockerignorePath:      dockerignorePath,
		Executable:            executable,
		Args:                  args,
		RegistryDomain:        registryDomain,
	}, nil
}

func RunCommand(command []string) int {
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	err := cmd.Run()
	if err != nil {
		log.Errorln("Command failed:", err.Error())
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
