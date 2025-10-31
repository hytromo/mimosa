package testutils

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CreateTestImage creates a simple test image and pushes it to the registry
func CreateTestImage(t *testing.T, imageName, tag string) string {
	fullImageName := fmt.Sprintf("%s/%s:%s", "localhost:5000", imageName, tag)

	// Create a simple Dockerfile
	dockerfile := `FROM alpine:latest
RUN echo "test image" > /test.txt
CMD ["cat", "/test.txt"]`

	// Create temporary directory for Dockerfile
	tempDir, err := os.MkdirTemp("", "mimosa_test_*")
	require.NoError(t, err)
	defer func() {
		err := os.RemoveAll(tempDir)
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

// CreateMultiPlatformTestImage creates a multi-platform test image and pushes it to the registry
func CreateMultiPlatformTestImage(t *testing.T, imageName, tag string, platforms []string) string {
	fullImageName := fmt.Sprintf("%s/%s:%s", "localhost:5000", imageName, tag)

	// Create a simple Dockerfile
	dockerfile := `FROM alpine:latest
RUN echo "multi-platform test image" > /test.txt
RUN uname -m > /arch.txt
CMD ["cat", "/test.txt"]`

	// Create temporary directory for Dockerfile
	tempDir, err := os.MkdirTemp("", "mimosa_multiplatform_test_*")
	require.NoError(t, err)
	defer func() {
		err := os.RemoveAll(tempDir)
		assert.NoError(t, err)
	}()

	dockerfilePath := filepath.Join(tempDir, "Dockerfile")
	err = os.WriteFile(dockerfilePath, []byte(dockerfile), 0644)
	require.NoError(t, err)

	// Build multi-platform image using the ephemeral builder
	platformArgs := make([]string, 0, len(platforms)*2)
	for _, platform := range platforms {
		platformArgs = append(platformArgs, "--platform", platform)
	}

	buildCmd := exec.Command("docker", "buildx", "build", "--push")
	buildCmd.Args = append(buildCmd.Args, platformArgs...)
	buildCmd.Args = append(buildCmd.Args, "-t", fullImageName, tempDir)

	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build multi-platform test image: %s", string(output))

	return fullImageName
}

// CheckTagExists checks if a Docker image tag exists using the OCI registry HTTP API
func CheckTagExists(imageTag string) error {
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
	defer func() {
		_ = resp.Body.Close()
	}()

	// Check if the tag exists (200 OK means it exists)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("tag %s does not exist (status: %d)", imageTag, resp.StatusCode)
	}

	return nil
}

// GetImageDigests gets the digests of an image
func GetImageDigests(t *testing.T, imageTag string) []string {
	parsed, err := name.ParseReference(imageTag)
	require.NoError(t, err, "Failed to parse image tag %s", imageTag)

	manifest, err := remote.Get(parsed)
	require.NoError(t, err, "Failed to get manifest for %s", parsed)

	// Check if it's a multi-platform image
	manifestList, err := manifest.ImageIndex()
	if err == nil {
		// It's a multi-platform image
		indexManifest, err := manifestList.IndexManifest()
		require.NoError(t, err, "Failed to get index manifest for %s", parsed)

		digests := make([]string, len(indexManifest.Manifests))
		for i, descriptor := range indexManifest.Manifests {
			digests[i] = descriptor.Digest.String()
		}
		return digests
	}

	// It's a single platform image
	img, err := manifest.Image()
	require.NoError(t, err, "Failed to get image for %s", parsed)

	digest, err := img.Digest()
	require.NoError(t, err, "Failed to get digest for %s", parsed)

	return []string{digest.String()}
}
