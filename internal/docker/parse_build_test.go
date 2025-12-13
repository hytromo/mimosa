package docker

import (
	"os"
	"strings"
	"testing"

	"github.com/hytromo/mimosa/internal/configuration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseBuildCommand_ValidCommand(t *testing.T) {
	testCases := []struct {
		name     string
		command  []string
		expected configuration.ParsedCommand
	}{
		{
			name:    "Simple build command",
			command: []string{"docker", "build", "-t", "myapp:latest", "."},
			expected: configuration.ParsedCommand{
				Command: []string{"docker", "build", "-t", "myapp:latest", "."},
				TagsByTarget: map[string][]string{
					"default": {"myapp:latest"},
				},
			},
		},
		{
			name:    "Build command with multiple tags",
			command: []string{"docker", "build", "-t", "myapp:latest", "-t", "myapp:v1.0.0", "."},
			expected: configuration.ParsedCommand{
				Command: []string{"docker", "build", "-t", "myapp:latest", "-t", "myapp:v1.0.0", "."},
				TagsByTarget: map[string][]string{
					"default": {"myapp:latest", "myapp:v1.0.0"},
				},
			},
		},
		{
			name:    "Build command with custom Dockerfile",
			command: []string{"docker", "build", "-f", "Dockerfile.prod", "-t", "myapp:latest", "."},
			expected: configuration.ParsedCommand{
				Command: []string{"docker", "build", "-f", "Dockerfile.prod", "-t", "myapp:latest", "."},
				TagsByTarget: map[string][]string{
					"default": {"myapp:latest"},
				},
			},
		},
		{
			name:    "Buildx command",
			command: []string{"docker", "buildx", "build", "-t", "myapp:latest", "."},
			expected: configuration.ParsedCommand{
				Command: []string{"docker", "buildx", "build", "-t", "myapp:latest", "."},
				TagsByTarget: map[string][]string{
					"default": {"myapp:latest"},
				},
			},
		},
		{
			name:    "Build command with build context",
			command: []string{"docker", "build", "--build-context", "backend", "./backend", "-t", "myapp:latest", "."},
			expected: configuration.ParsedCommand{
				Command: []string{"docker", "build", "--build-context", "backend", "./backend", "-t", "myapp:latest", "."},
				TagsByTarget: map[string][]string{
					"default": {"myapp:latest"},
				},
			},
		},
		{
			name:    "Build command with equals syntax",
			command: []string{"docker", "build", "--tag=myapp:latest", "--file=Dockerfile.prod", "."},
			expected: configuration.ParsedCommand{
				Command: []string{"docker", "build", "--tag=myapp:latest", "--file=Dockerfile.prod", "."},
				TagsByTarget: map[string][]string{
					"default": {"myapp:latest"},
				},
			},
		},
		{
			name:    "Build command with short flags equals syntax",
			command: []string{"docker", "build", "-t=myapp:latest", "-f=Dockerfile.prod", "."},
			expected: configuration.ParsedCommand{
				Command: []string{"docker", "build", "-t=myapp:latest", "-f=Dockerfile.prod", "."},
				TagsByTarget: map[string][]string{
					"default": {"myapp:latest"},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()

			originalWd, err := os.Getwd()
			require.NoError(t, err)
			defer func() { _ = os.Chdir(originalWd) }()
			err = os.Chdir(tempDir)
			require.NoError(t, err)

			dockerfilePath := "Dockerfile"
			if tc.name == "Build command with custom Dockerfile" || tc.name == "Build command with equals syntax" || tc.name == "Build command with short flags equals syntax" {
				dockerfilePath = "Dockerfile.prod"
			}
			err = os.WriteFile(dockerfilePath, []byte("FROM alpine:latest"), 0644)
			require.NoError(t, err)

			result, err := ParseBuildCommand(tc.command)
			require.NoError(t, err)

			assert.Equal(t, tc.expected.Command, result.Command)
			assert.Equal(t, tc.expected.TagsByTarget, result.TagsByTarget)
			assert.NotEmpty(t, result.Hash)
		})
	}
}

func TestParseBuildCommand_InvalidCommands(t *testing.T) {
	testCases := []struct {
		name        string
		command     []string
		expectedErr string
	}{
		{
			name:        "Empty command",
			command:     []string{},
			expectedErr: "not enough arguments for a docker build command",
		},
		{
			name:        "Single argument",
			command:     []string{"docker"},
			expectedErr: "not enough arguments for a docker build command",
		},
		{
			name:        "Wrong executable",
			command:     []string{"podman", "build", "-t", "myapp:latest", "."},
			expectedErr: "only 'docker' executable is supported for caching",
		},
		{
			name:        "Wrong subcommand",
			command:     []string{"docker", "run", "-t", "myapp:latest", "."},
			expectedErr: "only image building is supported",
		},
		{
			name:        "No tag specified",
			command:     []string{"docker", "build", "."},
			expectedErr: "cannot find image tag using the -t or --tag option",
		},
		{
			name:        "No context path",
			command:     []string{"docker", "build", "-t", "myapp:latest"},
			expectedErr: "context path not found",
		},
		{
			name:        "No context path - buildx",
			command:     []string{"docker", "buildx", "build", "-t", "myapp:latest"},
			expectedErr: "context path not found",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseBuildCommand(tc.command)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.expectedErr)
		})
	}
}

