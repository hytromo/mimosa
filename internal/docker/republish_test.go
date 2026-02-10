package docker

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/hytromo/mimosa/internal/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublishManifestsUnderTag_SingleImage(t *testing.T) {
	testID := rand.IntN(10000000000)
	originalImage := testutils.CreateTestImage(t, fmt.Sprintf("testapp-%d", testID), "v1.0.0")

	// Get the descriptors of the original image
	// Note: modern Docker Desktop may create OCI indexes with attestation manifests
	// even for single-platform builds, so there may be more than one descriptor
	originalDescriptors := testutils.GetImageDescriptors(t, originalImage)
	require.GreaterOrEqual(t, len(originalDescriptors), 1, "Single image should have at least one descriptor")

	// Extract image name without tag
	imageName := fmt.Sprintf("%s/testapp-%d", "localhost:5000", testID)
	newTag := "v1.1.0"

	// Publish manifests under new tag (same repository)
	err := PublishManifestsUnderTag(imageName, newTag, originalDescriptors)
	assert.NoError(t, err)

	// Verify the new tag exists
	newImageTag := fmt.Sprintf("%s:%s", imageName, newTag)
	err = testutils.CheckTagExists(newImageTag)
	assert.NoError(t, err, "Failed to check published image %s: %s", newImageTag, err)

	// Verify the new image has the same digests
	newDigests := testutils.GetImageDigests(t, newImageTag)
	originalDigests := make([]string, len(originalDescriptors))
	for i, d := range originalDescriptors {
		originalDigests[i] = d.Digest.String()
	}
	assert.Equal(t, originalDigests, newDigests, "New image should have the same digests as original")
}

func TestPublishManifestsUnderTag_MultiPlatformImage(t *testing.T) {
	testID := rand.IntN(10000000000)
	platforms := []string{"linux/amd64", "linux/arm64"}
	originalImage := testutils.CreateMultiPlatformTestImage(t, fmt.Sprintf("multiplatform-app-%d", testID), "v1.0.0", platforms)

	// Get the full descriptors of the original image (including platform info)
	originalDescriptors := testutils.GetImageDescriptors(t, originalImage)
	require.GreaterOrEqual(t, len(originalDescriptors), 2, "Multi-platform image should have at least 2 descriptors")

	// Extract image name without tag
	imageName := fmt.Sprintf("%s/multiplatform-app-%d", "localhost:5000", testID)
	newTag := "v1.1.0"

	// Publish manifests under new tag (same repository)
	err := PublishManifestsUnderTag(imageName, newTag, originalDescriptors)
	assert.NoError(t, err)

	// Verify the new tag exists
	newImageTag := fmt.Sprintf("%s:%s", imageName, newTag)
	err = testutils.CheckTagExists(newImageTag)
	assert.NoError(t, err, "Failed to check published image %s: %s", newImageTag, err)

	// Verify the new image has the same digests
	originalDigests := make([]string, len(originalDescriptors))
	for i, d := range originalDescriptors {
		originalDigests[i] = d.Digest.String()
	}
	newDigests := testutils.GetImageDigests(t, newImageTag)
	assert.Equal(t, originalDigests, newDigests, "New image should have the same digests as original")

	// Verify it's still a multi-platform image with preserved platform info
	parsed, err := name.ParseReference(newImageTag)
	require.NoError(t, err, "Failed to parse new image tag %s", newImageTag)

	manifest, err := remote.Get(parsed)
	require.NoError(t, err, "Failed to get manifest for %s", parsed)

	manifestList, err := manifest.ImageIndex()
	require.NoError(t, err, "New image should still be a multi-platform image")

	indexManifest, err := manifestList.IndexManifest()
	require.NoError(t, err, "Failed to get index manifest for new image")

	assert.GreaterOrEqual(t, len(indexManifest.Manifests), 2, "New image should still have at least 2 manifests")

	// Verify platform information is preserved on each manifest entry
	for i, desc := range indexManifest.Manifests {
		require.NotNil(t, desc.Platform, "Manifest %d should have platform info", i)
		assert.NotEmpty(t, desc.Platform.OS, "Manifest %d should have non-empty OS", i)
		assert.NotEmpty(t, desc.Platform.Architecture, "Manifest %d should have non-empty Architecture", i)
	}
}

func TestPublishManifestsUnderTag_MixedManifests(t *testing.T) {
	testID := rand.IntN(10000000000)

	// Create a multi-platform image to get multiple descriptors from the same image
	imageName := fmt.Sprintf("%s/mixed-app-%d", "localhost:5000", testID)
	multiImage := testutils.CreateMultiPlatformTestImage(t, fmt.Sprintf("mixed-app-%d", testID), "v1.0.0", []string{"linux/amd64", "linux/arm64"})

	// Get descriptors from the multi-platform image
	multiDescriptors := testutils.GetImageDescriptors(t, multiImage)

	// Publish manifests under new tag (same repository)
	newTag := "v1.1.0"
	err := PublishManifestsUnderTag(imageName, newTag, multiDescriptors)
	assert.NoError(t, err)

	// Verify the new tag exists
	newImageTag := fmt.Sprintf("%s:%s", imageName, newTag)
	err = testutils.CheckTagExists(newImageTag)
	assert.NoError(t, err, "Failed to check published image %s: %s", newImageTag, err)

	// Verify the new image has the expected digests
	newDigests := testutils.GetImageDigests(t, newImageTag)
	assert.Equal(t, len(multiDescriptors), len(newDigests), "New image should have the same number of digests")
}

