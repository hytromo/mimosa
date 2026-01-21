package docker

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/hytromo/mimosa/internal/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTagExists_ExistingTag(t *testing.T) {
	testID := rand.IntN(10000000000)
	imageTag := testutils.CreateTestImage(t, fmt.Sprintf("testapp-%d", testID), "v1.0.0")

	exists, err := TagExists(imageTag)
	require.NoError(t, err)
	assert.True(t, exists, "Tag should exist: %s", imageTag)
}

func TestTagExists_NonExistentTag(t *testing.T) {
	testID := rand.IntN(10000000000)
	nonExistentTag := fmt.Sprintf("%s/nonexistent-image-%d:tag", "localhost:5000", testID)

	exists, err := TagExists(nonExistentTag)
	require.NoError(t, err)
	assert.False(t, exists, "Tag should not exist: %s", nonExistentTag)
}

func TestTagExists_InvalidTag(t *testing.T) {
	invalidTag := "invalid-tag-format"

	exists, err := TagExists(invalidTag)
	assert.Error(t, err)
	assert.False(t, exists)
}
