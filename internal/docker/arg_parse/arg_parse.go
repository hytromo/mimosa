package argparse

import (
	"github.com/hytromo/mimosa/internal/utils/dockerutil"
)

func ExtractRegistryDomain(tag string) string {
	if parsed, err := dockerutil.ParseTag(tag); err == nil {
		return parsed.Registry
	}

	// If parsing fails, fall back to Docker Hub
	return "index.docker.io"
}
