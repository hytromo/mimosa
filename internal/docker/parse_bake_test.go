package docker

import (
	"os"
	"testing"

	"github.com/hytromo/mimosa/internal/configuration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseBakeCommand_ValidCommand(t *testing.T) {
	testCases := []struct {
		name           string
		command        []string
		expected       configuration.ParsedCommand
		bakeFileTarget string
	}{
		{
			name:           "Simple bake command",
			command:        []string{"docker", "bake"},
			bakeFileTarget: "default",
			expected: configuration.ParsedCommand{
				Command: []string{"docker", "bake"},
				TagsByTarget: map[string][]string{
					"default": nil,
				},
			},
		},
		{
			name:           "Bake command with target",
			command:        []string{"docker", "bake", "app"},
			bakeFileTarget: "app,db",
			expected: configuration.ParsedCommand{
				Command: []string{"docker", "bake", "app"},
				TagsByTarget: map[string][]string{
					"app": nil,
				},
			},
		},
		{
			name:           "Bake command with multiple targets",
			command:        []string{"docker", "bake", "app", "db"},
			bakeFileTarget: "app,db",
			expected: configuration.ParsedCommand{
				Command: []string{"docker", "bake", "app", "db"},
				TagsByTarget: map[string][]string{
					"app": nil,
					"db":  nil,
				},
			},
		},
		{
			name:           "Bake command with file",
			command:        []string{"docker", "bake", "-f", "docker-bake.json"},
			bakeFileTarget: "default",
			expected: configuration.ParsedCommand{
				Command: []string{"docker", "bake", "-f", "docker-bake.json"},
				TagsByTarget: map[string][]string{
					"default": nil,
				},
			},
		},
		{
			name:           "Bake command with file equals syntax",
			command:        []string{"docker", "bake", "--file=docker-bake.json"},
			bakeFileTarget: "default",
			expected: configuration.ParsedCommand{
				Command: []string{"docker", "bake", "--file=docker-bake.json"},
				TagsByTarget: map[string][]string{
					"default": nil,
				},
			},
		},
		{
			name:           "Bake command with set override",
			command:        []string{"docker", "bake", "--set", "*.platform=linux/amd64"},
			bakeFileTarget: "default",
			expected: configuration.ParsedCommand{
				Command: []string{"docker", "bake", "--set", "*.platform=linux/amd64"},
				TagsByTarget: map[string][]string{
					"default": nil,
				},
			},
		},
		{
			name:           "Bake command with set equals syntax",
			command:        []string{"docker", "bake", "--set=*.platform=linux/amd64"},
			bakeFileTarget: "default",
			expected: configuration.ParsedCommand{
				Command: []string{"docker", "bake", "--set=*.platform=linux/amd64"},
				TagsByTarget: map[string][]string{
					"default": nil,
				},
			},
		},
		{
			name:           "Complex bake command",
			command:        []string{"docker", "bake", "-f", "docker-bake.json", "--set", "*.platform=linux/amd64", "app", "db"},
			bakeFileTarget: "app,db",
			expected: configuration.ParsedCommand{
				Command: []string{"docker", "bake", "-f", "docker-bake.json", "--set", "*.platform=linux/amd64", "app", "db"},
				TagsByTarget: map[string][]string{
					"app": nil,
					"db":  nil,
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

			var bakeFile string
			if tc.bakeFileTarget == "app,db" {
				bakeFile = `{
						"target": {
							"app": {
								"context": ".",
								"dockerfile": "Dockerfile"
							},
							"db": {
								"context": ".",
								"dockerfile": "Dockerfile.db"
							}
						}
					}`
			} else {
				bakeFile = `{
						"target": {
							"default": {
								"context": ".",
								"dockerfile": "Dockerfile"
							}
						}
					}`
			}
			err = os.WriteFile("docker-bake.json", []byte(bakeFile), 0644)
			require.NoError(t, err)

			// Parse the command
			result, err := ParseBakeCommand(tc.command)
			require.NoError(t, err)

			// Verify basic fields
			assert.Equal(t, tc.expected.Command, result.Command)
			assert.Equal(t, tc.expected.TagsByTarget, result.TagsByTarget)
			assert.NotEmpty(t, result.Hash)
		})
	}
}

func TestParseBakeCommand_InvalidCommands(t *testing.T) {
	testCases := []struct {
		name        string
		command     []string
		expectedErr string
	}{
		{
			name:        "Empty command",
			command:     []string{},
			expectedErr: "failed to extract bake flags",
		},
		{
			name:        "No bake files found",
			command:     []string{"docker", "bake"},
			expectedErr: "no bake files found",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseBakeCommand(tc.command)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectedErr)
		})
	}
}

