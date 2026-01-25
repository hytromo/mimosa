package configuration

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommandContainer_Interface(t *testing.T) {
	// Test that RememberSubcommandOptions
	// properly implement the CommandContainer interface

	var container CommandContainer

	// Test with RememberSubcommandOptions
	rememberOptions := RememberSubcommandOptions{
		Enabled:      true,
		CommandToRun: []string{"docker", "build"},
		DryRun:       false,
	}
	container = rememberOptions
	result := container.GetCommandToRun()
	assert.Equal(t, []string{"docker", "build"}, result)
}
