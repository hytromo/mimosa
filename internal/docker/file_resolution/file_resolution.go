package fileresolution

import (
	"os"
	"path/filepath"
)

func ResolveDockerfilePath(cwd, extractFromCommandDokcerfilePath string) string {
	if extractFromCommandDokcerfilePath == "" {
		extractFromCommandDokcerfilePath = "Dockerfile"
	}

	path, err := filepath.Abs(filepath.Join(cwd, extractFromCommandDokcerfilePath))

	if err == nil {
		return path
	}

	return filepath.Join(cwd, "Dockerfile")
}

func FindDockerignorePath(contextPath, dockerfilePath string) string {
	dockerfileDir := filepath.Dir(dockerfilePath)
	dockerfileBase := filepath.Base(dockerfilePath)
	dockerignoreCandidate := filepath.Join(dockerfileDir, dockerfileBase+".dockerignore")
	if fi, err := os.Stat(dockerignoreCandidate); err == nil && !fi.IsDir() {
		if abs, err := filepath.Abs(dockerignoreCandidate); err == nil {
			return abs
		}
	}
	contextDockerignore := filepath.Join(contextPath, ".dockerignore")
	if fi, err := os.Stat(contextDockerignore); err == nil && !fi.IsDir() {
		if abs, err := filepath.Abs(contextDockerignore); err == nil {
			return abs
		}
	}
	return ""
}