func TestExtractBuildFlags(t *testing.T) {
	testCases := []struct {
		name                   string
		args                   []string
		expectedTags           []string
		expectedBuildContexts  map[string]string
		expectedDockerfilePath string
		expectError            bool
		expectedErrorContains  string
	}{
		{
			name:                   "Simple tags",
			args:                   []string{"build", "-t", "myapp:latest", "."},
			expectedTags:           []string{"myapp:latest"},
			expectedBuildContexts:  map[string]string{},
			expectedDockerfilePath: "",
		},
		{
			name:                   "Multiple tags",
			args:                   []string{"build", "-t", "myapp:latest", "-t", "myapp:v1.0.0", "."},
			expectedTags:           []string{"myapp:latest", "myapp:v1.0.0"},
			expectedBuildContexts:  map[string]string{},
			expectedDockerfilePath: "",
		},
		{
			name:                   "Tags with equals syntax",
			args:                   []string{"build", "--tag=myapp:latest", "--tag=myapp:v1.0.0", "."},
			expectedTags:           []string{"myapp:latest", "myapp:v1.0.0"},
			expectedBuildContexts:  map[string]string{},
			expectedDockerfilePath: "",
		},
		{
			name:                   "Short tag with equals syntax",
			args:                   []string{"build", "-t=myapp:latest", "."},
			expectedTags:           []string{"myapp:latest"},
			expectedBuildContexts:  map[string]string{},
			expectedDockerfilePath: "",
		},
		{
			name:                   "With Dockerfile path",
			args:                   []string{"build", "-f", "Dockerfile.prod", "-t", "myapp:latest", "."},
			expectedTags:           []string{"myapp:latest"},
			expectedBuildContexts:  map[string]string{},
			expectedDockerfilePath: "Dockerfile.prod",
		},
		{
			name:                   "Dockerfile with equals syntax",
			args:                   []string{"build", "--file=Dockerfile.prod", "-t", "myapp:latest", "."},
			expectedTags:           []string{"myapp:latest"},
			expectedBuildContexts:  map[string]string{},
			expectedDockerfilePath: "Dockerfile.prod",
		},
		{
			name:                   "Short Dockerfile with equals syntax",
			args:                   []string{"build", "-f=Dockerfile.prod", "-t", "myapp:latest", "."},
			expectedTags:           []string{"myapp:latest"},
			expectedBuildContexts:  map[string]string{},
			expectedDockerfilePath: "Dockerfile.prod",
		},
		{
			name:                   "With build context",
			args:                   []string{"build", "--build-context", "backend=./backend", "-t", "myapp:latest", "."},
			expectedTags:           []string{"myapp:latest"},
			expectedBuildContexts:  map[string]string{"backend": "./backend"},
			expectedDockerfilePath: "",
		},
		{
			name:                   "No tags",
			args:                   []string{"build", "."},
			expectedTags:           nil,
			expectedBuildContexts:  map[string]string{},
			expectedDockerfilePath: "",
			expectError:            true,
			expectedErrorContains:  "cannot find image tag",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tags, buildContexts, dockerfilePath, err := extractBuildFlags(tc.args)

			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrorContains)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedTags, tags)
				assert.Equal(t, tc.expectedBuildContexts, buildContexts)
				assert.Equal(t, tc.expectedDockerfilePath, dockerfilePath)
			}
		})
	}
}

