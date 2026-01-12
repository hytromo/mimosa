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

// flagTemplate defines a flag whose value should be templated for hash calculation.
// This ensures that run-specific values (like temp file paths or builder IDs) don't
// affect the hash, allowing cache hits for identical builds.
type flagTemplate struct {
	longFlag  string   // e.g., "--tag"
	shortFlag string   // e.g., "-t" (optional, empty if no short form)
	subKeys   []string // e.g., ["builder-id"] for partial templating within the value (optional)
}

// flagsToTemplate defines which flags should have their values replaced with <VALUE>
// (or have specific sub-keys within their values templated) for hash calculation.
// This list is easily extensible - just add new entries for additional flags.
var flagsToTemplate = []flagTemplate{
	// Tags are different between builds but don't affect the image content
	{longFlag: "--tag", shortFlag: "-t"},
	// Output files - purely for writing results, don't affect the build
	{longFlag: "--iidfile"},
	{longFlag: "--metadata-file"},
	// Attestation contains builder-id which has run-specific GitHub Actions URLs
	{longFlag: "--attest", subKeys: []string{"builder-id"}},
	// Output destination flags - where to put the image, not what's in it
	{longFlag: "--cache-to"},
	// Builder selection - infrastructure choice, not build content
	{longFlag: "--builder"},
	// Display format - purely cosmetic
	{longFlag: "--progress"},
	// labels
	{longFlag: "--label"},
	// secrets
	{longFlag: "--secret", subKeys: []string{"src", "env"}},
}

