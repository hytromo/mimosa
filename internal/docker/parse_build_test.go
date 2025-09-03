package docker

import (
	"os"
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
			defer os.Chdir(originalWd)
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

func TestBuildCmdWithoutTagArguments(t *testing.T) {
	testCases := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "Simple tag replacement",
			input:    []string{"docker", "build", "-t", "myapp:latest", "."},
			expected: []string{"docker", "build", "."},
		},
		{
			name:     "Multiple tags - replace first",
			input:    []string{"docker", "build", "-t", "myapp:latest", "-t", "myapp:v1.0.0", "."},
			expected: []string{"docker", "build", "."},
		},
		{
			name:     "Tag with equals syntax",
			input:    []string{"docker", "build", "--tag=myapp:latest", "."},
			expected: []string{"docker", "build", "."},
		},
		{
			name:     "Short tag with equals syntax",
			input:    []string{"docker", "build", "-t=myapp:latest", "."},
			expected: []string{"docker", "build", "."},
		},
		{
			name:     "No tag in command",
			input:    []string{"docker", "build", "."},
			expected: []string{"docker", "build", "."},
		},
		{
			name:     "Complex command with tag",
			input:    []string{"docker", "build", "-f", "Dockerfile.prod", "--no-cache", "-t", "myapp:latest", "."},
			expected: []string{"docker", "build", "-f", "Dockerfile.prod", "--no-cache", "."},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := buildCommandWithoutTagArguments(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestParseBuildCommand_DockerignoreHandling(t *testing.T) {
	tempDir := t.TempDir()

	originalWd, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(originalWd)
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
	defer os.Chdir(originalWd)
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
