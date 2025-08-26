package argparse

import (
	"net"
	"strings"
)

func ExtractRegistryDomain(tag string) string {
	// Split image reference into domain and remainder
	// Format: [domain/][user/]repo[:tag|@digest]
	slashParts := strings.SplitN(tag, "/", 2)

	if len(slashParts) == 1 {
		// No domain, so it's Docker Hub
		return "docker.io"
	}

	first := slashParts[0]

	// Check if the first segment looks like a domain or IP
	if strings.Contains(first, ".") || strings.Contains(first, ":") || net.ParseIP(first) != nil {
		return first
	}

	// Otherwise, it's a Docker Hub namespace like "library/ubuntu"
	return "docker.io"
}
