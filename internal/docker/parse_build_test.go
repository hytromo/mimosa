package docker

import (
	"os"
	"strings"
	"testing"

	"github.com/hytromo/mimosa/internal/configuration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseBuildCommand_ValidCommand(t *testing.T) {
	testCases := []struct {
		name     string
		command  []string
		expected configuration.ParsedCommand
	}{
		{
			name:    "Simple build command",
			command: []string{"docker", "build", "-t", "myapp:latest", "."},
			expected: configuration.ParsedCommand{
				Command: []string{"docker", "build", "-t", "myapp:latest", "."},
				TagsByTarget: map[string][]string{
					"default": {"myapp:latest"},
				},
			},
		},
		{
			name:    "Build command with multiple tags",
			command: []string{"docker", "build", "-t", "myapp:latest", "-t", "myapp:v1.0.0", "."},
			expected: configuration.ParsedCommand{
				Command: []string{"docker", "build", "-t", "myapp:latest", "-t", "myapp:v1.0.0", "."},
				TagsByTarget: map[string][]string{
					"default": {"myapp:latest", "myapp:v1.0.0"},
				},
			},
		},
		{
			name:    "Build command with custom Dockerfile",
			command: []string{"docker", "build", "-f", "Dockerfile.prod", "-t", "myapp:latest", "."},
			expected: configuration.ParsedCommand{
				Command: []string{"docker", "build", "-f", "Dockerfile.prod", "-t", "myapp:latest", "."},
				TagsByTarget: map[string][]string{
					"default": {"myapp:latest"},
				},
			},
		},
		{
			name:    "Buildx command",
			command: []string{"docker", "buildx", "build", "-t", "myapp:latest", "."},
			expected: configuration.ParsedCommand{
				Command: []string{"docker", "buildx", "build", "-t", "myapp:latest", "."},
				TagsByTarget: map[string][]string{
					"default": {"myapp:latest"},
				},
			},
		},
		{
			name:    "Build command with build context",
			command: []string{"docker", "build", "--build-context", "backend", "./backend", "-t", "myapp:latest", "."},
			expected: configuration.ParsedCommand{
				Command: []string{"docker", "build", "--build-context", "backend", "./backend", "-t", "myapp:latest", "."},
				TagsByTarget: map[string][]string{
					"default": {"myapp:latest"},
				},
			},
		},
		{
			name:    "Build command with equals syntax",
			command: []string{"docker", "build", "--tag=myapp:latest", "--file=Dockerfile.prod", "."},
			expected: configuration.ParsedCommand{
				Command: []string{"docker", "build", "--tag=myapp:latest", "--file=Dockerfile.prod", "."},
				TagsByTarget: map[string][]string{
					"default": {"myapp:latest"},
				},
			},
		},
		{
			name:    "Build command with short flags equals syntax",
			command: []string{"docker", "build", "-t=myapp:latest", "-f=Dockerfile.prod", "."},
			expected: configuration.ParsedCommand{
				Command: []string{"docker", "build", "-t=myapp:latest", "-f=Dockerfile.prod", "."},
				TagsByTarget: map[string][]string{
					"default": {"myapp:latest"},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()

			originalWd, err := os.Getwd()
			require.NoError(t, err)
			defer func() { _ = os.Chdir(originalWd) }()
			err = os.Chdir(tempDir)
			require.NoError(t, err)

			dockerfilePath := "Dockerfile"
			if tc.name == "Build command with custom Dockerfile" || tc.name == "Build command with equals syntax" || tc.name == "Build command with short flags equals syntax" {
				dockerfilePath = "Dockerfile.prod"
			}
			err = os.WriteFile(dockerfilePath, []byte("FROM alpine:latest"), 0644)
			require.NoError(t, err)

			result, err := ParseBuildCommand(tc.command)
			require.NoError(t, err)

			assert.Equal(t, tc.expected.Command, result.Command)
			assert.Equal(t, tc.expected.TagsByTarget, result.TagsByTarget)
			assert.NotEmpty(t, result.Hash)
		})
	}
}

func TestParseBuildCommand_InvalidCommands(t *testing.T) {
	testCases := []struct {
		name        string
		command     []string
		expectedErr string
	}{
		{
			name:        "Empty command",
			command:     []string{},
			expectedErr: "not enough arguments for a docker build command",
		},
		{
			name:        "Single argument",
			command:     []string{"docker"},
			expectedErr: "not enough arguments for a docker build command",
		},
		{
			name:        "Wrong executable",
			command:     []string{"podman", "build", "-t", "myapp:latest", "."},
			expectedErr: "only 'docker' executable is supported for caching",
		},
		{
			name:        "Wrong subcommand",
			command:     []string{"docker", "run", "-t", "myapp:latest", "."},
			expectedErr: "only image building is supported",
		},
		{
			name:        "No tag specified",
			command:     []string{"docker", "build", "."},
			expectedErr: "cannot find image tag using the -t or --tag option",
		},
		{
			name:        "No context path",
			command:     []string{"docker", "build", "-t", "myapp:latest"},
			expectedErr: "context path not found",
		},
		{
			name:        "No context path - buildx",
			command:     []string{"docker", "buildx", "build", "-t", "myapp:latest"},
			expectedErr: "context path not found",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseBuildCommand(tc.command)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.expectedErr)
		})
	}
}