func TestFindContextPath(t *testing.T) {
	testCases := []struct {
		name          string
		args          []string
		expectedPath  string
		expectError   bool
		expectedError string
	}{
		{
			name:         "Simple context path",
			args:         []string{"docker", "build", "-t", "myapp:latest", "."},
			expectedPath: ".",
		},
		{
			name:         "Context path with directory",
			args:         []string{"docker", "build", "-t", "myapp:latest", "./src"},
			expectedPath: "./src",
		},
		{
			name:         "Context path with absolute path",
			args:         []string{"docker", "build", "-t", "myapp:latest", "/tmp/build"},
			expectedPath: "/tmp/build",
		},
		{
			name:         "Context path after flags",
			args:         []string{"docker", "build", "-f", "Dockerfile.prod", "-t", "myapp:latest", "."},
			expectedPath: ".",
		},
		{
			name:         "Context path with buildx",
			args:         []string{"docker", "buildx", "build", "-t", "myapp:latest", "."},
			expectedPath: ".",
		},
		{
			name:         "Context path with build context",
			args:         []string{"docker", "build", "--build-context", "backend=./backend", "-t", "myapp:latest", "."},
			expectedPath: ".",
		},
		{
			name:         "Context path with build context after boolean flag",
			args:         []string{"docker", "build", "--build-context", "backend=./backend", "-t", "myapp:latest", "--push", "./docs"},
			expectedPath: "./docs",
		},
		{
			name:          "No context path",
			args:          []string{"docker", "build", "-t", "myapp:latest"},
			expectError:   true,
			expectedError: "context path not found",
		},
		{
			name:          "Only flags",
			args:          []string{"docker", "build", "-t", "myapp:latest", "--no-cache"},
			expectError:   true,
			expectedError: "context path not found",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			path, err := findContextPath(tc.args)

			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedPath, path)
			}
		})
	}
}

