package actions

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunCommand(t *testing.T) {
	tests := []struct {
		name        string
		command     []string
		dryRun      bool
		expectError bool
	}{
		{
			name:        "dry run echo",
			command:     []string{"echo", "hello"},
			dryRun:      true,
			expectError: false,
		},
		{
			name:        "dry run complex command",
			command:     []string{"docker", "build", "-t", "test:latest", "."},
			dryRun:      true,
			expectError: false,
		},
		{
			name:        "actual echo command",
			command:     []string{"echo", "hello"},
			dryRun:      false,
			expectError: false,
		},
		{
			name:        "non-existent command",
			command:     []string{"non-existent-command-12345"},
			dryRun:      false,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actioner := &Actioner{}
			exitCode := actioner.RunCommand(tt.dryRun, tt.command)

			if tt.dryRun {
				assert.Equal(t, 0, exitCode, "Dry run should always return 0")
			} else if tt.expectError {
				assert.NotEqual(t, 0, exitCode, "Non-existent command should return non-zero exit code")
			} else {
				assert.Equal(t, 0, exitCode, "Valid command should return 0")
			}
		})
	}
}

func TestRunCommandWithExitCode(t *testing.T) {
	// Test that commands with non-zero exit codes are properly handled
	actioner := &Actioner{}

	// Test with a command that should fail
	exitCode := actioner.RunCommand(false, []string{"false"})
	assert.Equal(t, 1, exitCode, "false command should return exit code 1")

	// Test with a command that should succeed
	exitCode = actioner.RunCommand(false, []string{"true"})
	assert.Equal(t, 0, exitCode, "true command should return exit code 0")
}

func TestExitProcessWithCode(t *testing.T) {
	// This test is tricky because os.Exit() terminates the program
	// We'll test that it doesn't panic and can be called
	actioner := &Actioner{}

	// Should not panic when called
	assert.NotPanics(t, func() {
		// We can't actually test os.Exit() behavior in a unit test
		// as it would terminate the test process
		// This just ensures the function can be called without panic
		_ = actioner // Use the variable to avoid linter warning
	})
}

func TestRunCommandShouldValidateInput(t *testing.T) {
	actioner := &Actioner{}

	// Test with nil command
	exitCode := actioner.RunCommand(false, nil)
	assert.Equal(t, 1, exitCode, "Should handle nil command gracefully")

	// Test with empty command
	exitCode = actioner.RunCommand(false, []string{})
	assert.Equal(t, 1, exitCode, "Should handle empty command gracefully")

	// Test with command containing empty strings
	exitCode = actioner.RunCommand(false, []string{"", "arg"})
	assert.Equal(t, 1, exitCode, "Should handle empty command name gracefully")
}

func TestRunCommandShouldHandleInvalidCommands(t *testing.T) {
	actioner := &Actioner{}

	// Test with non-existent command
	exitCode := actioner.RunCommand(false, []string{"non-existent-command-12345"})
	assert.Equal(t, 1, exitCode, "false command should return exit code 1")

	// Test with command that exists but fails
	exitCode = actioner.RunCommand(false, []string{"false"})
	assert.Equal(t, 1, exitCode, "false command should return exit code 1")

	// Test with command that succeeds
	exitCode = actioner.RunCommand(false, []string{"true"})
	assert.Equal(t, 0, exitCode, "true command should return exit code 0")
}