func TestExtractBakeFlags(t *testing.T) {
	testCases := []struct {
		name              string
		args              []string
		expectedFiles     []string
		expectedTargets   []string
		expectedOverrides []string
	}{
		{
			name:              "Simple bake command",
			args:              []string{"bake"},
			expectedFiles:     []string{},
			expectedTargets:   []string{"default"},
			expectedOverrides: []string{},
		},
		{
			name:              "Bake with target",
			args:              []string{"bake", "app"},
			expectedFiles:     []string{},
			expectedTargets:   []string{"app"},
			expectedOverrides: []string{},
		},
		{
			name:              "Bake with multiple targets",
			args:              []string{"bake", "app", "db"},
			expectedFiles:     []string{},
			expectedTargets:   []string{"app", "db"},
			expectedOverrides: []string{},
		},
		{
			name:              "Bake with file flag",
			args:              []string{"bake", "--file", "docker-bake.json"},
			expectedFiles:     []string{"docker-bake.json"},
			expectedTargets:   []string{"default"},
			expectedOverrides: []string{},
		},
		{
			name:              "Bake with short file flag",
			args:              []string{"bake", "-f", "docker-bake.json"},
			expectedFiles:     []string{"docker-bake.json"},
			expectedTargets:   []string{"default"},
			expectedOverrides: []string{},
		},
		{
			name:              "Bake with file equals syntax",
			args:              []string{"bake", "--file=docker-bake.json"},
			expectedFiles:     []string{"docker-bake.json"},
			expectedTargets:   []string{"default"},
			expectedOverrides: []string{},
		},
		{
			name:              "Bake with short file equals syntax",
			args:              []string{"bake", "-f=docker-bake.json"},
			expectedFiles:     []string{"docker-bake.json"},
			expectedTargets:   []string{"default"},
			expectedOverrides: []string{},
		},
		{
			name:              "Bake with set flag",
			args:              []string{"bake", "--set", "*.platform=linux/amd64"},
			expectedFiles:     []string{},
			expectedTargets:   []string{"default"},
			expectedOverrides: []string{"*.platform=linux/amd64"},
		},
		{
			name:              "Bake with set equals syntax",
			args:              []string{"bake", "--set=*.platform=linux/amd64"},
			expectedFiles:     []string{},
			expectedTargets:   []string{"default"},
			expectedOverrides: []string{"*.platform=linux/amd64"},
		},
		{
			name:              "Bake with multiple sets",
			args:              []string{"bake", "--set", "*.platform=linux/amd64", "--set", "*.push=true"},
			expectedFiles:     []string{},
			expectedTargets:   []string{"default"},
			expectedOverrides: []string{"*.platform=linux/amd64", "*.push=true"},
		},
		{
			name:              "Complex bake command",
			args:              []string{"bake", "-f", "docker-bake.json", "--set", "*.platform=linux/amd64", "app", "db"},
			expectedFiles:     []string{"docker-bake.json"},
			expectedTargets:   []string{"app", "db"},
			expectedOverrides: []string{"*.platform=linux/amd64"},
		},
		{
			name:              "Bake with flags after targets",
			args:              []string{"bake", "app", "--file", "docker-bake.json"},
			expectedFiles:     []string{"docker-bake.json"},
			expectedTargets:   []string{"app"},
			expectedOverrides: []string{},
		},
		{
			name:              "Bake with flags that we do not care about",
			args:              []string{"bake", "app", "--builder", "mybuilder", "--file", "docker-bake.json"},
			expectedFiles:     []string{"docker-bake.json"},
			expectedTargets:   []string{"app"},
			expectedOverrides: []string{},
		},
		{
			name:              "Bake with multiple unknown flags",
			args:              []string{"bake", "--no-cache", "--pull", "--debug", "app", "--file", "docker-bake.json"},
			expectedFiles:     []string{"docker-bake.json"},
			expectedTargets:   []string{"app"},
			expectedOverrides: []string{},
		},
		{
			name:              "Bake with unknown flags with equals syntax",
			args:              []string{"bake", "--progress=plain", "--builder=mybuilder", "app", "db"},
			expectedFiles:     []string{},
			expectedTargets:   []string{"app", "db"},
			expectedOverrides: []string{},
		},
		{
			name:              "Bake with mixed known and unknown flags",
			args:              []string{"bake", "--set", "*.platform=linux/amd64", "--load", "--push", "--file", "docker-bake.json", "app"},
			expectedFiles:     []string{"docker-bake.json"},
			expectedTargets:   []string{"app"},
			expectedOverrides: []string{"*.platform=linux/amd64"},
		},
		{
			name:              "Bake with unknown flags between targets",
			args:              []string{"bake", "app", "--allow", "network=host", "db", "--file", "docker-bake.json"},
			expectedFiles:     []string{"docker-bake.json"},
			expectedTargets:   []string{"app", "db"},
			expectedOverrides: []string{},
		},
		{
			name:              "Bake with shorthand flags that should be ignored",
			args:              []string{"bake", "-D", "--load", "app", "-f", "docker-bake.json"},
			expectedFiles:     []string{"docker-bake.json"},
			expectedTargets:   []string{"app"},
			expectedOverrides: []string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			files, targets, overrides, err := extractBakeFlags(tc.args)
			require.NoError(t, err)

			assert.Equal(t, tc.expectedFiles, files)
			assert.Equal(t, tc.expectedTargets, targets)
			assert.Equal(t, tc.expectedOverrides, overrides)
		})
	}
}

