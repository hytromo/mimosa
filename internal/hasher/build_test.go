package hasher

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/hytromo/mimosa/internal/configuration"
	"github.com/stretchr/testify/assert"
)

func TestRegistryDomainsHash_EmptyInput(t *testing.T) {
	assert.Equal(t, registryDomainsHash([]string{}), "", "Expected empty hash for empty input")
}

func TestRegistryDomainsHash_SingleDomain(t *testing.T) {
	domains := []string{"index.docker.io"}
	assert.NotEqual(t, registryDomainsHash(domains), "", "Expected non-empty hash for single domain")
}

func TestRegistryDomainsHash_MultipleDomains_Deterministic(t *testing.T) {
	domains1 := []string{"index.docker.io", "gcr.io", "quay.io"}
	domains2 := []string{"quay.io", "index.docker.io", "gcr.io"}

	hash1 := registryDomainsHash(domains1)
	hash2 := registryDomainsHash(domains2)

	assert.Equal(t, hash1, hash2, "Expected same hash for same domains in different order")
}

func TestRegistryDomainsHash_DuplicateDomains(t *testing.T) {
	domains := []string{"index.docker.io", "gcr.io", "index.docker.io"}
	hash := registryDomainsHash(domains)
	assert.NotEqual(t, hash, "", "Expected non-empty hash for domains with duplicates")
	domainsNoDuplicates := []string{"index.docker.io", "gcr.io"}
	hashNoDuplicates := registryDomainsHash(domainsNoDuplicates)
	assert.Equal(t, hash, hashNoDuplicates, "Expected same hash for domains with duplicates")
}

func TestHashBuildCommand_EmptyCommand(t *testing.T) {
	hash := HashBuildCommand(DockerBuildCommand{})
	assert.NotEqual(t, hash, "", "Expected non-empty hash for empty command")
}

func TestHashBuildCommand_WithRegistryDomains(t *testing.T) {
	command := DockerBuildCommand{
		AllRegistryDomains:    []string{"index.docker.io", "gcr.io"},
		CmdWithTagPlaceholder: []string{"docker", "buildx", "build", "."},
	}
	hash := HashBuildCommand(command)
	assert.NotEqual(t, hash, "", "Expected non-empty hash for command with registry domains")
}

