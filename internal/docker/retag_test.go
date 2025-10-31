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

func TestRetagSingle_SinglePlatform(t *testing.T) {
	testID := rand.IntN(10000000000)
	originalImage := testutils.CreateTestImage(t, fmt.Sprintf("testapp-%d", testID), "v1.0.0")
	newTag := fmt.Sprintf("%s/testapp-%d:v1.1.0", "localhost:5000", testID)

	// Test dry run
	err := RetagSingleTag(originalImage, newTag, true)
	assert.NoError(t, err)

	// Verify the new tag doesn't exist (because it was dry run)
	err = testutils.CheckTagExists(newTag)
	assert.Error(t, err, "Image should not exist in dry run mode: %s", newTag)

	// Test actual retag
	err = RetagSingleTag(originalImage, newTag, false)
	assert.NoError(t, err)

	// Verify the new tag exists
	err = testutils.CheckTagExists(newTag)
	assert.NoError(t, err, "Failed to check retagged image %s: %s", newTag, err)
}

func TestRetagSingle_MultiPlatform(t *testing.T) {
	testID := rand.IntN(10000000000)
	platforms := []string{"linux/amd64", "linux/arm64"}
	originalImage := testutils.CreateMultiPlatformTestImage(t, fmt.Sprintf("multiplatform-app-%d", testID), "v1.0.0", platforms)
	newTag := fmt.Sprintf("%s/multiplatform-app-%d:v1.1.0", "localhost:5000", testID)

	// Test actual retag
	err := RetagSingleTag(originalImage, newTag, false)
	assert.NoError(t, err)

	// Verify the new tag exists
	err = testutils.CheckTagExists(newTag)
	assert.NoError(t, err, "Failed to check retagged image %s: %s", newTag, err)

	// Check that all original digests are preserved
	checkMultiPlatformManifest(t, newTag, originalImage)
}

func TestRetagSingle_InvalidSourceTag(t *testing.T) {
	testID := rand.IntN(10000000000)
	newTag := fmt.Sprintf("%s/testapp-%d:v1.0.0", "localhost:5000", testID)

	// Test with invalid source tag
	err := RetagSingleTag("invalid-image:tag", newTag, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get descriptor")
}

func TestRetagSingle_InvalidTargetTag(t *testing.T) {
	testID := rand.IntN(10000000000)
	originalImage := testutils.CreateTestImage(t, fmt.Sprintf("testapp-%d", testID), "v1.0.0")

	// Test with invalid target tag
	err := RetagSingleTag(originalImage, "invalid-target:tag", false)
	assert.Error(t, err)
}

func TestRetag_SingleTarget(t *testing.T) {
	testCases := []struct {
		name           string
		multiPlatform  bool
		imageName      string
		verifyManifest bool
	}{
		{
			name:           "Single Platform",
			multiPlatform:  false,
			imageName:      "testapp",
			verifyManifest: false,
		},
		{
			name:           "Multi Platform",
			multiPlatform:  true,
			imageName:      "multiplatform-app",
			verifyManifest: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testID := rand.IntN(10000000000)

			// Create test image
			var originalImage string
			if tc.multiPlatform {
				platforms := []string{"linux/amd64", "linux/arm64"}
				originalImage = testutils.CreateMultiPlatformTestImage(t, fmt.Sprintf("%s-%d", tc.imageName, testID), "v1.0.0", platforms)
			} else {
				originalImage = testutils.CreateTestImage(t, fmt.Sprintf("%s-%d", tc.imageName, testID), "v1.0.0")
			}

			// Create parsed command with new tags
			latestTagByTarget := map[string]string{
				"default": originalImage,
			}
			newTagsByTarget := map[string][]string{
				"default": {
					fmt.Sprintf("%s/%s-%d:v1.1.0", "localhost:5000", tc.imageName, testID),
					fmt.Sprintf("%s/%s-%d:latest", "localhost:5000", tc.imageName, testID),
				},
			}

			// Test dry run
			err := Retag(latestTagByTarget, newTagsByTarget, true)
			assert.NoError(t, err)

			// Test actual retag
			err = Retag(latestTagByTarget, newTagsByTarget, false)
			assert.NoError(t, err)

			// Verify the new tags exist
			for _, newTag := range newTagsByTarget["default"] {
				err := testutils.CheckTagExists(newTag)
				assert.NoError(t, err, "Failed to check retagged image %s: %s", newTag, err)

				// For multi-platform images, also check that all original digests are preserved
				if tc.verifyManifest {
					checkMultiPlatformManifest(t, newTag, originalImage)
				}
			}
		})
	}
}

