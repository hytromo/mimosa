package actions

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	actioner := New()
	assert.NotNil(t, actioner)
	assert.IsType(t, &Actioner{}, actioner)
}

func TestActionerImplementsActionsInterface(t *testing.T) {
	// This test ensures that Actioner implements the Actions interface
	var _ Actions = &Actioner{}
}
