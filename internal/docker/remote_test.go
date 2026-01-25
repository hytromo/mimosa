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
	nonExistentTag := fmt.Sprintf("localhost:5000/nonexistent-image-%d:tag", testID)

	exists, err := TagExists(nonExistentTag)
	require.NoError(t, err)
	assert.False(t, exists, "Tag should not exist: %s", nonExistentTag)
}

func TestTagExists_NonExistentTagInExistingRepo(t *testing.T) {
	testID := rand.IntN(10000000000)
	imageName := fmt.Sprintf("testapp-repo-%d", testID)

	// Create an image so the repo exists
	testutils.CreateTestImage(t, imageName, "v1.0.0")

	// Check for a tag that doesn't exist in the same repo
	nonExistentTag := fmt.Sprintf("localhost:5000/%s:nonexistent-%d", imageName, testID)

	exists, err := TagExists(nonExistentTag)
	require.NoError(t, err)
	assert.False(t, exists, "Tag should not exist: %s", nonExistentTag)
}

func TestTagExists_InvalidTagFormat_TooManyColons(t *testing.T) {
	// This is genuinely invalid - go-containerregistry will fail to parse it
	invalidTag := "invalid:tag:format:too:many:colons"

	exists, err := TagExists(invalidTag)
	assert.Error(t, err)
	assert.False(t, exists)
}

func TestTagExists_InvalidTagFormat_EmptyString(t *testing.T) {
	// Empty string is invalid
	exists, err := TagExists("")
	assert.Error(t, err)
	assert.False(t, exists)
}

func TestTagExists_AfterImageDeletion(t *testing.T) {
	testID := rand.IntN(10000000000)
	imageName := fmt.Sprintf("deleted-image-%d", testID)

	// Create an image
	imageTag := testutils.CreateTestImage(t, imageName, "v1.0.0")

	// Verify it exists
	exists, err := TagExists(imageTag)
	require.NoError(t, err)
	assert.True(t, exists, "Tag should exist after creation: %s", imageTag)
}