func TestRetag_MultipleTargets(t *testing.T) {
	testCases := []struct {
		name           string
		multiPlatform  bool
		verifyManifest bool
	}{
		{
			name:           "Single Platform",
			multiPlatform:  false,
			verifyManifest: false,
		},
		{
			name:           "Multi Platform",
			multiPlatform:  true,
			verifyManifest: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testID := rand.IntN(10000000000)

			// Create test images for multiple targets
			var backendImage, frontendImage string
			if tc.multiPlatform {
				platforms := []string{"linux/amd64", "linux/arm64"}
				backendImage = testutils.CreateMultiPlatformTestImage(t, fmt.Sprintf("backend-%d", testID), "v1.0.0", platforms)
				frontendImage = testutils.CreateMultiPlatformTestImage(t, fmt.Sprintf("frontend-%d", testID), "v1.0.0", platforms)
			} else {
				backendImage = testutils.CreateTestImage(t, fmt.Sprintf("backend-%d", testID), "v1.0.0")
				frontendImage = testutils.CreateTestImage(t, fmt.Sprintf("frontend-%d", testID), "v1.0.0")
			}

			// Create parsed command with new tags for multiple targets
			latestTagByTarget := map[string]string{
				"backend":  backendImage,
				"frontend": frontendImage,
			}
			newTagsByTarget := map[string][]string{
				"backend": {
					fmt.Sprintf("%s/backend-%d:v1.1.0", "localhost:5000", testID),
					fmt.Sprintf("%s/backend-%d:latest", "localhost:5000", testID),
				},
				"frontend": {
					fmt.Sprintf("%s/frontend-%d:v1.1.0", "localhost:5000", testID),
					fmt.Sprintf("%s/frontend-%d:latest", "localhost:5000", testID),
				},
			}

			// Test actual retag
			err := Retag(latestTagByTarget, newTagsByTarget, false)
			assert.NoError(t, err)

			// Verify all new tags exist
			for target, newTags := range newTagsByTarget {
				originalImage := ""
				switch target {
				case "backend":
					originalImage = backendImage
				case "frontend":
					originalImage = frontendImage
				}

				for _, newTag := range newTags {
					err := testutils.CheckTagExists(newTag)
					assert.NoError(t, err, "Failed to check retagged image %s for target %s: %s", newTag, target, err)

					// For multi-platform images, also check that all original digests are preserved
					if tc.verifyManifest {
						checkMultiPlatformManifest(t, newTag, originalImage)
					}
				}
			}
		})
	}
}

func TestRetag_DifferentTargetCounts(t *testing.T) {
	testID := rand.IntN(10000000000)
	originalImage := testutils.CreateTestImage(t, fmt.Sprintf("testapp-%d", testID), "v1.0.0")

	latestTagByTarget := map[string]string{
		"default": originalImage,
	}
	newTagsByTarget := map[string][]string{
		"default": {fmt.Sprintf("%s/testapp-%d:v1.1.0", "localhost:5000", testID)},
		"extra":   {fmt.Sprintf("%s/extra-%d:v1.1.0", "localhost:5000", testID)},
	}

	// Test should fail because target counts don't match
	err := Retag(latestTagByTarget, newTagsByTarget, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "different amount of targets between cache and new tags")
}

func TestRetag_DifferentTargets(t *testing.T) {
	testID := rand.IntN(10000000000)
	originalImage := testutils.CreateTestImage(t, fmt.Sprintf("testapp-%d", testID), "v1.0.0")

	latestTagByTarget := map[string]string{
		"default": originalImage,
	}
	newTagsByTarget := map[string][]string{
		"different_target": {fmt.Sprintf("%s/testapp-%d:v1.1.0", "localhost:5000", testID)},
	}

	// Test should fail because targets don't match
	err := Retag(latestTagByTarget, newTagsByTarget, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "different targets between cache and new tags")
}

func TestRetag_DryRun(t *testing.T) {
	testCases := []struct {
		name          string
		multiPlatform bool
		imageName     string
	}{
		{
			name:          "Single Platform",
			multiPlatform: false,
			imageName:     "testapp-dryrun",
		},
		{
			name:          "Multi Platform",
			multiPlatform: true,
			imageName:     "multiplatform-dryrun",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testID := rand.IntN(10000000000)

			// Create test image
			var originalImage string
			if tc.multiPlatform {
				platforms := []string{"linux/amd64", "linux/arm64"}
				originalImage = testutils.CreateMultiPlatformTestImage(t, fmt.Sprintf("%s-%d", tc.imageName, testID), "v1.0.0", platforms)
			} else {
				originalImage = testutils.CreateTestImage(t, fmt.Sprintf("%s-%d", tc.imageName, testID), "v1.0.0")
			}

			// Create parsed command
			latestTagByTarget := map[string]string{
				"default": originalImage,
			}
			newTagsByTarget := map[string][]string{
				"default": {fmt.Sprintf("%s/%s-%d:v1.1.0", "localhost:5000", tc.imageName, testID)},
			}

			// Test dry run - should not actually retag
			err := Retag(latestTagByTarget, newTagsByTarget, true)
			assert.NoError(t, err)

			// Verify the new tag doesn't exist (because it was dry run)
			newTag := newTagsByTarget["default"][0]
			err = testutils.CheckTagExists(newTag)
			assert.Error(t, err, "Image should not exist in dry run mode: %s", newTag)
		})
	}
}

