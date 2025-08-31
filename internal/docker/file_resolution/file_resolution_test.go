package fileresolution

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveDockerfilePath(t *testing.T) {
	testCases := []struct {
		name                       string
		cliArgPassedDockerFilePath string
		expectedDockerfilePath     string
	}{
		{
			name:                       "Empty dockerfile path",
			cliArgPassedDockerFilePath: "",
			expectedDockerfilePath:     "Dockerfile",
		},
		{
			name:                       "Just Dockerfile name",
			cliArgPassedDockerFilePath: "Dockerfile",
			expectedDockerfilePath:     "Dockerfile",
		},
		{
			name:                       "Dockerfile with absolute path",
			cliArgPassedDockerFilePath: "/absolute/path/Dockerfile",
			expectedDockerfilePath:     "/absolute/path/Dockerfile",
		},
		{
			name:                       "Dockerfile with multiple dots",
			cliArgPassedDockerFilePath: "Dockerfile.prod.staging",
			expectedDockerfilePath:     "Dockerfile.prod.staging",
		},
		{
			name:                       "Dockerfile in inner directory",
			cliArgPassedDockerFilePath: "inner/dir/Dockerfile_prod",
			expectedDockerfilePath:     "inner/dir/Dockerfile_prod",
		},
		{
			name:                       "Dockerfile in previous directory",
			cliArgPassedDockerFilePath: "../../Dockerfile.prod",
			expectedDockerfilePath:     "../../Dockerfile.prod",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			workDir := t.TempDir()

			foundDockerfilePath := ResolveAbsoluteDockerfilePath(workDir, tc.cliArgPassedDockerFilePath)
			assert.True(t, filepath.IsAbs(foundDockerfilePath))

			expectedDockerfilePathAbs := tc.expectedDockerfilePath
			var err error
			if !filepath.IsAbs(expectedDockerfilePathAbs) {
				expectedDockerfilePathAbs, err = filepath.Abs(filepath.Join(workDir, tc.expectedDockerfilePath))
				require.NoError(t, err)
			}

			assert.Equal(t, expectedDockerfilePathAbs, foundDockerfilePath)
		})
	}
}

func TestResolveDockerignorePath(t *testing.T) {
	testCases := []struct {
		name                                      string
		dockerfilePath                            string
		expectedDockerignorePath                  string
		createDockerignoreInContext               bool
		createDockerignoreInDockerfileDir         bool
		createDockerignoreInDockerfileDirNoPrefix bool
	}{
		{
			name:                              "Dockerignore in context",
			dockerfilePath:                    "Dockerfile",
			expectedDockerignorePath:          ".dockerignore",
			createDockerignoreInContext:       true,
			createDockerignoreInDockerfileDir: false,
		},
		{
			name:                              "Dockerignore in Dockerfile directory",
			dockerfilePath:                    "docker/Dockerfile-random-name",
			expectedDockerignorePath:          "docker/Dockerfile-random-name.dockerignore",
			createDockerignoreInContext:       false,
			createDockerignoreInDockerfileDir: true,
		},
		{
			name:                              "Dockerignore in Dockerfile directory with random name preferred over context .dockerignore",
			dockerfilePath:                    "docker/Dockerfilebobos",
			expectedDockerignorePath:          "docker/Dockerfilebobos.dockerignore",
			createDockerignoreInContext:       true,
			createDockerignoreInDockerfileDir: true,
		},
		{
			name:                              "No dockerignore files",
			dockerfilePath:                    "Dockerfile",
			expectedDockerignorePath:          "",
			createDockerignoreInContext:       false,
			createDockerignoreInDockerfileDir: false,
		},
		{
			name:                                      ".dockerignore in Dockerfile directory ignored",
			dockerfilePath:                            "inner/dir/Dockerfile",
			expectedDockerignorePath:                  "",
			createDockerignoreInContext:               false,
			createDockerignoreInDockerfileDir:         false,
			createDockerignoreInDockerfileDirNoPrefix: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			contextDirAbs := t.TempDir()

			dockerfilePathAbs := filepath.Join(contextDirAbs, testCase.dockerfilePath)
			// Dockerfile directory: {contextDir}/{dir of dockerfilePath}
			dockerfileDirAbs := filepath.Dir(dockerfilePathAbs)
			err := os.MkdirAll(dockerfileDirAbs, 0755)
			require.NoError(t, err)

			if testCase.createDockerignoreInContext {
				dockerignorePathAbs := filepath.Join(contextDirAbs, ".dockerignore")
				err = os.WriteFile(dockerignorePathAbs, []byte("*.log\nnode_modules"), 0644)
				require.NoError(t, err)
			}

			if testCase.createDockerignoreInDockerfileDir {
				dockerFileFilename := filepath.Base(testCase.dockerfilePath)
				dockerignorePathAbs := filepath.Join(dockerfileDirAbs, dockerFileFilename+".dockerignore")
				err = os.WriteFile(dockerignorePathAbs, []byte("*.log\nnode_modules"), 0644)
				require.NoError(t, err)
			}

			if testCase.createDockerignoreInDockerfileDirNoPrefix {
				dockerignorePathAbs := filepath.Join(dockerfileDirAbs, ".dockerignore")
				err = os.WriteFile(dockerignorePathAbs, []byte("*.log\nnode_modules"), 0644)
				require.NoError(t, err)
			}

			// Make dockerfile path relative to context
			foundDockerIgnorePath := ResolveAbsoluteDockerIgnorePath(contextDirAbs, dockerfilePathAbs)

			if testCase.expectedDockerignorePath != "" {
				expectedDockerignorePathAbs := filepath.Join(contextDirAbs, testCase.expectedDockerignorePath)
				assert.True(t, filepath.IsAbs(foundDockerIgnorePath))
				assert.Equal(t, expectedDockerignorePathAbs, foundDockerIgnorePath)
			} else {
				assert.Equal(t, "", foundDockerIgnorePath)
			}
		})
	}
}