func TestExtractBuildFlags(t *testing.T) {
	testCases := []struct {
		name                   string
		args                   []string
		expectedTags           []string
		expectedBuildContexts  map[string]string
		expectedDockerfilePath string
		expectError            bool
		expectedErrorContains  string
	}{
		{
			name:                   "Simple tags",
			args:                   []string{"build", "-t", "myapp:latest", "."},
			expectedTags:           []string{"myapp:latest"},
			expectedBuildContexts:  map[string]string{},
			expectedDockerfilePath: "",
		},
		{
			name:                   "Multiple tags",
			args:                   []string{"build", "-t", "myapp:latest", "-t", "myapp:v1.0.0", "."},
			expectedTags:           []string{"myapp:latest", "myapp:v1.0.0"},
			expectedBuildContexts:  map[string]string{},
			expectedDockerfilePath: "",
		},
		{
			name:                   "Tags with equals syntax",
			args:                   []string{"build", "--tag=myapp:latest", "--tag=myapp:v1.0.0", "."},
			expectedTags:           []string{"myapp:latest", "myapp:v1.0.0"},
			expectedBuildContexts:  map[string]string{},
			expectedDockerfilePath: "",
		},
		{
			name:                   "Short tag with equals syntax",
			args:                   []string{"build", "-t=myapp:latest", "."},
			expectedTags:           []string{"myapp:latest"},
			expectedBuildContexts:  map[string]string{},
			expectedDockerfilePath: "",
		},
		{
			name:                   "With Dockerfile path",
			args:                   []string{"build", "-f", "Dockerfile.prod", "-t", "myapp:latest", "."},
			expectedTags:           []string{"myapp:latest"},
			expectedBuildContexts:  map[string]string{},
			expectedDockerfilePath: "Dockerfile.prod",
		},
		{
			name:                   "Dockerfile with equals syntax",
			args:                   []string{"build", "--file=Dockerfile.prod", "-t", "myapp:latest", "."},
			expectedTags:           []string{"myapp:latest"},
			expectedBuildContexts:  map[string]string{},
			expectedDockerfilePath: "Dockerfile.prod",
		},
		{
			name:                   "Short Dockerfile with equals syntax",
			args:                   []string{"build", "-f=Dockerfile.prod", "-t", "myapp:latest", "."},
			expectedTags:           []string{"myapp:latest"},
			expectedBuildContexts:  map[string]string{},
			expectedDockerfilePath: "Dockerfile.prod",
		},
		{
			name:                   "With build context",
			args:                   []string{"build", "--build-context", "backend=./backend", "-t", "myapp:latest", "."},
			expectedTags:           []string{"myapp:latest"},
			expectedBuildContexts:  map[string]string{"backend": "./backend"},
			expectedDockerfilePath: "",
		},
		{
			name:                   "No tags",
			args:                   []string{"build", "."},
			expectedTags:           nil,
			expectedBuildContexts:  map[string]string{},
			expectedDockerfilePath: "",
			expectError:            true,
			expectedErrorContains:  "cannot find image tag",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tags, buildContexts, dockerfilePath, err := extractBuildFlags(tc.args)

			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrorContains)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedTags, tags)
				assert.Equal(t, tc.expectedBuildContexts, buildContexts)
				assert.Equal(t, tc.expectedDockerfilePath, dockerfilePath)
			}
		})
	}
}

func TestFindContextPath(t *testing.T) {
	testCases := []struct {
		name          string
		args          []string
		expectedPath  string
		expectError   bool
		expectedError string
	}{
		{
			name:         "Simple context path",
			args:         []string{"docker", "build", "-t", "myapp:latest", "."},
			expectedPath: ".",
		},
		{
			name:         "Context path with directory",
			args:         []string{"docker", "build", "-t", "myapp:latest", "./src"},
			expectedPath: "./src",
		},
		{
			name:         "Context path with absolute path",
			args:         []string{"docker", "build", "-t", "myapp:latest", "/tmp/build"},
			expectedPath: "/tmp/build",
		},
		{
			name:         "Context path after flags",
			args:         []string{"docker", "build", "-f", "Dockerfile.prod", "-t", "myapp:latest", "."},
			expectedPath: ".",
		},
		{
			name:         "Context path with buildx",
			args:         []string{"docker", "buildx", "build", "-t", "myapp:latest", "."},
			expectedPath: ".",
		},
		{
			name:         "Context path with build context",
			args:         []string{"docker", "build", "--build-context", "backend=./backend", "-t", "myapp:latest", "."},
			expectedPath: ".",
		},
		{
			name:         "Context path with build context after boolean flag",
			args:         []string{"docker", "build", "--build-context", "backend=./backend", "-t", "myapp:latest", "--push", "./docs"},
			expectedPath: "./docs",
		},
		{
			name:          "No context path",
			args:          []string{"docker", "build", "-t", "myapp:latest"},
			expectError:   true,
			expectedError: "context path not found",
		},
		{
			name:          "Only flags",
			args:          []string{"docker", "build", "-t", "myapp:latest", "--no-cache"},
			expectError:   true,
			expectedError: "context path not found",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			path, err := findContextPath(tc.args)

			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedPath, path)
			}
		})
	}
}