func TestParseBakeCommand_WithRealBakeFile(t *testing.T) {
	tempDir := t.TempDir()

	// Change to temp directory
	originalWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalWd) }()
	err = os.Chdir(tempDir)
	require.NoError(t, err)

	// Create a realistic bake file with tags
	bakeFile := `{
		"target": {
			"app": {
				"context": ".",
				"dockerfile": "Dockerfile",
				"tags": ["myapp:latest", "myapp:v1.0.0"]
			},
			"db": {
				"context": ".",
				"dockerfile": "Dockerfile.db",
				"tags": ["mydb:latest", "mydb:v1.0.0"]
			}
		}
	}`
	err = os.WriteFile("docker-bake.json", []byte(bakeFile), 0644)
	require.NoError(t, err)

	command := []string{"docker", "bake", "app", "db"}

	// Parse the command
	result, err := ParseBakeCommand(command)
	require.NoError(t, err)

	// Verify the command was parsed successfully
	assert.Equal(t, command, result.Command)
	assert.NotEmpty(t, result.Hash)
	assert.Equal(t, map[string][]string{
		"app": {"myapp:latest", "myapp:v1.0.0"},
		"db":  {"mydb:latest", "mydb:v1.0.0"},
	}, result.TagsByTarget)
}

func TestParseBakeCommand_DefaultFileLookup(t *testing.T) {
	testCases := []struct {
		name     string
		filename string
		content  string
	}{
		{
			name:     "docker-compose.yml",
			filename: "docker-compose.yml",
			content: `version: '3.8'
services:
  app:
    build:
      context: .
      dockerfile: Dockerfile
    image: myapp:latest`,
		},
		{
			name:     "docker-bake.json",
			filename: "docker-bake.json",
			content: `{
				"target": {
					"default": {
						"context": ".",
						"dockerfile": "Dockerfile"
					}
				}
			}`,
		},
		{
			name:     "docker-bake.hcl",
			filename: "docker-bake.hcl",
			content: `target "default" {
				context = "."
				dockerfile = "Dockerfile"
			}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()

			// Change to temp directory
			originalWd, err := os.Getwd()
			require.NoError(t, err)
			defer func() {
				_ = os.Chdir(originalWd)
			}()
			err = os.Chdir(tempDir)
			require.NoError(t, err)

			// Create the bake file
			err = os.WriteFile(tc.filename, []byte(tc.content), 0644)
			require.NoError(t, err)

			command := []string{"docker", "bake"}

			// Parse the command
			result, err := ParseBakeCommand(command)
			require.NoError(t, err)

			// Verify the command was parsed successfully
			assert.Equal(t, command, result.Command)
			assert.NotEmpty(t, result.Hash)
		})
	}
}

func TestParseBakeCommand_WithOverrides(t *testing.T) {
	tempDir := t.TempDir()

	// Change to temp directory
	originalWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalWd) }()
	err = os.Chdir(tempDir)
	require.NoError(t, err)

	// Create a bake file
	bakeFile := `{
		"target": {
			"app": {
				"context": ".",
				"dockerfile": "Dockerfile",
				"tags": ["myapp:latest"]
			}
		}
	}`
	err = os.WriteFile("docker-bake.json", []byte(bakeFile), 0644)
	require.NoError(t, err)

	command := []string{"docker", "bake", "--set", "*.tags=myapp:override", "app"}

	// Parse the command
	result, err := ParseBakeCommand(command)
	require.NoError(t, err)

	// Verify the command was parsed successfully
	assert.Equal(t, command, result.Command)
	assert.NotEmpty(t, result.Hash)
	// Note: The actual override behavior depends on the bake library implementation
	// We're mainly testing that the command parses without error
}

func TestParseBakeCommand_ErrorHandling(t *testing.T) {
	testCases := []struct {
		name        string
		bakeFile    string
		command     []string
		expectedErr string
	}{
		{
			name:        "Invalid JSON",
			bakeFile:    `{ invalid json }`,
			command:     []string{"docker", "bake"},
			expectedErr: "failed to parse bake targets",
		},
		{
			name:        "Invalid HCL",
			bakeFile:    `target "default" { invalid hcl }`,
			command:     []string{"docker", "bake"},
			expectedErr: "failed to parse bake targets",
		},
		{
			name:        "Non-existent target",
			bakeFile:    `{"target": {"default": {"context": "."}}}`,
			command:     []string{"docker", "bake", "nonexistent"},
			expectedErr: "failed to parse bake targets",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()

			// Change to temp directory
			originalWd, err := os.Getwd()
			require.NoError(t, err)
			defer func() { _ = os.Chdir(originalWd) }()
			err = os.Chdir(tempDir)
			require.NoError(t, err)

			// Create the bake file
			filename := "docker-bake.json"
			if tc.name == "Invalid HCL" {
				filename = "docker-bake.hcl"
			}
			err = os.WriteFile(filename, []byte(tc.bakeFile), 0644)
			require.NoError(t, err)

			// Parse the command
			_, err = ParseBakeCommand(tc.command)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectedErr)
		})
	}
}
