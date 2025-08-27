package actions

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseCommand(t *testing.T) {
	tests := []struct {
		name        string
		command     []string
		expectError bool
	}{
		{
			name:        "empty command",
			command:     []string{},
			expectError: true,
		},
		{
			name:        "single element",
			command:     []string{"docker"},
			expectError: true,
		},
		{
			name:        "two elements",
			command:     []string{"docker", "build"},
			expectError: true,
		},
		{
			name:        "not docker command",
			command:     []string{"echo", "hello", "world"},
			expectError: true,
		},
		{
			name:        "docker build valid",
			command:     []string{"docker", "build", "."},
			expectError: true, // Will fail because no tag is provided
		},
		{
			name:        "docker build with context and tag",
			command:     []string{"docker", "build", "-t", "myimage:latest", "."},
			expectError: false,
		},
		{
			name:        "docker buildx without subcommand",
			command:     []string{"docker", "buildx"},
			expectError: true,
		},
		{
			name:        "docker buildx with only build",
			command:     []string{"docker", "buildx", "build"},
			expectError: true,
		},
		{
			name:        "docker buildx build valid",
			command:     []string{"docker", "buildx", "build", "."},
			expectError: true, // Will fail because no tag is provided
		},
		{
			name:        "docker buildx bake valid",
			command:     []string{"docker", "buildx", "bake", "myfile"},
			expectError: true, // Will fail because no bake file exists, but command should be preserved
		},
		{
			name:        "docker buildx invalid subcommand",
			command:     []string{"docker", "buildx", "invalid", "."},
			expectError: true,
		},
		{
			name:        "docker invalid subcommand",
			command:     []string{"docker", "invalid", "."},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actioner := &Actioner{}
			result, err := actioner.ParseCommand(tt.command)

			if tt.expectError {
				assert.Error(t, err)
				// Should still return the command in the result for most cases
				if len(result.Command) > 0 {
					assert.Equal(t, tt.command, result.Command)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.command, result.Command)
			}
		})
	}
}

func TestParseCommandShouldPreserveCommandOnError(t *testing.T) {
	actioner := &Actioner{}

	testCases := []struct {
		name        string
		command     []string
		expectError bool
	}{
		{
			name:        "docker buildx bake with missing file",
			command:     []string{"docker", "buildx", "bake", "missing-file"},
			expectError: true,
		},
		{
			name:        "docker build without tag",
			command:     []string{"docker", "build", "."},
			expectError: true,
		},
		{
			name:        "docker buildx build without tag",
			command:     []string{"docker", "buildx", "build", "."},
			expectError: true,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			result, err := actioner.ParseCommand(tt.command)

			if tt.expectError {
				assert.Error(t, err, "Should return error for: %v", tt.command)
				assert.Equal(t, tt.command, result.Command, "Original command should be preserved even on error: %v", tt.command)
			}
		})
	}
}

func TestParseCommandShouldValidateInput(t *testing.T) {
	actioner := &Actioner{}

	result, err := actioner.ParseCommand(nil)
	assert.Error(t, err, "Should return error for nil command")
	assert.Nil(t, result.Command, "Command should be nil for nil input")

	result, err = actioner.ParseCommand([]string{})
	assert.Error(t, err, "Should return error for empty command")
	assert.Equal(t, []string{}, result.Command, "Command should be empty for empty input")
}
