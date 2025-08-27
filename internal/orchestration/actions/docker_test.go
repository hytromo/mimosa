package actions

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hytromo/mimosa/internal/cacher"
	"github.com/hytromo/mimosa/internal/configuration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	sharedRegistry *testRegistry
)

// generateTestID generates a unique test identifier to avoid conflicts between tests
func generateTestID() string {
	// Generate 8 random bytes and encode as hex
	bytes := make([]byte, 8)
	_, err := rand.Read(bytes)
	if err != nil {
		panic(fmt.Sprintf("failed to generate test ID: %v", err))
	}
	return fmt.Sprintf("%x", bytes)
}

type testRegistry struct {
	port int
	name string
	url  string
}

// startSharedRegistry starts a single Docker registry that will be shared across all tests
func startSharedRegistry() (*testRegistry, error) {
	// Generate a random port between 5000-65535
	portRange := big.NewInt(60535) // 65535 - 5000
	randomPort, err := rand.Int(rand.Reader, portRange)
	if err != nil {
		return nil, err
	}
	port := int(randomPort.Int64()) + 5000

	// Generate a unique container name
	name := fmt.Sprintf("mimosa_registry_%d", randomPort.Int64())
	url := fmt.Sprintf("localhost:%d", port)

	// Start the registry
	cmd := exec.Command("docker", "run", "-d", "--rm",
		"-p", fmt.Sprintf("%d:5000", port),
		"--name", name,
		"registry:3")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to start registry: %s", string(output))
	}

	// Wait for registry to be ready
	timeoutSeconds := 30
	timeout := time.Now().Add(time.Duration(timeoutSeconds) * time.Second)
	for time.Now().Before(timeout) {
		resp, err := http.Get(fmt.Sprintf("http://%s/v2/", url))
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return &testRegistry{
					port: port,
					name: name,
					url:  url,
				}, nil
			}
		}
		time.Sleep(1 * time.Second)
	}

	return nil, fmt.Errorf("registry failed to start within %d seconds", timeoutSeconds)
}

func TestMain(m *testing.M) {
	// Start shared registry before running tests
	registry, err := startSharedRegistry()

	if err != nil || registry == nil {
		fmt.Printf("Failed to start shared registry: %v\n", err)
		os.Exit(1)
		return
	}

	fmt.Printf("Shared test registry started on port %d with name %s\n", registry.port, registry.name)

	sharedRegistry = registry

	exitCode := m.Run()

	// Clean up before exiting
	registry.cleanup(nil)
	os.Exit(exitCode)
}

// cleanup stops and removes the test registry container
func (tr *testRegistry) cleanup(t *testing.T) {
	if tr.name == "" {
		return
	}

	killCmd := exec.Command("docker", "kill", "-s", "9", tr.name)
	killOutput, killErr := killCmd.CombinedOutput()
	if killErr != nil {
		if t != nil {
			t.Logf("Failed to stop/kill registry container: %s, %s", string(killOutput), string(killOutput))
		} else {
			fmt.Printf("Failed to stop/kill registry container: %s, %s\n", string(killOutput), string(killOutput))
		}
	}

	if t != nil {
		t.Logf("Shared test registry cleaned up: %s", tr.name)
	} else {
		fmt.Printf("Shared test registry cleaned up: %s\n", tr.name)
	}
}

// createTestImage creates a simple test image and pushes it to the registry
func createTestImage(t *testing.T, registry *testRegistry, imageName, tag string) string {
	fullImageName := fmt.Sprintf("%s/%s:%s", registry.url, imageName, tag)

	// Create a simple Dockerfile
	dockerfile := `FROM alpine:latest
RUN echo "test image" > /test.txt
CMD ["cat", "/test.txt"]`

	// Create temporary directory for Dockerfile
	tempDir, err := os.MkdirTemp("", "mimosa_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

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
	// Create temporary cache directory
	tempCacheDir, err := os.MkdirTemp("", "mimosa_cache_test_*")
	require.NoError(t, err)

	// Override cache directory for test
	originalCacheDir := cacher.CacheDir
	cacher.CacheDir = tempCacheDir
	t.Cleanup(func() {
		cacher.CacheDir = originalCacheDir
		os.RemoveAll(tempCacheDir)
	})

	// Create cache file
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
	defer resp.Body.Close()

	// Check if the tag exists (200 OK means it exists)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("tag %s does not exist (status: %d)", imageTag, resp.StatusCode)
	}

	return nil
}