func TestNormalizeCommandForHashing(t *testing.T) {
	// Shared expected output for GitHub Actions commands with different temp paths
	githubActionsExpected := []string{
		"docker", "buildx", "build",
		"--attest", "type=provenance,mode=min,inline-only=true,builder-id=<VALUE>",
		"--file",
		"--iidfile", "<VALUE>",
		"--metadata-file", "<VALUE>",
		"--platform",
		"--push",
		"--secret", "id=ARTIFACTORY_PASSWORD,src=<VALUE>",
		"--secret", "id=ARTIFACTORY_USER,src=<VALUE>",
		"--tag", "<VALUE>",
		"--target",
		".",
		"Dockerfile.gpu",
		"application",
		"linux/amd64",
	}

	// Shared expected output for GitHub Actions commands with labels - label values are templated
	githubActionsWithLabelsExpected := []string{
		"docker", "buildx", "build",
		"--attest", "type=provenance,mode=min,inline-only=true,builder-id=<VALUE>",
		"--file",
		"--iidfile", "<VALUE>",
		"--label", "org.opencontainers.image.created=<VALUE>",
		"--label", "org.opencontainers.image.description=<VALUE>",
		"--label", "org.opencontainers.image.licenses=<VALUE>",
		"--label", "org.opencontainers.image.revision=<VALUE>",
		"--label", "org.opencontainers.image.source=<VALUE>",
		"--label", "org.opencontainers.image.title=<VALUE>",
		"--label", "org.opencontainers.image.url=<VALUE>",
		"--label", "org.opencontainers.image.version=<VALUE>",
		"--metadata-file", "<VALUE>",
		"--platform",
		"--push",
		"--secret", "id=ARTIFACTORY_PASSWORD,src=<VALUE>",
		"--secret", "id=ARTIFACTORY_USER,src=<VALUE>",
		"--tag", "<VALUE>",
		"--target",
		".",
		"Dockerfile.gpu",
		"application",
		"linux/amd64",
	}

	testCases := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "Simple tag templating",
			input:    []string{"docker", "build", "-t", "myapp:latest", "."},
			expected: []string{"docker", "build", "-t", "<VALUE>", "."},
		},
		{
			name:     "Multiple tags templating",
			input:    []string{"docker", "build", "-t", "myapp:latest", "-t", "myapp:v1.0.0", "."},
			expected: []string{"docker", "build", "-t", "<VALUE>", "-t", "<VALUE>", "."},
		},
		{
			name:     "Tag with equals syntax",
			input:    []string{"docker", "build", "--tag=myapp:latest", "."},
			expected: []string{"docker", "build", "--tag=<VALUE>", "."},
		},
		{
			name:     "Short tag with equals syntax",
			input:    []string{"docker", "build", "-t=myapp:latest", "."},
			expected: []string{"docker", "build", "-t=<VALUE>", "."},
		},
		{
			name:     "No tag in command",
			input:    []string{"docker", "build", "."},
			expected: []string{"docker", "build", "."},
		},
		{
			name:     "iidfile templating",
			input:    []string{"docker", "build", "--iidfile", "/tmp/random123.txt", "-t", "myapp:latest", "."},
			expected: []string{"docker", "build", "--iidfile", "<VALUE>", "-t", "<VALUE>", "."},
		},
		{
			name:     "iidfile with equals syntax",
			input:    []string{"docker", "build", "--iidfile=/tmp/random.txt", "-t", "myapp:latest", "."},
			expected: []string{"docker", "build", "--iidfile=<VALUE>", "-t", "<VALUE>", "."},
		},
		{
			name:     "metadata-file templating",
			input:    []string{"docker", "build", "--metadata-file", "/tmp/metadata.json", "-t", "myapp:latest", "."},
			expected: []string{"docker", "build", "--metadata-file", "<VALUE>", "-t", "<VALUE>", "."},
		},
		{
			name:     "metadata-file with equals syntax",
			input:    []string{"docker", "build", "--metadata-file=/tmp/metadata.json", "-t", "myapp:latest", "."},
			expected: []string{"docker", "build", "--metadata-file=<VALUE>", "-t", "<VALUE>", "."},
		},
		{
			name:     "attest with builder-id templating",
			input:    []string{"docker", "build", "--attest", "type=provenance,mode=max,builder-id=https://github.com/example/actions/runs/123", "-t", "myapp:latest", "."},
			expected: []string{"docker", "build", "--attest", "type=provenance,mode=max,builder-id=<VALUE>", "-t", "<VALUE>", "."},
		},
		{
			name:     "attest with equals syntax and builder-id",
			input:    []string{"docker", "build", "--attest=type=provenance,builder-id=https://example.com/run/456", "-t", "myapp:latest", "."},
			expected: []string{"docker", "build", "--attest=type=provenance,builder-id=<VALUE>", "-t", "<VALUE>", "."},
		},
		{
			name:     "attest without builder-id unchanged",
			input:    []string{"docker", "build", "--attest", "type=sbom,generator=image", "-t", "myapp:latest", "."},
			expected: []string{"docker", "build", "--attest", "type=sbom,generator=image", "-t", "<VALUE>", "."},
		},
		{
			name:     "buildx command",
			input:    []string{"docker", "buildx", "build", "-t", "myapp:latest", "."},
			expected: []string{"docker", "buildx", "build", "-t", "<VALUE>", "."},
		},
		{
			name:     "buildx with multiple templated flags",
			input:    []string{"docker", "buildx", "build", "--iidfile", "/tmp/id.txt", "--metadata-file", "/tmp/meta.json", "-t", "myapp:latest", "."},
			expected: []string{"docker", "buildx", "build", "--iidfile", "<VALUE>", "--metadata-file", "<VALUE>", "-t", "<VALUE>", "."},
		},
		// Boolean flags to discard tests
		{
			name:     "quiet flag discarded",
			input:    []string{"docker", "build", "--quiet", "-t", "myapp:latest", "."},
			expected: []string{"docker", "build", "-t", "<VALUE>", "."},
		},
		{
			name:     "short quiet flag discarded",
			input:    []string{"docker", "build", "-q", "-t", "myapp:latest", "."},
			expected: []string{"docker", "build", "-t", "<VALUE>", "."},
		},
		{
			name:     "debug flag discarded",
			input:    []string{"docker", "build", "--debug", "-t", "myapp:latest", "."},
			expected: []string{"docker", "build", "-t", "<VALUE>", "."},
		},
		{
			name:     "short debug flag discarded",
			input:    []string{"docker", "build", "-D", "-t", "myapp:latest", "."},
			expected: []string{"docker", "build", "-t", "<VALUE>", "."},
		},
		{
			name:     "multiple boolean flags discarded",
			input:    []string{"docker", "build", "--quiet", "--debug", "-t", "myapp:latest", "."},
			expected: []string{"docker", "build", "-t", "<VALUE>", "."},
		},
		{
			name:     "boolean flags mixed with templated flags",
			input:    []string{"docker", "build", "-q", "--iidfile", "/tmp/id.txt", "-D", "-t", "myapp:latest", "."},
			expected: []string{"docker", "build", "--iidfile", "<VALUE>", "-t", "<VALUE>", "."},
		},
		{
			name:     "buildx with boolean and templated flags",
			input:    []string{"docker", "buildx", "build", "--quiet", "--progress", "plain", "-t", "myapp:latest", "."},
			expected: []string{"docker", "buildx", "build", "--progress", "<VALUE>", "-t", "<VALUE>", "."},
		},
		// Label tests
		{
			name:     "label with space-separated format",
			input:    []string{"docker", "build", "--label", "version=1.2.3", "-t", "myapp:latest", "."},
			expected: []string{"docker", "build", "--label", "version=<VALUE>", "-t", "<VALUE>", "."},
		},
		{
			name:     "label with equals format",
			input:    []string{"docker", "build", "--label=version=1.2.3", "-t", "myapp:latest", "."},
			expected: []string{"docker", "build", "--label=version=<VALUE>", "-t", "<VALUE>", "."},
		},
		{
			name:     "multiple labels with same keys different values",
			input:    []string{"docker", "build", "--label", "version=1.2.3", "--label", "version=2.0.0", "-t", "myapp:latest", "."},
			expected: []string{"docker", "build", "--label", "version=<VALUE>", "--label", "version=<VALUE>", "-t", "<VALUE>", "."},
		},
		{
			name:     "multiple labels with different keys",
			input:    []string{"docker", "build", "--label", "version=1.2.3", "--label", "build=123", "-t", "myapp:latest", "."},
			expected: []string{"docker", "build", "--label", "build=<VALUE>", "--label", "version=<VALUE>", "-t", "<VALUE>", "."},
		},
		// Secret tests
		{
			name:     "secret with src path",
			input:    []string{"docker", "build", "--secret", "id=mysecret,src=/path/to/file", "-t", "myapp:latest", "."},
			expected: []string{"docker", "build", "--secret", "id=mysecret,src=<VALUE>", "-t", "<VALUE>", "."},
		},
		{
			name:     "secret with env var",
			input:    []string{"docker", "build", "--secret", "id=mysecret,env=MY_SECRET", "-t", "myapp:latest", "."},
			expected: []string{"docker", "build", "--secret", "id=mysecret,env=<VALUE>", "-t", "<VALUE>", "."},
		},
		{
			name:     "secret with equals format",
			input:    []string{"docker", "build", "--secret=id=mysecret,src=/path/to/file", "-t", "myapp:latest", "."},
			expected: []string{"docker", "build", "--secret=id=mysecret,src=<VALUE>", "-t", "<VALUE>", "."},
		},
		{
			name:     "secret with both src and env",
			input:    []string{"docker", "build", "--secret", "id=mysecret,src=/path,env=VAR", "-t", "myapp:latest", "."},
			expected: []string{"docker", "build", "--secret", "id=mysecret,src=<VALUE>,env=<VALUE>", "-t", "<VALUE>", "."},
		},
		// Real-world GitHub Actions example - two different runs should normalize to the same output
		{
			name: "GitHub Actions buildx command with different temp paths and tags",
			input: []string{
				"docker", "buildx", "build",
				"--file", "Dockerfile.gpu",
				"--iidfile", "/home/runner/_work/_temp/docker-actions-toolkit-6E7XsB/build-iidfile-e7944d5539.txt",
				"--platform", "linux/amd64",
				"--attest", "type=provenance,mode=min,inline-only=true,builder-id=https://github.com/my-org/my-service/actions/runs/20918998792/attempts/1",
				"--secret", "id=ARTIFACTORY_USER,src=/home/runner/_work/_temp/docker-actions-toolkit-6E7XsB/tmp-1201-5HJKc4nPygdt",
				"--secret", "id=ARTIFACTORY_PASSWORD,src=/home/runner/_work/_temp/docker-actions-toolkit-6E7XsB/tmp-1201-sxPnEwpt8v1i",
				"--tag", "abc.dkr.ecr.us-east-1.amazonaws.com/my-service:tag1",
				"--target", "application",
				"--metadata-file", "/home/runner/_work/_temp/docker-actions-toolkit-6E7XsB/build-metadata-abcdef.json",
				"--push", ".",
			},
			expected: githubActionsExpected,
		},
		{
			name: "GitHub Actions buildx command with different temp paths and tags (second run)",
			input: []string{
				"docker", "buildx", "build",
				"--file", "Dockerfile.gpu",
				"--iidfile", "/home/runner/_work/_temp/docker-actions-toolkit-DZMjnc/build-iidfile-299ebe6141.txt",
				"--platform", "linux/amd64",
				"--attest", "type=provenance,mode=min,inline-only=true,builder-id=https://github.com/my-org/my-service/actions/runs/20919355217/attempts/1",
				"--secret", "id=ARTIFACTORY_USER,src=/home/runner/_work/_temp/docker-actions-toolkit-DZMjnc/tmp-1203-L5fG1HhXCT97",
				"--secret", "id=ARTIFACTORY_PASSWORD,src=/home/runner/_work/_temp/docker-actions-toolkit-DZMjnc/tmp-1203-bAi2R7ModuG6",
				"--tag", "abc.dkr.ecr.us-east-1.amazonaws.com/my-service:tag2",
				"--target", "application",
				"--metadata-file", "/home/runner/_work/_temp/docker-actions-toolkit-DZMjnc/build-metadata-abcd12.json",
				"--push", ".",
			},
			expected: githubActionsExpected,
		},
		// Real-world GitHub Actions example with labels - different label values should normalize to the same output
		{
			name: "GitHub Actions buildx command with labels (first run)",
			input: []string{
				"docker", "buildx", "build",
				"--file", "Dockerfile.gpu",
				"--iidfile", "/home/runner/_work/_temp/docker-actions-toolkit-Ro2ZIP/build-iidfile-4506fa0971.txt",
				"--label", "org.opencontainers.image.created=2026-01-12T12:05:03.700Z",
				"--label", "org.opencontainers.image.description=Repository for my-service service",
				"--label", "org.opencontainers.image.licenses=",
				"--label", "org.opencontainers.image.revision=f8c5acd1d51b19089edef3c61690a8c5780fea99",
				"--label", "org.opencontainers.image.source=https://github.com/my-org/my-service",
				"--label", "org.opencontainers.image.title=my-service",
				"--label", "org.opencontainers.image.url=https://github.com/my-org/my-service",
				"--label", "org.opencontainers.image.version=pr-191",
				"--platform", "linux/amd64",
				"--attest", "type=provenance,mode=min,inline-only=true,builder-id=https://github.com/my-org/my-service/actions/runs/20918681354/attempts/1",
				"--secret", "id=ARTIFACTORY_USER,src=/home/runner/_work/_temp/docker-actions-toolkit-Ro2ZIP/tmp-1219-pzokgcN55JNl",
				"--secret", "id=ARTIFACTORY_PASSWORD,src=/home/runner/_work/_temp/docker-actions-toolkit-Ro2ZIP/tmp-1219-CfqPonb1pG8F",
				"--tag", "abc.dkr.ecr.us-east-1.amazonaws.com/my-service:tag1",
				"--target", "application",
				"--metadata-file", "/home/runner/_work/_temp/docker-actions-toolkit-Ro2ZIP/build-metadata-7c16b0d3f6.json",
				"--push", ".",
			},
			expected: githubActionsWithLabelsExpected,
		},
		{
			name: "GitHub Actions buildx command with labels (different label values)",
			input: []string{
				"docker", "buildx", "build",
				"--file", "Dockerfile.gpu",
				"--iidfile", "/home/runner/_work/_temp/docker-actions-toolkit-OtherID/build-iidfile-different.txt",
				"--label", "org.opencontainers.image.created=2026-01-13T15:30:45.123Z",
				"--label", "org.opencontainers.image.description=Different description",
				"--label", "org.opencontainers.image.licenses=MIT",
				"--label", "org.opencontainers.image.revision=abc123def456",
				"--label", "org.opencontainers.image.source=https://github.com/my-org/my-service",
				"--label", "org.opencontainers.image.title=my-service",
				"--label", "org.opencontainers.image.url=https://github.com/my-org/my-service",
				"--label", "org.opencontainers.image.version=pr-192",
				"--platform", "linux/amd64",
				"--attest", "type=provenance,mode=min,inline-only=true,builder-id=https://github.com/my-org/my-service/actions/runs/99999999999/attempts/1",
				"--secret", "id=ARTIFACTORY_USER,src=/home/runner/_work/_temp/docker-actions-toolkit-OtherID/tmp-different1",
				"--secret", "id=ARTIFACTORY_PASSWORD,src=/home/runner/_work/_temp/docker-actions-toolkit-OtherID/tmp-different2",
				"--tag", "abc.dkr.ecr.us-east-1.amazonaws.com/my-service:tag2",
				"--target", "application",
				"--metadata-file", "/home/runner/_work/_temp/docker-actions-toolkit-OtherID/build-metadata-different.json",
				"--push", ".",
			},
			expected: githubActionsWithLabelsExpected,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := normalizeCommandForHashing(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestNormalizeCommandForHashing_OrderIndependence(t *testing.T) {
	// Commands with same flags in different order should produce identical normalized output
	testCases := []struct {
		name   string
		input1 []string
		input2 []string
	}{
		{
			name:   "Simple flag reordering",
			input1: []string{"docker", "build", "-t", "myapp:latest", "--platform", "linux/amd64", "."},
			input2: []string{"docker", "build", "--platform", "linux/amd64", "-t", "myapp:latest", "."},
		},
		{
			name:   "Multiple flags reordering",
			input1: []string{"docker", "build", "-t", "myapp:latest", "--iidfile", "/tmp/a.txt", "--push", "."},
			input2: []string{"docker", "build", "--push", "--iidfile", "/tmp/b.txt", "-t", "other:tag", "."},
		},
		{
			name:   "Buildx with complex reordering",
			input1: []string{"docker", "buildx", "build", "--metadata-file", "/path/1.json", "--platform", "linux/amd64,linux/arm64", "-t", "img:v1", "."},
			input2: []string{"docker", "buildx", "build", "-t", "img:v2", "--platform", "linux/amd64,linux/arm64", "--metadata-file", "/path/2.json", "."},
		},
		{
			name:   "With and without quiet flag",
			input1: []string{"docker", "build", "--quiet", "-t", "myapp:latest", "--platform", "linux/amd64", "."},
			input2: []string{"docker", "build", "-t", "myapp:latest", "--platform", "linux/amd64", "."},
		},
		{
			name:   "With and without debug flag",
			input1: []string{"docker", "build", "-D", "-t", "myapp:latest", "--platform", "linux/amd64", "."},
			input2: []string{"docker", "build", "-t", "myapp:latest", "--platform", "linux/amd64", "."},
		},
		{
			name:   "Different boolean flags produce same result",
			input1: []string{"docker", "build", "--quiet", "-t", "myapp:latest", "--platform", "linux/amd64", "."},
			input2: []string{"docker", "build", "--debug", "-t", "myapp:latest", "--platform", "linux/amd64", "."},
		},
		{
			name:   "Boolean flags in different positions",
			input1: []string{"docker", "build", "-q", "-t", "myapp:latest", "--platform", "linux/amd64", "."},
			input2: []string{"docker", "build", "-t", "myapp:latest", "-q", "--platform", "linux/amd64", "."},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result1 := normalizeCommandForHashing(tc.input1)
			result2 := normalizeCommandForHashing(tc.input2)
			assert.Equal(t, result1, result2, "Commands with same flags in different order should normalize to the same result")
		})
	}
}

func TestNormalizeCommandForHashing_GitHubActionsExample(t *testing.T) {
	// These are the actual commands from the GitHub Actions issue
	// They should produce identical normalized output
	cmd1 := []string{
		"docker", "buildx", "build",
		"--iidfile", "/home/runner/work/_temp/docker-actions-toolkit-FfxZzf/build-iidfile-beb46e5a7d.txt",
		"--platform", "linux/amd64,linux/arm64",
		"--attest", "type=provenance,mode=max,builder-id=https://github.com/hytromo/mimosa/actions/runs/20193832931/attempts/1",
		"--tag", "hytromo/mimosa-testing:recommended-example-cache-hit-c7ad46653a914718bf8e31f484f69614552e92e8",
		"--metadata-file", "/home/runner/work/_temp/docker-actions-toolkit-FfxZzf/build-metadata-98d5a7c1b3.json",
		"--push",
		"docs/gh-actions/actions-example",
	}

	cmd2 := []string{
		"docker", "buildx", "build",
		"--iidfile", "/home/runner/work/_temp/docker-actions-toolkit-BtGuR6/build-iidfile-81bd4a8cf4.txt",
		"--platform", "linux/amd64,linux/arm64",
		"--attest", "type=provenance,mode=max,builder-id=https://github.com/hytromo/mimosa/actions/runs/20193832931/attempts/1",
		"--tag", "hytromo/mimosa-testing:recommended-example-c7ad46653a914718bf8e31f484f69614552e92e8",
		"--metadata-file", "/home/runner/work/_temp/docker-actions-toolkit-BtGuR6/build-metadata-7bb018a1cf.json",
		"--push",
		"docs/gh-actions/actions-example",
	}

	result1 := normalizeCommandForHashing(cmd1)
	result2 := normalizeCommandForHashing(cmd2)

	assert.Equal(t, result1, result2, "GitHub Actions example commands should normalize to the same result")

	// Verify the normalized output contains expected flags
	normalizedStr := strings.Join(result1, " ")
	assert.Contains(t, normalizedStr, "--iidfile")
	assert.Contains(t, normalizedStr, "<VALUE>")
	assert.Contains(t, normalizedStr, "--attest")
	assert.Contains(t, normalizedStr, "type=provenance,mode=max,builder-id=<VALUE>")
}

func TestTemplateSubKeys(t *testing.T) {
	testCases := []struct {
		name     string
		value    string
		subKeys  []string
		expected string
	}{
		{
			name:     "Template builder-id at end",
			value:    "type=provenance,mode=max,builder-id=https://github.com/example/runs/123",
			subKeys:  []string{"builder-id"},
			expected: "type=provenance,mode=max,builder-id=<VALUE>",
		},
		{
			name:     "Template builder-id in middle",
			value:    "type=provenance,builder-id=https://example.com,mode=max",
			subKeys:  []string{"builder-id"},
			expected: "type=provenance,builder-id=<VALUE>,mode=max",
		},
		{
			name:     "Template builder-id at start",
			value:    "builder-id=https://example.com,type=provenance",
			subKeys:  []string{"builder-id"},
			expected: "builder-id=<VALUE>,type=provenance",
		},
		{
			name:     "No matching subkey",
			value:    "type=provenance,mode=max",
			subKeys:  []string{"builder-id"},
			expected: "type=provenance,mode=max",
		},
		{
			name:     "Multiple subkeys",
			value:    "type=provenance,builder-id=url1,secret=abc123",
			subKeys:  []string{"builder-id", "secret"},
			expected: "type=provenance,builder-id=<VALUE>,secret=<VALUE>",
		},
		{
			name:     "Empty value",
			value:    "",
			subKeys:  []string{"builder-id"},
			expected: "",
		},
		{
			name:     "Empty subkeys",
			value:    "type=provenance,builder-id=url",
			subKeys:  []string{},
			expected: "type=provenance,builder-id=url",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := templateSubKeys(tc.value, tc.subKeys)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestBuildCmdWithoutTagArguments tests backward compatibility
func TestBuildCmdWithoutTagArguments(t *testing.T) {
	// This function should delegate to normalizeCommandForHashing
	input := []string{"docker", "build", "-t", "myapp:latest", "."}
	result := buildCommandWithoutTagArguments(input)
	expected := normalizeCommandForHashing(input)
	assert.Equal(t, expected, result)
}

// TestNormalizeCommandForHashing_ShortFlagEqualsWithSubKeys tests the branch where
// a short flag with equals syntax has subKeys defined for partial templating.
func TestNormalizeCommandForHashing_ShortFlagEqualsWithSubKeys(t *testing.T) {
	// Save original and restore after test
	original := flagsToTemplate
	defer func() { flagsToTemplate = original }()

	// Add a test flag with both shortFlag and subKeys
	flagsToTemplate = append(flagsToTemplate, flagTemplate{
		longFlag:  "--test-attest",
		shortFlag: "-a",
		subKeys:   []string{"builder-id"},
	})

	input := []string{"docker", "build", "-a=type=provenance,builder-id=https://example.com", "-t", "myapp:latest", "."}
	result := normalizeCommandForHashing(input)

	// The short flag with equals should have its subKey templated
	assert.Contains(t, result, "-a=type=provenance,builder-id=<VALUE>")
}

func TestParseBuildCommand_DockerignoreHandling(t *testing.T) {
	tempDir := t.TempDir()

	originalWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalWd) }()
	err = os.Chdir(tempDir)
	require.NoError(t, err)

	err = os.WriteFile("Dockerfile", []byte("FROM alpine:latest"), 0644)
	require.NoError(t, err)

	err = os.WriteFile(".dockerignore", []byte("*.log\nnode_modules"), 0644)
	require.NoError(t, err)

	command := []string{"docker", "build", "-t", "myapp:latest", "."}

	result, err := ParseBuildCommand(command)
	require.NoError(t, err)

	assert.Equal(t, command, result.Command)
	assert.NotEmpty(t, result.Hash)
}

func TestParseBuildCommand_CustomDockerignoreHandling(t *testing.T) {
	tempDir := t.TempDir()

	originalWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalWd) }()
	err = os.Chdir(tempDir)
	require.NoError(t, err)

	err = os.WriteFile("Dockerfile", []byte("FROM alpine:latest"), 0644)
	require.NoError(t, err)

	err = os.WriteFile("Dockerfile.prod", []byte("FROM alpine:latest"), 0644)
	require.NoError(t, err)
	err = os.WriteFile("Dockerfile.prod.dockerignore", []byte("*.tmp"), 0644)
	require.NoError(t, err)

	command := []string{"docker", "build", "-f", "Dockerfile.prod", "-t", "myapp:latest", "."}

	result, err := ParseBuildCommand(command)
	require.NoError(t, err)

	assert.Equal(t, command, result.Command)
	assert.NotEmpty(t, result.Hash)
}