// flagsToDiscard defines boolean flags that should be completely removed before
// hash calculation. These flags don't take values and don't affect the image content.
var flagsToDiscard = []flagTemplate{
	// Display/logging flags - purely cosmetic, no values
	{longFlag: "--quiet", shortFlag: "-q"},
	{longFlag: "--debug", shortFlag: "-D"},
}

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
	booleanFlags := []string{
		"--check", "-D", "--debug", "--load", "--no-cache", "--pull", "--push", "-q", "--quiet",
	}

	var previousArgument string

	// skip docker build/docker buildx build args
	hasBuildx := slices.Contains(dockerBuildArgs, "buildx")
	firstIndex := 2
	if hasBuildx {
		firstIndex = 3
	}

	for i := firstIndex; i < len(dockerBuildArgs); i++ {
		arg := dockerBuildArgs[i]

		// if the argument is a boolean flag, skip it
		if slices.Contains(booleanFlags, arg) {
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

// templateSubKeys replaces specific sub-key values within a flag value.
// For example, for "--attest type=provenance,builder-id=https://..." with subKeys=["builder-id"],
// it returns "type=provenance,builder-id=<VALUE>"
func templateSubKeys(value string, subKeys []string) string {
	result := value
	for _, subKey := range subKeys {
		prefix := subKey + "="
		searchStart := 0
		// Find all occurrences of the subKey in the value
		for searchStart < len(result) {
			idx := strings.Index(result[searchStart:], prefix)
			if idx == -1 {
				break
			}
			idx += searchStart // adjust for the offset
			// Find the end of this sub-key's value (next comma or end of string)
			startOfValue := idx + len(prefix)
			endOfValue := len(result)
			for j := startOfValue; j < len(result); j++ {
				if result[j] == ',' {
					endOfValue = j
					break
				}
			}
			// Replace the value with <VALUE>
			result = result[:startOfValue] + "<VALUE>" + result[endOfValue:]
			// Move search position past the replacement to avoid infinite loop
			searchStart = startOfValue + len("<VALUE>")
		}
	}
	return result
}

// templateLabelValue extracts the key from a label value and templates only the value part.
// For example, for "--label version=1.2.3", it returns "version=<VALUE>"
// This ensures that label keys affect the cache, but label values do not.
func templateLabelValue(value string) string {
	// Find the first '=' which separates key from value
	idx := strings.Index(value, "=")
	if idx == -1 {
		// No '=' found, return as-is (edge case)
		return value
	}
	// Keep the key and '=', template the value
	return value[:idx+1] + "<VALUE>"
}

// normalizeCommandForHashing processes a docker build command to create a normalized
// version suitable for consistent hash calculation. It:
// 1. Discards boolean flags defined in flagsToDiscard (they don't affect image content)
// 2. Templates flag values defined in flagsToTemplate (replacing with <VALUE>)
// 3. Sorts the resulting arguments to ensure order independence
func normalizeCommandForHashing(dockerBuildCmd []string) []string {
	var normalized []string

	for i := 0; i < len(dockerBuildCmd); i++ {
		arg := dockerBuildCmd[i]
		handled := false

		// Check if this is a boolean flag to discard entirely
		for _, ft := range flagsToDiscard {
			if arg == ft.longFlag || (ft.shortFlag != "" && arg == ft.shortFlag) {
				handled = true
				break
			}
		}
		if handled {
			continue
		}

		for _, ft := range flagsToTemplate {
			// Special handling for --label: keep key, template value
			if ft.longFlag == "--label" {
				// Check for space-separated format: --label key=value
				if arg == "--label" {
					normalized = append(normalized, arg)
					if i+1 < len(dockerBuildCmd) {
						i++
						normalized = append(normalized, templateLabelValue(dockerBuildCmd[i]))
					}
					handled = true
					break
				}
				// Check for equals format: --label=key=value
				if strings.HasPrefix(arg, "--label=") {
					value := arg[len("--label="):]
					normalized = append(normalized, "--label="+templateLabelValue(value))
					handled = true
					break
				}
			}

			// Check for space-separated format: --flag value or -f value
			if arg == ft.longFlag || (ft.shortFlag != "" && arg == ft.shortFlag) {
				if len(ft.subKeys) > 0 && i+1 < len(dockerBuildCmd) {
					// Partial templating: keep flag, template only sub-keys in value
					normalized = append(normalized, arg)
					i++
					normalized = append(normalized, templateSubKeys(dockerBuildCmd[i], ft.subKeys))
				} else {
					// Full templating: replace entire value with <VALUE>
					normalized = append(normalized, arg)
					if i+1 < len(dockerBuildCmd) {
						i++
						normalized = append(normalized, "<VALUE>")
					}
				}
				handled = true
				break
			}

			// Check for equals format: --flag=value or -f=value
			longPrefix := ft.longFlag + "="
			shortPrefix := ""
			if ft.shortFlag != "" {
				shortPrefix = ft.shortFlag + "="
			}

			if strings.HasPrefix(arg, longPrefix) {
				if len(ft.subKeys) > 0 {
					// Partial templating: template only sub-keys
					value := arg[len(longPrefix):]
					normalized = append(normalized, longPrefix+templateSubKeys(value, ft.subKeys))
				} else {
					// Full templating
					normalized = append(normalized, longPrefix+"<VALUE>")
				}
				handled = true
				break
			}

			if shortPrefix != "" && strings.HasPrefix(arg, shortPrefix) {
				if len(ft.subKeys) > 0 {
					// Partial templating: template only sub-keys
					value := arg[len(shortPrefix):]
					normalized = append(normalized, shortPrefix+templateSubKeys(value, ft.subKeys))
				} else {
					// Full templating
					normalized = append(normalized, shortPrefix+"<VALUE>")
				}
				handled = true
				break
			}
		}

		if !handled {
			normalized = append(normalized, arg)
		}
	}

	// Sort arguments (excluding the command prefix like "docker build" or "docker buildx build")
	// to ensure order independence, while keeping flag-value pairs together
	prefixLen := 2 // "docker build"
	if len(normalized) > 2 && normalized[1] == "buildx" {
		prefixLen = 3 // "docker buildx build"
	}

	if len(normalized) > prefixLen {
		argsToSort := normalized[prefixLen:]
		// Group flag-value pairs together before sorting
		groups := groupFlagValuePairs(argsToSort)
		// Sort groups by their first element, then by second element if first elements are equal
		slices.SortFunc(groups, func(a, b []string) int {
			if len(a) == 0 && len(b) == 0 {
				return 0
			}
			if len(a) == 0 {
				return -1
			}
			if len(b) == 0 {
				return 1
			}
			// Compare first elements
			if a[0] < b[0] {
				return -1
			}
			if a[0] > b[0] {
				return 1
			}
			// First elements are equal, compare second elements if they exist
			if len(a) > 1 && len(b) > 1 {
				if a[1] < b[1] {
					return -1
				}
				if a[1] > b[1] {
					return 1
				}
			}
			return 0
		})
		// Flatten groups back into a single slice
		argsToSort = []string{}
		for _, group := range groups {
			argsToSort = append(argsToSort, group...)
		}
		normalized = append(normalized[:prefixLen], argsToSort...)
	}

	return normalized
}

// groupFlagValuePairs groups flags with their values together.
// Flags that take values (from flagsToTemplate) are grouped with their following value.
func groupFlagValuePairs(args []string) [][]string {
	var groups [][]string
	flagsWithValues := make(map[string]bool)

	// Build a set of flags that take values
	for _, ft := range flagsToTemplate {
		flagsWithValues[ft.longFlag] = true
		if ft.shortFlag != "" {
			flagsWithValues[ft.shortFlag] = true
		}
	}

	i := 0
	for i < len(args) {
		arg := args[i]
		// Check if this is a flag that takes a value
		if flagsWithValues[arg] && i+1 < len(args) {
			// Group flag with its value
			groups = append(groups, []string{arg, args[i+1]})
			i += 2
		} else {
			// Standalone argument
			groups = append(groups, []string{arg})
			i++
		}
	}

	return groups
}

// buildCommandWithoutTagArguments is kept for backward compatibility but now calls
// the more general normalizeCommandForHashing function.
func buildCommandWithoutTagArguments(dockerBuildCmd []string) []string {
	return normalizeCommandForHashing(dockerBuildCmd)
}

func ParseBuildCommand(dockerBuildCmd []string) (parsedCommand configuration.ParsedCommand, err error) {
	slog.Debug("Parsing command", "command", dockerBuildCmd)
	parsedCommand.Command = dockerBuildCmd

	if len(dockerBuildCmd) < 2 {
		return parsedCommand, fmt.Errorf("not enough arguments for a docker build command")
	}

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

	allTags, allBuildContexts, relativeDockerfilePath, err := extractBuildFlags(args)

	if err != nil {
		return parsedCommand, err
	}

	relativeContextPath, err := findContextPath(dockerBuildCmd)
	if err != nil {
		return parsedCommand, err
	}

	// Get absolute path for contextPath
	absoluteContextPath, err := filepath.Abs(relativeContextPath)
	if err != nil {
		return parsedCommand, err
	}

	allRegistryDomains := []string{}
	for _, tag := range allTags {
		allRegistryDomains = append(allRegistryDomains, argparse.ExtractRegistryDomain(tag))
	}

	cwd, err := os.Getwd()

	if err != nil {
		return parsedCommand, err
	}

	absoluteDockerfilePath := fileresolution.ResolveAbsoluteDockerfilePath(cwd, relativeDockerfilePath)
	dockerignorePath := fileresolution.ResolveAbsoluteDockerIgnorePath(absoluteContextPath, relativeDockerfilePath)

	// add the context in all the build contexts:
	allBuildContexts[configuration.MainBuildContextName] = absoluteContextPath

	parsedCommand.Hash = hasher.HashBuildCommand(hasher.DockerBuildCommand{
		DockerfilePath:         absoluteDockerfilePath,
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
