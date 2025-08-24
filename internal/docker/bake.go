package docker

import (
	"os"
	"strings"

	composecli "github.com/compose-spec/compose-go/v2/cli"
	"github.com/docker/buildx/bake"
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

// ParsedBakeCommand represents a parsed docker bake command
type ParsedBakeCommand struct {
	Targets map[string]*bake.Target // parsed targets from bake files
}

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
// func ParseBakeCommand(dockerBakeCmd []string) (ParsedBakeCommand, error) {
// 	log.Debugln("Parsing bake command:", dockerBakeCmd)
// 	if len(dockerBakeCmd) < 2 {
// 		return ParsedBakeCommand{}, fmt.Errorf("not enough arguments for a docker bake command")
// 	}

// 	// Check executable
// 	executable := dockerBakeCmd[0]
// 	if executable != "docker" {
// 		return ParsedBakeCommand{}, fmt.Errorf("only 'docker' executable is supported for caching, got: %s", executable)
// 	}

// 	args := dockerBakeCmd[1:]
// 	if len(args) < 1 {
// 		return ParsedBakeCommand{}, fmt.Errorf("missing docker subcommand")
// 	}

// 	firstArg := args[0]
// 	if firstArg != "buildx" {
// 		return ParsedBakeCommand{}, fmt.Errorf("only buildx is supported for bake commands")
// 	}

// 	if len(args) < 2 || args[1] != "bake" {
// 		return ParsedBakeCommand{}, fmt.Errorf("only bake subcommand is supported")
// 	}

// 	// Extract flags
// 	bakeFiles, targetNames, overrides, err := extractBakeFlags(args)
// 	if err != nil {
// 		return ParsedBakeCommand{}, fmt.Errorf("failed to extract bake flags: %w", err)
// 	}

// 	if len(bakeFiles) == 0 {
// 		return ParsedBakeCommand{}, fmt.Errorf("no bake files found")
// 	}

// 	// Read bake files
// 	ctx := context.Background()
// 	files, err := bake.ReadLocalFiles(bakeFiles, nil, nil)
// 	if err != nil {
// 		return ParsedBakeCommand{}, fmt.Errorf("failed to read bake files: %w", err)
// 	}

// 	// Parse targets
// 	targets, _, err := bake.ReadTargets(ctx, files, targetNames, overrides, nil, nil)
// 	if err != nil {
// 		return ParsedBakeCommand{}, fmt.Errorf("failed to parse targets: %w", err)
// 	}

// 	parsedBakeCommand := ParsedBakeCommand{
// 		Targets: targets,
// 	}

// 	if log.IsLevelEnabled(log.DebugLevel) {
// 		log.Debugln("Parsed bake command:")
// 		log.Debugln("  Bake files:", bakeFiles)
// 		log.Debugln("  Target names:", targetNames)
// 		for name, target := range targets {
// 			log.Debugln("  Target:", name, "Tags:", target.Tags)
// 		}
// 	}

// 	return parsedBakeCommand, nil
// }