func TestNormalizeCommandForHashing_LabelBehavior(t *testing.T) {
	testCases := []struct {
		name     string
		input1   []string
		input2   []string
		expected bool // true if they should normalize to the same result
	}{
		{
			name:     "Same label key, different values should produce same normalized output",
			input1:   []string{"docker", "build", "--label", "version=1.2.3", "-t", "myapp:latest", "."},
			input2:   []string{"docker", "build", "--label", "version=2.0.0", "-t", "myapp:latest", "."},
			expected: true,
		},
		{
			name:     "Different label keys should produce different normalized output",
			input1:   []string{"docker", "build", "--label", "version=1.2.3", "-t", "myapp:latest", "."},
			input2:   []string{"docker", "build", "--label", "build=123", "-t", "myapp:latest", "."},
			expected: false,
		},
		{
			name:     "Same label key in equals format, different values should produce same normalized output",
			input1:   []string{"docker", "build", "--label=version=1.2.3", "-t", "myapp:latest", "."},
			input2:   []string{"docker", "build", "--label=version=2.0.0", "-t", "myapp:latest", "."},
			expected: true,
		},
		{
			name:     "Multiple labels with same keys, different values should produce same normalized output",
			input1:   []string{"docker", "build", "--label", "version=1.2.3", "--label", "build=123", "-t", "myapp:latest", "."},
			input2:   []string{"docker", "build", "--label", "version=2.0.0", "--label", "build=456", "-t", "myapp:latest", "."},
			expected: true,
		},
		{
			name:     "Label without equals sign should be preserved as-is",
			input1:   []string{"docker", "build", "--label", "novalue", "-t", "myapp:latest", "."},
			input2:   []string{"docker", "build", "--label", "novalue", "-t", "myapp:latest", "."},
			expected: true,
		},
		{
			name:     "Label with empty value",
			input1:   []string{"docker", "build", "--label", "key=", "-t", "myapp:latest", "."},
			input2:   []string{"docker", "build", "--label", "key=value", "-t", "myapp:latest", "."},
			expected: true, // key is the same, value doesn't matter
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result1 := normalizeCommandForHashing(tc.input1)
			result2 := normalizeCommandForHashing(tc.input2)
			if tc.expected {
				assert.Equal(t, result1, result2, "Commands should normalize to the same result")
			} else {
				assert.NotEqual(t, result1, result2, "Commands should normalize to different results")
			}
		})
	}
}

