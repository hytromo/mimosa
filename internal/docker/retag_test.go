package docker

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

func dockerUsername(t *testing.T) string {
	configPath := filepath.Join(os.Getenv("HOME"), ".docker", "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Skipf("cannot read Docker config: %v", err)
	}

	var config struct {
		Auths map[string]struct {
			Auth string `json:"auth"`
		} `json:"auths"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("failed to parse Docker config: %v", err)
	}

	authEntry, ok := config.Auths["https://index.docker.io/v1/"]
	if !ok || authEntry.Auth == "" {
		t.Skip("no DockerHub auth entry found, skipping test")
	}

	decoded, err := base64.StdEncoding.DecodeString(authEntry.Auth)
	if err != nil {
		t.Fatalf("failed to decode auth string: %v", err)
	}

	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		t.Fatalf("invalid auth format")
	}

	return parts[0]
}

func randomTag() string {
	return fmt.Sprintf("retag-%d", rand.Intn(1e9))
}

func dockerRmi(ref string) {
	_ = exec.Command("docker", "rmi", "-f", ref).Run()
}

func dockerPush(t *testing.T, ref string) {
	cmd := exec.Command("docker", "push", ref)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("docker push failed: %v\n%s", err, out)
	}
}

func verifySimpleImageExists(ref string, isIndex bool) error {
	// verify that ref points to a simple image, use google/go-containerregistry
	parsedRef, err := name.ParseReference(ref)
	if err != nil {
		return err
	}

	desc, err := Get(parsedRef)

	if err != nil {
		return err
	}

	if isIndex {
		if desc.MediaType == types.OCIImageIndex || desc.MediaType == types.DockerManifestList {
			return nil
		}
		return fmt.Errorf("expected index, got %s", desc.MediaType)
	}

	return nil
}

func TestRetag_SimpleImage(t *testing.T) {
	username := dockerUsername(t)
	repo := fmt.Sprintf("%s/mimosa-testing", username)
	origTag := randomTag()
	newTag := randomTag()
	origRef := fmt.Sprintf("%s:%s", repo, origTag)
	newRef := fmt.Sprintf("%s:%s", repo, newTag)

	// Build a simple scratch image
	dockerfile := "FROM scratch\nCOPY . ."
	tmpDir := t.TempDir()
	defer func() { _ = os.RemoveAll(tmpDir) }()
	dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
	simpleFile := filepath.Join(tmpDir, "simple")
	simpleFileContent := "hello world"
	if err := os.WriteFile(simpleFile, []byte(simpleFileContent), 0644); err != nil {
		t.Fatalf("failed to write simple.txt: %v", err)
	}
	if err := os.WriteFile(dockerfilePath, []byte(dockerfile), 0644); err != nil {
		t.Fatalf("failed to write Dockerfile: %v", err)
	}
	cmd := exec.Command("docker", "build", "-t", origRef, "-f", dockerfilePath, tmpDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("docker build failed: %v\n%s", err, out)
	}
	defer dockerRmi(origRef)
	defer dockerRmi(newRef)

	dockerPush(t, origRef)
	defer func() { _ = exec.Command("docker", "rmi", "-f", origRef).Run() }()

	// Test retag
	if err := Retag(origRef, newRef, false); err != nil {
		t.Fatalf("Retag failed: %v", err)
	}

	// Pull the new tag to verify it exists
	err = verifySimpleImageExists(newRef, false)
	if err != nil {
		t.Fatalf("failed to verify image: %v", err)
	}
}

func TestRetag_MultiPlatformManifestList(t *testing.T) {
	username := dockerUsername(t)
	repo := fmt.Sprintf("%s/mimosa-testing", username)
	origTag := randomTag()
	newTag := randomTag()
	origRef := fmt.Sprintf("%s:%s", repo, origTag)
	newRef := fmt.Sprintf("%s:%s", repo, newTag)

	tmpDir := t.TempDir()
	defer func() { _ = os.RemoveAll(tmpDir) }()
	dockerfileContent := "FROM scratch\nCOPY . ."
	dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
	simpleFilePath := filepath.Join(tmpDir, "simple.txt")
	simpleFileContent := "hello world"
	if err := os.WriteFile(simpleFilePath, []byte(simpleFileContent), 0644); err != nil {
		t.Fatalf("failed to write simple.txt: %v", err)
	}
	if err := os.WriteFile(dockerfilePath, []byte(dockerfileContent), 0644); err != nil {
		t.Fatalf("failed to write Dockerfile: %v", err)
	}

	// Build and push multi-platform image (linux/amd64, linux/arm64)
	cmd := exec.Command("docker", "buildx", "build",
		"--platform=linux/amd64,linux/arm64",
		"-t", origRef,
		"--push",
		"-f", dockerfilePath, tmpDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("docker buildx build failed: %v\n%s", err, out)
	}
	defer dockerRmi(origRef)
	defer dockerRmi(newRef)

	// Test retag
	if err := Retag(origRef, newRef, false); err != nil {
		t.Fatalf("Retag (manifest list) failed: %v", err)
	}

	// Pull the new tag to verify it exists for both platforms
	err = verifySimpleImageExists(newRef, true)
	if err != nil {
		t.Fatalf("failed to verify image: %v", err)
	}
}

func TestSimpleRetag_InvalidSource(t *testing.T) {
	err := SimpleRetag("invalid_ref/1/1: d", "valid_ref")

	if err == nil {
		t.Errorf("expected error, got nil")
	}
}

func TestSimpleRetag_InvalidDest(t *testing.T) {
	err := SimpleRetag("valid_ref", "invalid_ref/1/1: d")

	if err == nil {
		t.Errorf("expected error, got nil")
	}
}

func TestRetag_DryRun(t *testing.T) {
	if err := Retag(randomTag(), randomTag(), true); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}
