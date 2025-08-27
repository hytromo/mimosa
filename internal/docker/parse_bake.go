package docker

import (
	"context"
	"fmt"
	"os"
	"strings"

	composecli "github.com/compose-spec/compose-go/v2/cli"
	"github.com/docker/buildx/bake"
	"github.com/hytromo/mimosa/internal/configuration"
	"github.com/hytromo/mimosa/internal/hasher"
	log "github.com/sirupsen/logrus"
)

var (
	// taken from https://docs.docker.com/build/bake/reference/
	// "Files are merged according to the lookup order" - order matters
	defaultBakeLookupOrder = append(composecli.DefaultFileNames, []string{
		"docker-bake.json",
		"docker-bake.hcl",
		"docker-bake.override.json",
		"docker-bake.override.hcl",
	}...)
)

// extractBakeFlags extracts flags from a docker bake command
func extractBakeFlags(args []string) (bakeFiles, targetNames, overrides []string, err error) {
	bakeFiles = []string{}
	targetNames = []string{}
	overrides = []string{}

	for i := 1; i < len(args); i++ {
		arg := args[i]

		switch {
		case arg == "bake":
			continue
		case arg == "--file" || arg == "-f":
			if i+1 < len(args) {
				bakeFiles = append(bakeFiles, args[i+1])
				i++ // skip next
			}
		case strings.HasPrefix(arg, "--file=") || strings.HasPrefix(arg, "-f="):
			bakeFiles = append(bakeFiles, strings.TrimPrefix(strings.TrimPrefix(arg, "--file="), "-f="))
		case arg == "--set":
			if i+1 < len(args) {
				overrides = append(overrides, args[i+1])
				i++ // skip next
			}
		case strings.HasPrefix(arg, "--set="):
			overrides = append(overrides, strings.TrimPrefix(arg, "--set="))
		case !strings.HasPrefix(arg, "-"):
			// If it doesn't start with -, it's a target name
			targetNames = append(targetNames, arg)
		}
	}

	// If no bake files specified, look for default ones
	if len(bakeFiles) == 0 {
		for _, file := range defaultBakeLookupOrder {
			if _, err := os.Stat(file); err == nil {
				bakeFiles = append(bakeFiles, file)
				break
			}
		}
	}

	// If no target names specified, use "default"
	if len(targetNames) == 0 {
		targetNames = []string{"default"}
	}

	return bakeFiles, targetNames, overrides, nil
}

// ParseBakeCommand parses a docker bake command
func ParseBakeCommand(dockerBakeCmd []string) (parsedCommand configuration.ParsedCommand, err error) {
	log.Debugln("Parsing bake command:", dockerBakeCmd)
	parsedCommand.Command = dockerBakeCmd

	// Extract flags
	bakeFiles, targetNames, overrides, err := extractBakeFlags(dockerBakeCmd[1:])
	if err != nil {
		return parsedCommand, fmt.Errorf("failed to extract bake flags: %w", err)
	}

	if len(bakeFiles) == 0 {
		return parsedCommand, fmt.Errorf("no bake files found")
	}

	// Read bake files
	ctx := context.Background()
	files, err := bake.ReadLocalFiles(bakeFiles, nil, nil)
	if err != nil {
		return parsedCommand, fmt.Errorf("failed to read bake files: %w", err)
	}

	// Parse targets
	targets, _, err := bake.ReadTargets(ctx, files, targetNames, overrides, nil, nil)
	if err != nil {
		return parsedCommand, fmt.Errorf("failed to parse bake targets: %w", err)
	}

	tagsByTarget := make(map[string][]string)
	for name, target := range targets {
		tagsByTarget[name] = target.Tags
	}

	if log.IsLevelEnabled(log.DebugLevel) {
		log.Debugln("Parsed bake command:")
		log.Debugln("  Bake files:", bakeFiles)
		log.Debugln("  Target names:", targetNames)
		for name, target := range targets {
			log.Debugln("  Target:", name, "Tags:", target.Tags)
		}
	}

	parsedCommand.TagsByTarget = tagsByTarget
	parsedCommand.Hash = hasher.HashBakeTargets(targets)

	return parsedCommand, nil
}