func TestNormalizeCommandForHashing_SecretBehavior(t *testing.T) {
	testCases := []struct {
		name     string
		input1   []string
		input2   []string
		expected bool // true if they should normalize to the same result
	}{
		{
			name:     "Same secret ID, different src paths should produce same normalized output",
			input1:   []string{"docker", "build", "--secret", "id=mysecret,src=/path/to/file1", "-t", "myapp:latest", "."},
			input2:   []string{"docker", "build", "--secret", "id=mysecret,src=/path/to/file2", "-t", "myapp:latest", "."},
			expected: true,
		},
		{
			name:     "Same secret ID, different env vars should produce same normalized output",
			input1:   []string{"docker", "build", "--secret", "id=mysecret,env=VAR1", "-t", "myapp:latest", "."},
			input2:   []string{"docker", "build", "--secret", "id=mysecret,env=VAR2", "-t", "myapp:latest", "."},
			expected: true,
		},
		{
			name:     "Different secret IDs should produce different normalized output",
			input1:   []string{"docker", "build", "--secret", "id=secret1,src=/path", "-t", "myapp:latest", "."},
			input2:   []string{"docker", "build", "--secret", "id=secret2,src=/path", "-t", "myapp:latest", "."},
			expected: false,
		},
		{
			name:     "Same secret ID in equals format, different src should produce same normalized output",
			input1:   []string{"docker", "build", "--secret=id=mysecret,src=/path1", "-t", "myapp:latest", "."},
			input2:   []string{"docker", "build", "--secret=id=mysecret,src=/path2", "-t", "myapp:latest", "."},
			expected: true,
		},
		{
			name:     "Multiple secrets with same IDs, different src/env should produce same normalized output",
			input1:   []string{"docker", "build", "--secret", "id=secret1,src=/path1", "--secret", "id=secret2,env=VAR1", "-t", "myapp:latest", "."},
			input2:   []string{"docker", "build", "--secret", "id=secret1,src=/path2", "--secret", "id=secret2,env=VAR2", "-t", "myapp:latest", "."},
			expected: true,
		},
		{
			name:     "Secret with both src and env, same ID different sources should produce same normalized output",
			input1:   []string{"docker", "build", "--secret", "id=mysecret,src=/path1,env=VAR1", "-t", "myapp:latest", "."},
			input2:   []string{"docker", "build", "--secret", "id=mysecret,src=/path2,env=VAR2", "-t", "myapp:latest", "."},
			expected: true,
		},
		{
			name:     "Secret with only id (no src or env) should be preserved",
			input1:   []string{"docker", "build", "--secret", "id=mysecret", "-t", "myapp:latest", "."},
			input2:   []string{"docker", "build", "--secret", "id=mysecret", "-t", "myapp:latest", "."},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result1 := normalizeCommandForHashing(tc.input1)
			result2 := normalizeCommandForHashing(tc.input2)
			if tc.expected {
				assert.Equal(t, result1, result2, "Commands should normalize to the same result")
			} else {
				assert.NotEqual(t, result1, result2, "Commands should normalize to different results")
			}
		})
	}
}

func TestTemplateLabelValue(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Simple key=value",
			input:    "version=1.2.3",
			expected: "version=<VALUE>",
		},
		{
			name:     "Key with empty value",
			input:    "key=",
			expected: "key=<VALUE>",
		},
		{
			name:     "Key with value containing equals",
			input:    "key=value=with=equals",
			expected: "key=<VALUE>",
		},
		{
			name:     "No equals sign",
			input:    "novalue",
			expected: "novalue",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := templateLabelValue(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}