func TestHashBuildCommand_WithBuildContexts_Local(t *testing.T) {
	contextDir := t.TempDir()

	testFile := filepath.Join(contextDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	command := DockerBuildCommand{
		BuildContexts: map[string]string{
			configuration.MainBuildContextName: contextDir,
		},
		CmdWithTagPlaceholder: []string{"docker", "buildx", "build", "."},
	}
	hash := HashBuildCommand(command)
	assert.NotEqual(t, hash, "", "Expected non-empty hash for command with local build context")

	// change the file content:
	if err := os.WriteFile(testFile, []byte("test content 2"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	hash2 := HashBuildCommand(command)
	assert.NotEqual(t, hash, hash2, "Expected different hash for command with changed file content")
}

func TestHashBuildCommand_WithBuildContexts_Remote(t *testing.T) {
	command := DockerBuildCommand{
		BuildContexts: map[string]string{
			"remote": "https://github.com/user/repo.git",
		},
		CmdWithTagPlaceholder: []string{"docker", "buildx", "build", "."},
	}
	hash := HashBuildCommand(command)
	assert.NotEqual(t, hash, "", "Expected non-empty hash for command with remote build context")
	// expect the same hash for the same command without the remote context
	commandWithoutRemote := DockerBuildCommand{
		BuildContexts:         map[string]string{},
		CmdWithTagPlaceholder: []string{"docker", "buildx", "build", "."},
	}
	hashWithoutRemote := HashBuildCommand(commandWithoutRemote)
	assert.Equal(t, hash, hashWithoutRemote, "Expected same hash for command with and without remote build context")
}

func TestHashBuildCommand_WithBuildContexts_DockerImage(t *testing.T) {
	command := DockerBuildCommand{
		BuildContexts: map[string]string{
			"image": "docker-image://alpine:latest",
		},
		CmdWithTagPlaceholder: []string{"docker", "buildx", "build", "."},
	}
	hash := HashBuildCommand(command)
	assert.NotEqual(t, hash, "", "Expected non-empty hash for command with docker-image build context")
	// expect the same hash for the same command without the docker-image context
	commandWithoutDockerImage := DockerBuildCommand{
		BuildContexts:         map[string]string{},
		CmdWithTagPlaceholder: []string{"docker", "buildx", "build", "."},
	}
	hashWithoutDockerImage := HashBuildCommand(commandWithoutDockerImage)
	assert.Equal(t, hash, hashWithoutDockerImage, "Expected same hash for command with and without docker-image build context")
}

func TestHashBuildCommand_WithBuildContexts_OCILayout(t *testing.T) {
	command := DockerBuildCommand{
		BuildContexts: map[string]string{
			"oci": "oci-layout:///path/to/oci",
		},
		CmdWithTagPlaceholder: []string{"docker", "buildx", "build", "."},
	}
	hash := HashBuildCommand(command)
	assert.NotEqual(t, hash, "", "Expected non-empty hash for command with oci-layout build context")
	// expect the same hash for the same command without the oci-layout context
	commandWithoutOCILayout := DockerBuildCommand{
		BuildContexts:         map[string]string{},
		CmdWithTagPlaceholder: []string{"docker", "buildx", "build", "."},
	}
	hashWithoutOCILayout := HashBuildCommand(commandWithoutOCILayout)
	assert.Equal(t, hash, hashWithoutOCILayout, "Expected same hash for command with and without oci-layout build context")
}

func TestHashBuildCommand_WithBuildContexts_Mixed(t *testing.T) {
	dir := t.TempDir()

	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	command := DockerBuildCommand{
		BuildContexts: map[string]string{
			configuration.MainBuildContextName: dir,
			"remote":                           "https://github.com/user/repo.git",
			"image":                            "docker-image://alpine:latest",
		},
		CmdWithTagPlaceholder: []string{"docker", "buildx", "build", "."},
	}
	hash := HashBuildCommand(command)
	assert.NotEqual(t, hash, "", "Expected non-empty hash for command with mixed build contexts")
	// expect the same hash for the same command without the mixed contexts
	commandWithoutMixed := DockerBuildCommand{
		BuildContexts: map[string]string{
			configuration.MainBuildContextName: dir,
		},
		CmdWithTagPlaceholder: []string{"docker", "buildx", "build", "."},
	}
	hashWithoutMixed := HashBuildCommand(commandWithoutMixed)
	assert.Equal(t, hash, hashWithoutMixed, "Expected same hash for command with and without mixed build contexts")

	// add a file inside dir
	testFile2 := filepath.Join(dir, "test2.txt")
	if err := os.WriteFile(testFile2, []byte("test content 2"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	hash2 := HashBuildCommand(command)
	assert.NotEqual(t, hash, hash2, "Expected different hash for command with changed file content")
}

func TestHashBuildCommand_WithBuildContexts_Malformed(t *testing.T) {
	command := DockerBuildCommand{
		BuildContexts: map[string]string{
			"malformed": "invalid=context=path",
		},
		CmdWithTagPlaceholder: []string{"docker", "buildx", "build", "."},
	}
	hash := HashBuildCommand(command)
	assert.NotEqual(t, hash, "", "Expected non-empty hash for command with malformed build context")
}

func TestHashBuildCommand_WithDockerfileAndDockerignore(t *testing.T) {
	dir := t.TempDir()

	// Create Dockerfile
	dockerfile := filepath.Join(dir, "Dockerfile")
	if err := os.WriteFile(dockerfile, []byte("FROM alpine"), 0644); err != nil {
		t.Fatalf("Failed to create Dockerfile: %v", err)
	}

	// Create .dockerignore
	dockerignore := filepath.Join(dir, ".dockerignore")
	if err := os.WriteFile(dockerignore, []byte("*.tmp"), 0644); err != nil {
		t.Fatalf("Failed to create .dockerignore: %v", err)
	}

	command := DockerBuildCommand{
		DockerfilePath:   dockerfile,
		DockerignorePath: dockerignore,
		BuildContexts: map[string]string{
			configuration.MainBuildContextName: dir,
		},
		CmdWithTagPlaceholder: []string{"docker", "buildx", "build", "."},
	}
	hash := HashBuildCommand(command)
	assert.NotEqual(t, hash, "", "Expected non-empty hash for command with Dockerfile and .dockerignore")
	// create a file with .tmp extension and expect the same hash
	testFile := filepath.Join(dir, "test.tmp")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	hash2 := HashBuildCommand(command)
	assert.Equal(t, hash, hash2, "Expected same hash for command with and without .tmp file")
	// remove .dockerignore and expect different hash
	os.Remove(dockerignore)
	hash3 := HashBuildCommand(command)
	assert.NotEqual(t, hash, hash3, "Expected different hash for command without .dockerignore")
}

func TestHashBuildCommand_WithDockerfileOnly(t *testing.T) {
	dir := t.TempDir()

	// Create Dockerfile
	dockerfile := filepath.Join(dir, "Dockerfile")
	if err := os.WriteFile(dockerfile, []byte("FROM alpine"), 0644); err != nil {
		t.Fatalf("Failed to create Dockerfile: %v", err)
	}

	command := DockerBuildCommand{
		DockerfilePath: dockerfile,
		BuildContexts: map[string]string{
			configuration.MainBuildContextName: dir,
		},
		CmdWithTagPlaceholder: []string{"docker", "buildx", "build", "."},
	}
	hash := HashBuildCommand(command)
	assert.NotEqual(t, hash, "", "Expected non-empty hash for command with Dockerfile only")

	// add to Dockerfile and expect different hash
	if err := os.WriteFile(dockerfile, []byte("FROM alpine\nRUN echo 'test content' > test.txt"), 0644); err != nil {
		t.Fatalf("Failed to create Dockerfile: %v", err)
	}
	hash2 := HashBuildCommand(command)
	assert.NotEqual(t, hash, hash2, "Expected different hash for command with changed Dockerfile")
}

func TestHashBuildCommand_WithNonMainContext_NoDockerignore(t *testing.T) {
	dir := t.TempDir()

	command := DockerBuildCommand{
		BuildContexts: map[string]string{
			"frontend": dir,
		},
		CmdWithTagPlaceholder: []string{"docker", "buildx", "build", "."},
	}
	hash := HashBuildCommand(command)
	assert.NotEqual(t, hash, "", "Expected non-empty hash for command with non-main context without .dockerignore")
}

func TestHashBuildCommand_WithMultipleLocalContexts(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	// Create test files in both contexts
	testFile1 := filepath.Join(dir1, "test1.txt")
	if err := os.WriteFile(testFile1, []byte("test content 1"), 0644); err != nil {
		t.Fatalf("Failed to create test file 1: %v", err)
	}

	testFile2 := filepath.Join(dir2, "test2.txt")
	if err := os.WriteFile(testFile2, []byte("test content 2"), 0644); err != nil {
		t.Fatalf("Failed to create test file 2: %v", err)
	}

	command := DockerBuildCommand{
		BuildContexts: map[string]string{
			"frontend": dir1,
			"backend":  dir2,
		},
		CmdWithTagPlaceholder: []string{"docker", "buildx", "build", "."},
	}
	hash := HashBuildCommand(command)
	if hash == "" {
		t.Error("Expected non-empty hash for command with multiple local contexts")
	}
}

func TestHashBuildCommand_Deterministic(t *testing.T) {
	dir := t.TempDir()

	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	command := DockerBuildCommand{
		BuildContexts: map[string]string{
			configuration.MainBuildContextName: dir,
		},
		AllRegistryDomains:    []string{"index.docker.io"},
		CmdWithTagPlaceholder: []string{"docker", "buildx", "build", "."},
	}

	hash1 := HashBuildCommand(command)
	hash2 := HashBuildCommand(command)

	assert.Equal(t, hash1, hash2, "Expected same hash for same command")
}

func TestHashBuildCommand_DifferentCommands_DifferentHashes(t *testing.T) {
	dir := t.TempDir()

	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	command1 := DockerBuildCommand{
		BuildContexts: map[string]string{
			configuration.MainBuildContextName: dir,
		},
		CmdWithTagPlaceholder: []string{"docker", "buildx", "build", "."},
	}

	command2 := DockerBuildCommand{
		BuildContexts: map[string]string{
			configuration.MainBuildContextName: dir,
		},
		CmdWithTagPlaceholder: []string{"docker", "buildx", "build", "--no-cache", "."},
	}

	hash1 := HashBuildCommand(command1)
	hash2 := HashBuildCommand(command2)

	assert.NotEqual(t, hash1, hash2, "Expected different hashes for different commands")
}

func TestHashBuildCommand_DifferentRegistryDomains_DifferentHashes(t *testing.T) {
	dir := t.TempDir()

	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	command1 := DockerBuildCommand{
		BuildContexts: map[string]string{
			configuration.MainBuildContextName: dir,
		},
		AllRegistryDomains:    []string{"index.docker.io"},
		CmdWithTagPlaceholder: []string{"docker", "buildx", "build", "-t", "TAG", "--push", "."},
	}

	command2 := DockerBuildCommand{
		BuildContexts: map[string]string{
			configuration.MainBuildContextName: dir,
		},
		AllRegistryDomains:    []string{"gcr.io"},
		CmdWithTagPlaceholder: []string{"docker", "buildx", "build", "-t", "TAG", "--push", "."},
	}

	hash1 := HashBuildCommand(command1)
	hash2 := HashBuildCommand(command2)

	assert.NotEqual(t, hash1, hash2, "Expected different hashes for different registry domains")
}

func TestHashBuildCommand_WithLargeNumberOfContexts(t *testing.T) {
	// Test with more contexts than CPU cores to test worker pool behavior
	contexts := make(map[string]string)
	for i := 0; i < 100; i++ {
		dir := t.TempDir()
		testFile := filepath.Join(dir, "test.txt")
		if err := os.WriteFile(testFile, []byte("test content"+strconv.Itoa(i)), 0644); err != nil {
			t.Fatalf("Failed to create test file %d: %v", i, err)
		}
		contexts[fmt.Sprintf("context%d", i)] = dir
	}

	command := DockerBuildCommand{
		BuildContexts:         contexts,
		CmdWithTagPlaceholder: []string{"docker", "buildx", "build", "."},
	}
	hash := HashBuildCommand(command)
	assert.NotEqual(t, hash, "", "Expected non-empty hash for command with many contexts")
	hash2 := HashBuildCommand(command)
	assert.Equal(t, hash, hash2, "Expected same hash for command with many contexts")
}

func TestHashBuildCommand_WithContextContainingSpecialFiles(t *testing.T) {
	dir := t.TempDir()

	// Create various types of files
	files := map[string]string{
		"normal.txt":      "normal content",
		"hidden.txt":      "hidden content",
		"subdir/file.txt": "subdir content",
		".hidden":         "hidden file content",
	}

	for path, content := range files {
		fullPath := filepath.Join(dir, path)
		if strings.Contains(path, "/") {
			// Create subdirectory
			subdir := filepath.Dir(fullPath)
			if err := os.MkdirAll(subdir, 0755); err != nil {
				t.Fatalf("Failed to create subdirectory: %v", err)
			}
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", path, err)
		}
	}

	command := DockerBuildCommand{
		BuildContexts: map[string]string{
			configuration.MainBuildContextName: dir,
		},
		CmdWithTagPlaceholder: []string{"docker", "buildx", "build", "."},
	}
	hash := HashBuildCommand(command)
	assert.NotEqual(t, hash, "", "Expected non-empty hash for command with special files")
	// ignore subdir through a dockerignore file:
	dockerignore := filepath.Join(dir, ".dockerignore")
	if err := os.WriteFile(dockerignore, []byte("subdir/\n.dockerignore"), 0644); err != nil {
		t.Fatalf("Failed to create dockerignore file: %v", err)
	}
	hash2 := HashBuildCommand(command)
	assert.NotEqual(t, hash, hash2, "Expected same hash for command with special files and dockerignore")
	// but if we remove subdir we expect the same hash - as it is anyway ignored
	os.Remove(filepath.Join(dir, "subdir"))
	hash3 := HashBuildCommand(command)
	assert.Equal(t, hash2, hash3, "Expected same hash for command with special files and dockerignore")
}

func TestHashBuildCommand_WithNilCommandString(t *testing.T) {
	dir := t.TempDir()

	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	command := DockerBuildCommand{
		BuildContexts: map[string]string{
			configuration.MainBuildContextName: dir,
		},
		AllRegistryDomains:    []string{"index.docker.io"},
		CmdWithTagPlaceholder: nil,
	}
	hash := HashBuildCommand(command)
	assert.NotEqual(t, hash, "", "Expected non-empty hash for command with nil command string")
}

func TestHashBuildCommand_WithContextPathStartingWithEquals(t *testing.T) {
	command := DockerBuildCommand{
		BuildContexts: map[string]string{
			"=context": "=path",
		},
		CmdWithTagPlaceholder: []string{"docker", "buildx", "build", "."},
	}
	hash := HashBuildCommand(command)
	assert.NotEqual(t, hash, "", "Expected non-empty hash for command with context path starting with equals")
}

func TestHashBuildCommand_WithContextPathEndingWithEquals(t *testing.T) {
	command := DockerBuildCommand{
		BuildContexts: map[string]string{
			"context=": "path=",
		},
		CmdWithTagPlaceholder: []string{"docker", "buildx", "build", "."},
	}
	hash := HashBuildCommand(command)
	assert.NotEqual(t, hash, "", "Expected non-empty hash for command with context path ending with equals")
}

func TestHashBuildCommand_WithContextPathMultipleEquals(t *testing.T) {
	command := DockerBuildCommand{
		BuildContexts: map[string]string{
			"name=with=multiple=equals": "path=with=multiple=equals",
		},
		CmdWithTagPlaceholder: []string{"docker", "buildx", "build", "."},
	}
	hash := HashBuildCommand(command)
	assert.NotEqual(t, hash, "", "Expected non-empty hash for command with context path multiple equals")
}

func TestHashBuildCommand_WithContextPathSpecialCharacters(t *testing.T) {
	command := DockerBuildCommand{
		BuildContexts: map[string]string{
			"context-with-special-chars": "path/with/special/chars/and/spaces and more",
		},
		CmdWithTagPlaceholder: []string{"docker", "buildx", "build", "."},
	}
	hash := HashBuildCommand(command)
	assert.NotEqual(t, hash, "", "Expected non-empty hash for command with context path special characters")
}

func TestHashBuildCommand_WithContextPathUnicode(t *testing.T) {
	dir := t.TempDir()
	innerDirWithUnicode := filepath.Join(dir, "path/with/unicode/世界/привет")
	if err := os.MkdirAll(innerDirWithUnicode, 0755); err != nil {
		t.Fatalf("Failed to create inner directory: %v", err)
	}
	// create a dockerfile in the inner directory
	dockerfile := filepath.Join(innerDirWithUnicode, "Dockerfile")
	if err := os.WriteFile(dockerfile, []byte("FROM alpine"), 0644); err != nil {
		t.Fatalf("Failed to create dockerfile: %v", err)
	}
	command := DockerBuildCommand{
		BuildContexts: map[string]string{
			"context-with-unicode": innerDirWithUnicode,
		},
		CmdWithTagPlaceholder: []string{"docker", "buildx", "build", "."},
	}
	hash := HashBuildCommand(command)
	assert.NotEqual(t, hash, "", "Expected non-empty hash for command with context path unicode")

	// change the dockerfile and expect a different hash
	if err := os.WriteFile(dockerfile, []byte("FROM alpine:2"), 0644); err != nil {
		t.Fatalf("Failed to create dockerfile: %v", err)
	}
	hash2 := HashBuildCommand(command)
	assert.NotEqual(t, hash, hash2, "Expected different hash for command with changed dockerfile")
}

func TestHashBuildCommand_WithContextPathWhitespace(t *testing.T) {
	command := DockerBuildCommand{
		BuildContexts: map[string]string{
			"   context   ": "   path   ",
		},
		CmdWithTagPlaceholder: []string{"docker", "buildx", "build", "."},
	}
	hash := HashBuildCommand(command)
	assert.NotEqual(t, hash, "", "Expected non-empty hash for command with whitespace in context path")
}

func TestHashBuildCommand_WithContextPathNewlines(t *testing.T) {
	command := DockerBuildCommand{
		BuildContexts: map[string]string{
			"context\nwith\nnewlines": "path\nwith\nnewlines",
		},
		CmdWithTagPlaceholder: []string{"docker", "buildx", "build", "."},
	}
	hash := HashBuildCommand(command)
	assert.NotEqual(t, hash, "", "Expected non-empty hash for command with newlines in context path")
}

func TestHashBuildCommand_WithContextPathControlCharacters(t *testing.T) {
	command := DockerBuildCommand{
		BuildContexts: map[string]string{
			"context\x01\x02\x03with\x04\x05\x06control": "path\x01\x02\x03with\x04\x05\x06control",
		},
		CmdWithTagPlaceholder: []string{"docker", "buildx", "build", "."},
	}
	hash := HashBuildCommand(command)
	assert.NotEqual(t, hash, "", "Expected non-empty hash for command with control characters in context path")
}

func TestHashBuildCommand_WithContextPathBackslashes(t *testing.T) {
	command := DockerBuildCommand{
		BuildContexts: map[string]string{
			"context\\with\\backslashes": "path\\with\\backslashes",
		},
		CmdWithTagPlaceholder: []string{"docker", "buildx", "build", "."},
	}
	hash := HashBuildCommand(command)
	assert.NotEqual(t, hash, "", "Expected non-empty hash for command with backslashes in context path")
}