func TestRetag_SingleTarget(t *testing.T) {
	actioner := New()
	testID := generateTestID()

	// Create test image
	originalImage := createTestImage(t, sharedRegistry, fmt.Sprintf("testapp-%s", testID), "v1.0.0")

	// Create mock cache entry
	hash := fmt.Sprintf("test_hash_123_%s", testID)
	cacheEntry := createMockCacheEntry(t, hash, map[string][]string{
		"default": {originalImage},
	})

	// Create parsed command with new tags
	parsedCommand := configuration.ParsedCommand{
		TagsByTarget: map[string][]string{
			"default": {
				fmt.Sprintf("%s/testapp-%s:v1.1.0", sharedRegistry.url, testID),
				fmt.Sprintf("%s/testapp-%s:latest", sharedRegistry.url, testID),
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
	}
}

func TestRetag_MultipleTargets(t *testing.T) {
	actioner := New()
	testID := generateTestID()

	// Create test images for multiple targets
	backendImage := createTestImage(t, sharedRegistry, fmt.Sprintf("backend-%s", testID), "v1.0.0")
	frontendImage := createTestImage(t, sharedRegistry, fmt.Sprintf("frontend-%s", testID), "v1.0.0")

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
				fmt.Sprintf("%s/backend-%s:v1.1.0", sharedRegistry.url, testID),
				fmt.Sprintf("%s/backend-%s:latest", sharedRegistry.url, testID),
			},
			"frontend": {
				fmt.Sprintf("%s/frontend-%s:v1.1.0", sharedRegistry.url, testID),
				fmt.Sprintf("%s/frontend-%s:latest", sharedRegistry.url, testID),
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
		for _, newTag := range newTags {
			err := checkTagExists(newTag)
			assert.NoError(t, err, "Failed to check retagged image %s for target %s: %s", newTag, target, err)
		}
	}
}

func TestRetag_NonExistentCache(t *testing.T) {
	actioner := New()
	testID := generateTestID()

	// Create cache entry that doesn't exist on disk
	hash := fmt.Sprintf("non_existent_hash_%s", testID)
	cacheEntry := cacher.Cache{
		Hash:            hash,
		InMemoryEntries: cacher.GetAllInMemoryEntries(),
	}

	parsedCommand := configuration.ParsedCommand{
		TagsByTarget: map[string][]string{
			"default": {fmt.Sprintf("%s/testapp-%s:v1.0.0", sharedRegistry.url, testID)},
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
	testID := generateTestID()

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
			"different_target": {fmt.Sprintf("%s/testapp-%s:v1.1.0", sharedRegistry.url, testID)},
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
	actioner := New()
	testID := generateTestID()

	// Create test image
	originalImage := createTestImage(t, sharedRegistry, fmt.Sprintf("testapp-dryrun-%s", testID), "v1.0.0")

	// Create mock cache entry
	hash := fmt.Sprintf("test_hash_dryrun_%s", testID)
	cacheEntry := createMockCacheEntry(t, hash, map[string][]string{
		"default": {originalImage},
	})

	// Create parsed command
	parsedCommand := configuration.ParsedCommand{
		TagsByTarget: map[string][]string{
			"default": {fmt.Sprintf("%s/testapp-dryrun-%s:v1.1.0", sharedRegistry.url, testID)},
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
}

func TestRetag_InvalidImage(t *testing.T) {
	actioner := New()
	testID := generateTestID()

	// Create mock cache entry with invalid image
	hash := fmt.Sprintf("test_hash_invalid_%s", testID)
	cacheEntry := createMockCacheEntry(t, hash, map[string][]string{
		"default": {"invalid-image:tag"},
	})

	parsedCommand := configuration.ParsedCommand{
		TagsByTarget: map[string][]string{
			"default": {fmt.Sprintf("%s/testapp-%s:v1.0.0", sharedRegistry.url, testID)},
		},
		Hash:    hash,
		Command: []string{"docker", "retag"},
	}

	// Test should fail because source image doesn't exist
	err := actioner.Retag(cacheEntry, parsedCommand, false)
	assert.Error(t, err)
}