func TestPublishManifestsUnderTag_InvalidImageName(t *testing.T) {
	// Test with invalid image name
	invalidImageName := "invalid:image:name"
	newTag := "v1.1.0"
	someDescriptors := []v1.Descriptor{{
		Digest: v1.Hash{Algorithm: "sha256", Hex: "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"},
	}}

	// Publish manifests under new tag should fail (invalid image name)
	err := PublishManifestsUnderTag(invalidImageName, newTag, someDescriptors)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "creating tag ref")
}

func TestPublishManifestsUnderTag_InvalidDigest(t *testing.T) {
	testID := rand.IntN(10000000000)
	imageName := fmt.Sprintf("%s/testapp-%d", "localhost:5000", testID)
	newTag := "v1.1.0"

	// Test with invalid digest (empty hash produces invalid digest ref)
	invalidDescriptors := []v1.Descriptor{{
		Digest: v1.Hash{Algorithm: "invalid", Hex: "format"},
	}}

	// Publish manifests under new tag should fail
	err := PublishManifestsUnderTag(imageName, newTag, invalidDescriptors)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "creating digest ref")
}

func TestPublishManifestsUnderTag_NonExistentDigest(t *testing.T) {
	testID := rand.IntN(10000000000)
	imageName := fmt.Sprintf("%s/testapp-%d", "localhost:5000", testID)
	newTag := "v1.1.0"

	// Test with non-existent digest
	nonExistentDescriptors := []v1.Descriptor{{
		Digest: v1.Hash{Algorithm: "sha256", Hex: "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"},
	}}

	// Publish manifests under new tag should fail
	err := PublishManifestsUnderTag(imageName, newTag, nonExistentDescriptors)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "fetching descriptor")
}

func TestPublishManifestsUnderTag_EmptyManifests(t *testing.T) {
	testID := rand.IntN(10000000000)
	imageName := fmt.Sprintf("%s/testapp-%d", "localhost:5000", testID)
	newTag := "v1.1.0"

	// Test with empty manifests list
	emptyDescriptors := []v1.Descriptor{}

	// Publish manifests under new tag should fail
	err := PublishManifestsUnderTag(imageName, newTag, emptyDescriptors)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no manifests provided")
}

func TestPublishManifestsUnderTag_InvalidTag(t *testing.T) {
	testID := rand.IntN(10000000000)
	originalImage := testutils.CreateTestImage(t, fmt.Sprintf("testapp-%d", testID), "v1.0.0")

	// Get the descriptors of the original image
	originalDescriptors := testutils.GetImageDescriptors(t, originalImage)

	// Extract image name without tag
	imageName := fmt.Sprintf("%s/testapp-%d", "localhost:5000", testID)
	invalidTag := "invalid:tag:format"

	// Publish manifests under new tag should fail
	err := PublishManifestsUnderTag(imageName, invalidTag, originalDescriptors)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "creating tag ref")
}

func TestPublishManifestsUnderTag_OverwriteExistingTag(t *testing.T) {
	testID := rand.IntN(10000000000)
	originalImage := testutils.CreateTestImage(t, fmt.Sprintf("testapp-%d", testID), "v1.0.0")

	// Get the descriptors of the original image
	originalDescriptors := testutils.GetImageDescriptors(t, originalImage)

	// Extract image name without tag
	imageName := fmt.Sprintf("%s/testapp-%d", "localhost:5000", testID)
	tag := "v1.1.0"

	// Publish manifests under tag for the first time
	err := PublishManifestsUnderTag(imageName, tag, originalDescriptors)
	assert.NoError(t, err)

	// Verify the tag exists
	imageTag := fmt.Sprintf("%s:%s", imageName, tag)
	err = testutils.CheckTagExists(imageTag)
	assert.NoError(t, err, "Failed to check first published image %s: %s", imageTag, err)

	// Publish manifests under the same tag again (should overwrite)
	err = PublishManifestsUnderTag(imageName, tag, originalDescriptors)
	assert.NoError(t, err)

	// Verify the tag still exists
	err = testutils.CheckTagExists(imageTag)
	assert.NoError(t, err, "Failed to check overwritten image %s: %s", imageTag, err)

	// Verify the image still has the same digests
	originalDigests := make([]string, len(originalDescriptors))
	for i, d := range originalDescriptors {
		originalDigests[i] = d.Digest.String()
	}
	newDigests := testutils.GetImageDigests(t, imageTag)
	assert.Equal(t, originalDigests, newDigests, "Overwritten image should have the same digests")
}

func TestPublishManifestsUnderTag_LargeNumberOfManifests(t *testing.T) {
	testID := rand.IntN(10000000000)

	// Create a multi-platform image with multiple platforms to get more descriptors
	imageName := fmt.Sprintf("%s/large-app-%d", "localhost:5000", testID)
	multiImage := testutils.CreateMultiPlatformTestImage(t, fmt.Sprintf("large-app-%d", testID), "v1.0.0", []string{"linux/amd64", "linux/arm64", "linux/386"})
	multiDescriptors := testutils.GetImageDescriptors(t, multiImage)

	// Publish all manifests under new tag
	newTag := "v1.1.0"
	err := PublishManifestsUnderTag(imageName, newTag, multiDescriptors)
	assert.NoError(t, err)

	// Verify the new tag exists
	newImageTag := fmt.Sprintf("%s:%s", imageName, newTag)
	err = testutils.CheckTagExists(newImageTag)
	assert.NoError(t, err, "Failed to check published image %s: %s", newImageTag, err)

	// Verify the new image has the expected number of digests
	newDigests := testutils.GetImageDigests(t, newImageTag)
	assert.Equal(t, len(multiDescriptors), len(newDigests), "New image should have the same number of digests")
}
