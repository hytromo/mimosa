package actions

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/hytromo/mimosa/internal/cacher"
	"github.com/hytromo/mimosa/internal/configuration"
	"github.com/hytromo/mimosa/internal/testutils"
	"github.com/hytromo/mimosa/internal/utils/dockerutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Global variable to hold the shared registry for tests
var sharedRegistry *testutils.TestRegistry

func TestMain(m *testing.M) {
	// Get shared registry before running tests
	registry, err := testutils.GetSharedRegistry()
	defer testutils.CleanupSharedRegistry()

	if err != nil || registry == nil {
		fmt.Printf("Failed to get shared registry: %v\n", err)
		os.Exit(1)
		return
	}

	sharedRegistry = registry

	exitCode := m.Run()

	// Clean up before exiting
	defer os.Exit(exitCode)
}

// createTestImage creates a simple test image and pushes it to the registry
func createTestImage(t *testing.T, registry *testutils.TestRegistry, imageName, tag string) string {
	fullImageName := fmt.Sprintf("%s/%s:%s", registry.Url, imageName, tag)

	// Create a simple Dockerfile
	dockerfile := `FROM alpine:latest
RUN echo "test image" > /test.txt
CMD ["cat", "/test.txt"]`

	// Create temporary directory for Dockerfile
	tempDir, err := os.MkdirTemp("", "mimosa_test_*")
	require.NoError(t, err)
	defer func() {
		err = os.RemoveAll(tempDir)
		assert.NoError(t, err)
	}()

	dockerfilePath := filepath.Join(tempDir, "Dockerfile")
	err = os.WriteFile(dockerfilePath, []byte(dockerfile), 0644)
	require.NoError(t, err)

	// Build the image
	buildCmd := exec.Command("docker", "build", "-t", fullImageName, tempDir)
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build test image: %s", string(output))

	// Push the image to registry
	pushCmd := exec.Command("docker", "push", fullImageName)
	output, err = pushCmd.CombinedOutput()
	require.NoError(t, err, "Failed to push test image: %s", string(output))

	// remove the image after pushing
	removeCmd := exec.Command("docker", "rmi", fullImageName)
	output, err = removeCmd.CombinedOutput()
	require.NoError(t, err, "Failed to remove test image: %s", string(output))

	return fullImageName
}

// createMockCacheEntry creates a mock cache entry for testing
func createMockCacheEntry(t *testing.T, hash string, tagsByTarget map[string][]string) cacher.Cache {
	tempCacheDir := t.TempDir()

	cacheFile := cacher.CacheFile{
		TagsByTarget:  tagsByTarget,
		LastUpdatedAt: time.Now(),
	}

	data, err := json.Marshal(cacheFile)
	require.NoError(t, err)

	cachePath := filepath.Join(tempCacheDir, hash+".json")
	err = os.WriteFile(cachePath, data, 0644)
	require.NoError(t, err)

	return cacher.Cache{
		Hash:            hash,
		CacheDir:        tempCacheDir,
		InMemoryEntries: cacher.GetAllInMemoryEntries(),
	}
}