func TestNormalizeCommandForHashing(t *testing.T) {
	testCases := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "Simple tag templating",
			input:    []string{"docker", "build", "-t", "myapp:latest", "."},
			expected: []string{"docker", "build", "-t", ".", "<VALUE>"}, // sorted: "-t" < "." < "<VALUE>"
		},
		{
			name:     "Multiple tags templating",
			input:    []string{"docker", "build", "-t", "myapp:latest", "-t", "myapp:v1.0.0", "."},
			expected: []string{"docker", "build", "-t", "-t", ".", "<VALUE>", "<VALUE>"},
		},
		{
			name:     "Tag with equals syntax",
			input:    []string{"docker", "build", "--tag=myapp:latest", "."},
			expected: []string{"docker", "build", "--tag=<VALUE>", "."},
		},
		{
			name:     "Short tag with equals syntax",
			input:    []string{"docker", "build", "-t=myapp:latest", "."},
			expected: []string{"docker", "build", "-t=<VALUE>", "."},
		},
		{
			name:     "No tag in command",
			input:    []string{"docker", "build", "."},
			expected: []string{"docker", "build", "."},
		},
		{
			name:     "iidfile templating",
			input:    []string{"docker", "build", "--iidfile", "/tmp/random123.txt", "-t", "myapp:latest", "."},
			expected: []string{"docker", "build", "--iidfile", "-t", ".", "<VALUE>", "<VALUE>"},
		},
		{
			name:     "iidfile with equals syntax",
			input:    []string{"docker", "build", "--iidfile=/tmp/random.txt", "-t", "myapp:latest", "."},
			expected: []string{"docker", "build", "--iidfile=<VALUE>", "-t", ".", "<VALUE>"},
		},
		{
			name:     "metadata-file templating",
			input:    []string{"docker", "build", "--metadata-file", "/tmp/metadata.json", "-t", "myapp:latest", "."},
			expected: []string{"docker", "build", "--metadata-file", "-t", ".", "<VALUE>", "<VALUE>"},
		},
		{
			name:     "metadata-file with equals syntax",
			input:    []string{"docker", "build", "--metadata-file=/tmp/metadata.json", "-t", "myapp:latest", "."},
			expected: []string{"docker", "build", "--metadata-file=<VALUE>", "-t", ".", "<VALUE>"},
		},
		{
			name:     "attest with builder-id templating",
			input:    []string{"docker", "build", "--attest", "type=provenance,mode=max,builder-id=https://github.com/example/actions/runs/123", "-t", "myapp:latest", "."},
			expected: []string{"docker", "build", "--attest", "-t", ".", "<VALUE>", "type=provenance,mode=max,builder-id=<VALUE>"},
		},
		{
			name:     "attest with equals syntax and builder-id",
			input:    []string{"docker", "build", "--attest=type=provenance,builder-id=https://example.com/run/456", "-t", "myapp:latest", "."},
			expected: []string{"docker", "build", "--attest=type=provenance,builder-id=<VALUE>", "-t", ".", "<VALUE>"},
		},
		{
			name:     "attest without builder-id unchanged",
			input:    []string{"docker", "build", "--attest", "type=sbom,generator=image", "-t", "myapp:latest", "."},
			expected: []string{"docker", "build", "--attest", "-t", ".", "<VALUE>", "type=sbom,generator=image"},
		},
		{
			name:     "buildx command",
			input:    []string{"docker", "buildx", "build", "-t", "myapp:latest", "."},
			expected: []string{"docker", "buildx", "build", "-t", ".", "<VALUE>"},
		},
		{
			name:     "buildx with multiple templated flags",
			input:    []string{"docker", "buildx", "build", "--iidfile", "/tmp/id.txt", "--metadata-file", "/tmp/meta.json", "-t", "myapp:latest", "."},
			expected: []string{"docker", "buildx", "build", "--iidfile", "--metadata-file", "-t", ".", "<VALUE>", "<VALUE>", "<VALUE>"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := normalizeCommandForHashing(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestNormalizeCommandForHashing_OrderIndependence(t *testing.T) {
	// Commands with same flags in different order should produce identical normalized output
	testCases := []struct {
		name   string
		input1 []string
		input2 []string
	}{
		{
			name:   "Simple flag reordering",
			input1: []string{"docker", "build", "-t", "myapp:latest", "--platform", "linux/amd64", "."},
			input2: []string{"docker", "build", "--platform", "linux/amd64", "-t", "myapp:latest", "."},
		},
		{
			name:   "Multiple flags reordering",
			input1: []string{"docker", "build", "-t", "myapp:latest", "--iidfile", "/tmp/a.txt", "--push", "."},
			input2: []string{"docker", "build", "--push", "--iidfile", "/tmp/b.txt", "-t", "other:tag", "."},
		},
		{
			name:   "Buildx with complex reordering",
			input1: []string{"docker", "buildx", "build", "--metadata-file", "/path/1.json", "--platform", "linux/amd64,linux/arm64", "-t", "img:v1", "."},
			input2: []string{"docker", "buildx", "build", "-t", "img:v2", "--platform", "linux/amd64,linux/arm64", "--metadata-file", "/path/2.json", "."},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result1 := normalizeCommandForHashing(tc.input1)
			result2 := normalizeCommandForHashing(tc.input2)
			assert.Equal(t, result1, result2, "Commands with same flags in different order should normalize to the same result")
		})
	}
}

func TestNormalizeCommandForHashing_GitHubActionsExample(t *testing.T) {
	// These are the actual commands from the GitHub Actions issue
	// They should produce identical normalized output
	cmd1 := []string{
		"docker", "buildx", "build",
		"--iidfile", "/home/runner/work/_temp/docker-actions-toolkit-FfxZzf/build-iidfile-beb46e5a7d.txt",
		"--platform", "linux/amd64,linux/arm64",
		"--attest", "type=provenance,mode=max,builder-id=https://github.com/hytromo/mimosa/actions/runs/20193832931/attempts/1",
		"--tag", "hytromo/mimosa-testing:recommended-example-cache-hit-c7ad46653a914718bf8e31f484f69614552e92e8",
		"--metadata-file", "/home/runner/work/_temp/docker-actions-toolkit-FfxZzf/build-metadata-98d5a7c1b3.json",
		"--push",
		"docs/gh-actions/actions-example",
	}

	cmd2 := []string{
		"docker", "buildx", "build",
		"--iidfile", "/home/runner/work/_temp/docker-actions-toolkit-BtGuR6/build-iidfile-81bd4a8cf4.txt",
		"--platform", "linux/amd64,linux/arm64",
		"--attest", "type=provenance,mode=max,builder-id=https://github.com/hytromo/mimosa/actions/runs/20193832931/attempts/1",
		"--tag", "hytromo/mimosa-testing:recommended-example-c7ad46653a914718bf8e31f484f69614552e92e8",
		"--metadata-file", "/home/runner/work/_temp/docker-actions-toolkit-BtGuR6/build-metadata-7bb018a1cf.json",
		"--push",
		"docs/gh-actions/actions-example",
	}

	result1 := normalizeCommandForHashing(cmd1)
	result2 := normalizeCommandForHashing(cmd2)

	assert.Equal(t, result1, result2, "GitHub Actions example commands should normalize to the same result")

	// Verify the normalized output contains expected flags
	normalizedStr := strings.Join(result1, " ")
	assert.Contains(t, normalizedStr, "--iidfile")
	assert.Contains(t, normalizedStr, "<VALUE>")
	assert.Contains(t, normalizedStr, "--attest")
	assert.Contains(t, normalizedStr, "type=provenance,mode=max,builder-id=<VALUE>")
}

func TestTemplateSubKeys(t *testing.T) {
	testCases := []struct {
		name     string
		value    string
		subKeys  []string
		expected string
	}{
		{
			name:     "Template builder-id at end",
			value:    "type=provenance,mode=max,builder-id=https://github.com/example/runs/123",
			subKeys:  []string{"builder-id"},
			expected: "type=provenance,mode=max,builder-id=<VALUE>",
		},
		{
			name:     "Template builder-id in middle",
			value:    "type=provenance,builder-id=https://example.com,mode=max",
			subKeys:  []string{"builder-id"},
			expected: "type=provenance,builder-id=<VALUE>,mode=max",
		},
		{
			name:     "Template builder-id at start",
			value:    "builder-id=https://example.com,type=provenance",
			subKeys:  []string{"builder-id"},
			expected: "builder-id=<VALUE>,type=provenance",
		},
		{
			name:     "No matching subkey",
			value:    "type=provenance,mode=max",
			subKeys:  []string{"builder-id"},
			expected: "type=provenance,mode=max",
		},
		{
			name:     "Multiple subkeys",
			value:    "type=provenance,builder-id=url1,secret=abc123",
			subKeys:  []string{"builder-id", "secret"},
			expected: "type=provenance,builder-id=<VALUE>,secret=<VALUE>",
		},
		{
			name:     "Empty value",
			value:    "",
			subKeys:  []string{"builder-id"},
			expected: "",
		},
		{
			name:     "Empty subkeys",
			value:    "type=provenance,builder-id=url",
			subKeys:  []string{},
			expected: "type=provenance,builder-id=url",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := templateSubKeys(tc.value, tc.subKeys)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestBuildCmdWithoutTagArguments tests backward compatibility
func TestBuildCmdWithoutTagArguments(t *testing.T) {
	// This function should delegate to normalizeCommandForHashing
	input := []string{"docker", "build", "-t", "myapp:latest", "."}
	result := buildCommandWithoutTagArguments(input)
	expected := normalizeCommandForHashing(input)
	assert.Equal(t, expected, result)
}

func TestParseBuildCommand_DockerignoreHandling(t *testing.T) {
	tempDir := t.TempDir()

	originalWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalWd) }()
	err = os.Chdir(tempDir)
	require.NoError(t, err)

	err = os.WriteFile("Dockerfile", []byte("FROM alpine:latest"), 0644)
	require.NoError(t, err)

	err = os.WriteFile(".dockerignore", []byte("*.log\nnode_modules"), 0644)
	require.NoError(t, err)

	command := []string{"docker", "build", "-t", "myapp:latest", "."}

	result, err := ParseBuildCommand(command)
	require.NoError(t, err)

	assert.Equal(t, command, result.Command)
	assert.NotEmpty(t, result.Hash)
}

func TestParseBuildCommand_CustomDockerignoreHandling(t *testing.T) {
	tempDir := t.TempDir()

	originalWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalWd) }()
	err = os.Chdir(tempDir)
	require.NoError(t, err)

	err = os.WriteFile("Dockerfile", []byte("FROM alpine:latest"), 0644)
	require.NoError(t, err)

	err = os.WriteFile("Dockerfile.prod", []byte("FROM alpine:latest"), 0644)
	require.NoError(t, err)
	err = os.WriteFile("Dockerfile.prod.dockerignore", []byte("*.tmp"), 0644)
	require.NoError(t, err)

	command := []string{"docker", "build", "-f", "Dockerfile.prod", "-t", "myapp:latest", "."}

	result, err := ParseBuildCommand(command)
	require.NoError(t, err)

	assert.Equal(t, command, result.Command)
	assert.NotEmpty(t, result.Hash)
}
