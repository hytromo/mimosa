package fileresolution

import (
	"os"
	"path/filepath"

	"log/slog"
)

func ResolveAbsoluteDockerfilePath(cwd, absOrRelativeDockerfilePath string) string {
	if filepath.IsAbs(absOrRelativeDockerfilePath) {
		return absOrRelativeDockerfilePath
	}

	if absOrRelativeDockerfilePath == "" {
		absOrRelativeDockerfilePath = "Dockerfile"
	}

	path, err := filepath.Abs(filepath.Join(cwd, absOrRelativeDockerfilePath))

	if err == nil {
		return path
	}

	return filepath.Join(cwd, "Dockerfile")
}

func ResolveAbsoluteDockerIgnorePath(contextPathAbs, dockerfilePathAbs string) string {
	dockerfileDir := filepath.Dir(dockerfilePathAbs)
	dockerfileBase := filepath.Base(dockerfilePathAbs)
	dockerignoreCandidate := filepath.Join(dockerfileDir, dockerfileBase+".dockerignore")
	if fi, err := os.Stat(dockerignoreCandidate); err == nil && !fi.IsDir() {
		if abs, err := filepath.Abs(dockerignoreCandidate); err == nil {
			return abs
		} else {
			slog.Info("Failed to get absolute path for dockerignore candidate", "error", err)
		}
	}

	contextDockerignore := filepath.Join(contextPathAbs, ".dockerignore")
	if fi, err := os.Stat(contextDockerignore); err == nil && !fi.IsDir() {
		if abs, err := filepath.Abs(contextDockerignore); err == nil {
			return abs
		}
	}
	return ""
}