// checkTagExists checks if a Docker image tag exists using the OCI registry HTTP API
func checkTagExists(imageTag string) error {
	// Parse the image reference to extract registry, repository, and tag
	// Format: registry/repository:tag
	parts := strings.Split(imageTag, "/")
	if len(parts) < 2 {
		return fmt.Errorf("invalid image tag format: %s", imageTag)
	}

	// Extract registry and repository
	registry := parts[0]
	repository := strings.Join(parts[1:], "/")

	// Remove tag from repository
	repoParts := strings.Split(repository, ":")
	if len(repoParts) != 2 {
		return fmt.Errorf("invalid image tag format: %s", imageTag)
	}
	repo := repoParts[0]
	tag := repoParts[1]

	// Construct the OCI registry API URL
	url := fmt.Sprintf("http://%s/v2/%s/manifests/%s", registry, repo, tag)

	// Make HTTP HEAD request to check if manifest exists
	resp, err := http.Head(url)
	if err != nil {
		return fmt.Errorf("failed to check tag existence: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Check if the tag exists (200 OK means it exists)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("tag %s does not exist (status: %d)", imageTag, resp.StatusCode)
	}

	return nil
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
			actioner := New()
			testID := testutils.GenerateTestID()

			// Create test image
			var originalImage string
			if tc.multiPlatform {
				platforms := []string{"linux/amd64", "linux/arm64"}
				originalImage = createMultiPlatformTestImage(t, sharedRegistry, fmt.Sprintf("%s-%s", tc.imageName, testID), "v1.0.0", platforms)
			} else {
				originalImage = createTestImage(t, sharedRegistry, fmt.Sprintf("%s-%s", tc.imageName, testID), "v1.0.0")
			}

			// Create mock cache entry
			hash := fmt.Sprintf("test_hash_123_%s", testID)
			cacheEntry := createMockCacheEntry(t, hash, map[string][]string{
				"default": {originalImage},
			})

			// Create parsed command with new tags
			parsedCommand := configuration.ParsedCommand{
				TagsByTarget: map[string][]string{
					"default": {
						fmt.Sprintf("%s/%s-%s:v1.1.0", sharedRegistry.Url, tc.imageName, testID),
						fmt.Sprintf("%s/%s-%s:latest", sharedRegistry.Url, tc.imageName, testID),
					},
				},
				Hash:    hash,
				Command: []string{"docker", "retag"},
			}

			// Test dry run
			err := actioner.Retag(cacheEntry, parsedCommand, true)
			assert.NoError(t, err)

			// Test actual retag
			err = actioner.Retag(cacheEntry, parsedCommand, false)
			assert.NoError(t, err)

			// Verify the new tags exist
			for _, newTag := range parsedCommand.TagsByTarget["default"] {
				err := checkTagExists(newTag)
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
			actioner := New()
			testID := testutils.GenerateTestID()

			// Create test images for multiple targets
			var backendImage, frontendImage string
			if tc.multiPlatform {
				platforms := []string{"linux/amd64", "linux/arm64"}
				backendImage = createMultiPlatformTestImage(t, sharedRegistry, fmt.Sprintf("backend-%s", testID), "v1.0.0", platforms)
				frontendImage = createMultiPlatformTestImage(t, sharedRegistry, fmt.Sprintf("frontend-%s", testID), "v1.0.0", platforms)
			} else {
				backendImage = createTestImage(t, sharedRegistry, fmt.Sprintf("backend-%s", testID), "v1.0.0")
				frontendImage = createTestImage(t, sharedRegistry, fmt.Sprintf("frontend-%s", testID), "v1.0.0")
			}

			// Create mock cache entry with multiple targets
			hash := fmt.Sprintf("test_hash_multiple_%s", testID)
			cacheEntry := createMockCacheEntry(t, hash, map[string][]string{
				"backend":  {backendImage},
				"frontend": {frontendImage},
			})

			// Create parsed command with new tags for multiple targets
			parsedCommand := configuration.ParsedCommand{
				TagsByTarget: map[string][]string{
					"backend": {
						fmt.Sprintf("%s/backend-%s:v1.1.0", sharedRegistry.Url, testID),
						fmt.Sprintf("%s/backend-%s:latest", sharedRegistry.Url, testID),
					},
					"frontend": {
						fmt.Sprintf("%s/frontend-%s:v1.1.0", sharedRegistry.Url, testID),
						fmt.Sprintf("%s/frontend-%s:latest", sharedRegistry.Url, testID),
					},
				},
				Hash:    hash,
				Command: []string{"docker", "retag"},
			}

			// Test actual retag
			err := actioner.Retag(cacheEntry, parsedCommand, false)
			assert.NoError(t, err)

			// Verify all new tags exist
			for target, newTags := range parsedCommand.TagsByTarget {
				originalImage := ""
				switch target {
				case "backend":
					originalImage = backendImage
				case "frontend":
					originalImage = frontendImage
				}

				for _, newTag := range newTags {
					err := checkTagExists(newTag)
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

func TestRetag_NonExistentCache(t *testing.T) {
	actioner := New()
	testID := testutils.GenerateTestID()

	// Create cache entry that doesn't exist on disk
	hash := fmt.Sprintf("non_existent_hash_%s", testID)
	cacheEntry := cacher.Cache{
		Hash:            hash,
		CacheDir:        t.TempDir(),
		InMemoryEntries: cacher.GetAllInMemoryEntries(),
	}

	parsedCommand := configuration.ParsedCommand{
		TagsByTarget: map[string][]string{
			"default": {fmt.Sprintf("%s/testapp-%s:v1.0.0", sharedRegistry.Url, testID)},
		},
		Hash:    hash,
		Command: []string{"docker", "retag"},
	}

	// Test should fail because cache doesn't exist
	err := actioner.Retag(cacheEntry, parsedCommand, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no such file or directory")
}

func TestRetag_MismatchedTargets(t *testing.T) {
	actioner := New()
	testID := testutils.GenerateTestID()

	// Create test image
	originalImage := createTestImage(t, sharedRegistry, fmt.Sprintf("testapp-%s", testID), "v1.0.0")

	// Create mock cache entry
	hash := fmt.Sprintf("test_hash_mismatch_%s", testID)
	cacheEntry := createMockCacheEntry(t, hash, map[string][]string{
		"default": {originalImage},
	})

	// Create parsed command with different target
	parsedCommand := configuration.ParsedCommand{
		TagsByTarget: map[string][]string{
			"different_target": {fmt.Sprintf("%s/testapp-%s:v1.1.0", sharedRegistry.Url, testID)},
		},
		Hash:    hash,
		Command: []string{"docker", "retag"},
	}

	// Test should fail because targets don't match
	err := actioner.Retag(cacheEntry, parsedCommand, false)
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
			actioner := New()
			testID := testutils.GenerateTestID()

			// Create test image
			var originalImage string
			if tc.multiPlatform {
				platforms := []string{"linux/amd64", "linux/arm64"}
				originalImage = createMultiPlatformTestImage(t, sharedRegistry, fmt.Sprintf("%s-%s", tc.imageName, testID), "v1.0.0", platforms)
			} else {
				originalImage = createTestImage(t, sharedRegistry, fmt.Sprintf("%s-%s", tc.imageName, testID), "v1.0.0")
			}

			// Create mock cache entry
			hash := fmt.Sprintf("test_hash_dryrun_%s", testID)
			cacheEntry := createMockCacheEntry(t, hash, map[string][]string{
				"default": {originalImage},
			})

			// Create parsed command
			parsedCommand := configuration.ParsedCommand{
				TagsByTarget: map[string][]string{
					"default": {fmt.Sprintf("%s/%s-%s:v1.1.0", sharedRegistry.Url, tc.imageName, testID)},
				},
				Hash:    hash,
				Command: []string{"docker", "retag"},
			}

			// Test dry run - should not actually retag
			err := actioner.Retag(cacheEntry, parsedCommand, true)
			assert.NoError(t, err)

			// Verify the new tag doesn't exist (because it was dry run)
			newTag := parsedCommand.TagsByTarget["default"][0]
			err = checkTagExists(newTag)
			assert.Error(t, err, "Image should not exist in dry run mode: %s", newTag)
		})
	}
}

func TestRetag_InvalidImage(t *testing.T) {
	actioner := New()
	testID := testutils.GenerateTestID()

	// Create mock cache entry with invalid image
	hash := fmt.Sprintf("test_hash_invalid_%s", testID)
	cacheEntry := createMockCacheEntry(t, hash, map[string][]string{
		"default": {"invalid-image:tag"},
	})

	parsedCommand := configuration.ParsedCommand{
		TagsByTarget: map[string][]string{
			"default": {fmt.Sprintf("%s/testapp-%s:v1.0.0", sharedRegistry.Url, testID)},
		},
		Hash:    hash,
		Command: []string{"docker", "retag"},
	}

	// Test should fail because source image doesn't exist
	err := actioner.Retag(cacheEntry, parsedCommand, false)
	assert.Error(t, err)
}

// createMultiPlatformTestImage creates a multi-platform test image and pushes it to the registry
func createMultiPlatformTestImage(t *testing.T, registry *testutils.TestRegistry, imageName, tag string, platforms []string) string {
	fullImageName := fmt.Sprintf("%s/%s:%s", registry.Url, imageName, tag)

	// Create a simple Dockerfile
	dockerfile := `FROM alpine:latest
RUN echo "multi-platform test image" > /test.txt
RUN uname -m > /arch.txt
CMD ["cat", "/test.txt"]`

	// Create temporary directory for Dockerfile
	tempDir, err := os.MkdirTemp("", "mimosa_multiplatform_test_*")
	require.NoError(t, err)
	defer func() {
		err = os.RemoveAll(tempDir)
		assert.NoError(t, err)
	}()

	dockerfilePath := filepath.Join(tempDir, "Dockerfile")
	err = os.WriteFile(dockerfilePath, []byte(dockerfile), 0644)
	require.NoError(t, err)

	// Create a dedicated ephemeral builder for this test
	builderName := fmt.Sprintf("test_builder_%s", testutils.GenerateTestID())
	createCmd := exec.Command("docker", "buildx", "create", "--name", builderName, "--driver", "docker-container", "--driver-opt", "network=host", "--use")
	output, err := createCmd.CombinedOutput()
	require.NoError(t, err, "Failed to create test builder: %s", string(output))

	// Clean up the builder after the test
	defer func() {
		removeCmd := exec.Command("docker", "buildx", "rm", builderName)
		_, _ = removeCmd.CombinedOutput() // Ignore errors for cleanup
	}()

	// Build multi-platform image using the ephemeral builder
	platformArgs := make([]string, 0, len(platforms)*2)
	for _, platform := range platforms {
		platformArgs = append(platformArgs, "--platform", platform)
	}

	buildCmd := exec.Command("docker", "buildx", "build", "--push")
	buildCmd.Args = append(buildCmd.Args, platformArgs...)
	buildCmd.Args = append(buildCmd.Args, "-t", fullImageName, tempDir)

	output, err = buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build multi-platform test image: %s", string(output))

	return fullImageName
}

// checkMultiPlatformManifest checks if a multi-platform image has the same digests as the original
func checkMultiPlatformManifest(t *testing.T, imageTag string, originalImageTag string) {
	// Helper function to get manifest list from image tag
	getManifestList := func(tag string, description string) (*name.Reference, *v1.IndexManifest) {
		parsed, err := dockerutil.ParseTag(tag)
		if err != nil {
			t.Fatalf("failed to parse %s image tag %s: %v", description, tag, err)
		}

		ref, err := name.ParseReference(fmt.Sprintf("%s/%s:%s", parsed.Registry, parsed.ImageName, parsed.Tag))
		require.NoError(t, err, "Failed to parse reference for %s %s/%s:%s", description, parsed.Registry, parsed.ImageName, parsed.Tag)

		manifest, err := remote.Get(ref)
		require.NoError(t, err, "Failed to get manifest for %s %s", description, ref)

		manifestList, err := manifest.ImageIndex()
		if err != nil {
			t.Fatalf("%s %s is not a multi-platform image: %v", description, ref, err)
		}

		indexManifest, err := manifestList.IndexManifest()
		require.NoError(t, err, "Failed to get index manifest for %s %s", description, ref)

		return &ref, indexManifest
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

	// Parse the new image for digest verification
	parsed, err := dockerutil.ParseTag(imageTag)
	require.NoError(t, err, "Failed to parse retagged image tag %s", imageTag)

	// Check that all original digests are present
	foundDigests := make(map[string]bool)
	for _, descriptor := range indexManifest.Manifests {
		digest := descriptor.Digest.String()
		fmt.Printf("Found digest: %s\n", digest)
		foundDigests[digest] = true

		// Verify the digest exists by trying to get it
		digestRef, err := name.NewDigest(fmt.Sprintf("%s/%s@%s", parsed.Registry, parsed.ImageName, digest))
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
}