func TestSimpleRetag_Success(t *testing.T) {
	testID := rand.IntN(10000000000)
	originalImage := testutils.CreateTestImage(t, fmt.Sprintf("testapp-%d", testID), "v1.0.0")
	newTag := fmt.Sprintf("%s/testapp-%d:v1.1.0", "localhost:5000", testID)

	// Test simple retag
	err := SimpleRetag(originalImage, newTag)
	assert.NoError(t, err)

	// Verify the new tag exists
	err = testutils.CheckTagExists(newTag)
	assert.NoError(t, err, "Failed to check retagged image %s: %s", newTag, err)
}

func TestSimpleRetag_InvalidSourceReference(t *testing.T) {
	testID := rand.IntN(10000000000)
	newTag := fmt.Sprintf("%s/testapp-%d:v1.0.0", "localhost:5000", testID)

	// Test with invalid source reference
	err := SimpleRetag("invalid:reference:format", newTag)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse source reference")
}

func TestSimpleRetag_InvalidTargetReference(t *testing.T) {
	testID := rand.IntN(10000000000)
	originalImage := testutils.CreateTestImage(t, fmt.Sprintf("testapp-%d", testID), "v1.0.0")

	// Test with invalid target reference
	err := SimpleRetag(originalImage, "invalid:reference:format")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse destination reference")
}

func TestSimpleRetag_NonExistentSource(t *testing.T) {
	testID := rand.IntN(10000000000)
	newTag := fmt.Sprintf("%s/testapp-%d:v1.0.0", "localhost:5000", testID)

	// Test with non-existent source image
	err := SimpleRetag("nonexistent/image:tag", newTag)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get image from source reference")
}

// checkMultiPlatformManifest checks if a multi-platform image has the same digests as the original
func checkMultiPlatformManifest(t *testing.T, imageTag string, originalImageTag string) {
	// Helper function to get manifest list from image tag
	getManifestList := func(tag string, description string) (*name.Reference, *v1.IndexManifest) {
		parsed, err := name.ParseReference(tag)
		if err != nil {
			t.Fatalf("failed to parse %s image tag %s: %v", description, tag, err)
		}

		manifest, err := remote.Get(parsed)
		require.NoError(t, err, "Failed to get manifest for %s %s", description, parsed)

		manifestList, err := manifest.ImageIndex()
		if err != nil {
			t.Fatalf("%s %s is not a multi-platform image: %v", description, parsed, err)
		}

		indexManifest, err := manifestList.IndexManifest()
		require.NoError(t, err, "Failed to get index manifest for %s %s", description, parsed)

		return &parsed, indexManifest
	}

	// Get original image manifest list
	originalRef, originalIndexManifest := getManifestList(originalImageTag, "original")

	// Get original digests
	originalDigests := make(map[string]bool)
	for _, descriptor := range originalIndexManifest.Manifests {
		originalDigests[descriptor.Digest.String()] = true
	}

	// Assert that original manifest list has at least 2 manifests
	assert.GreaterOrEqual(t, len(originalIndexManifest.Manifests), 2,
		"Original image %s should have at least 2 manifests, but has %d", *originalRef, len(originalIndexManifest.Manifests))

	// Get new image manifest list
	ref, indexManifest := getManifestList(imageTag, "retagged")

	// Assert that retagged manifest list has at least 2 manifests
	assert.GreaterOrEqual(t, len(indexManifest.Manifests), 2,
		"Retagged image %s should have at least 2 manifests, but has %d", *ref, len(indexManifest.Manifests))

	// Check that all original digests are present
	foundDigests := make(map[string]bool)
	for _, descriptor := range indexManifest.Manifests {
		digest := descriptor.Digest.String()
		foundDigests[digest] = true

		// Verify the digest exists by trying to get it
		digestRef, err := name.NewDigest(fmt.Sprintf("%s@%s", (*ref).Context(), digest))
		require.NoError(t, err, "Failed to create digest reference for %s", digest)

		_, err = remote.Get(digestRef)
		require.NoError(t, err, "Failed to get manifest for digest %s", digest)
	}

	// Verify all original digests are present
	for originalDigest := range originalDigests {
		assert.True(t, foundDigests[originalDigest],
			"Original digest %s not found in retagged image %s. Found digests: %v",
			originalDigest, *ref, foundDigests)
	}

	t.Logf("Multi-platform image %s contains all original digests: %v", *ref, originalDigests)
}
