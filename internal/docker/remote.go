package docker

import (
	"strings"

	"log/slog"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

func Get(ref name.Reference, options ...remote.Option) (*remote.Descriptor, error) {
	return remote.Get(ref, remote.WithAuthFromKeychain(Keychain))
}

// TagExists checks if a tag exists in the remote registry
func TagExists(fullTag string) (bool, error) {
	ref, err := name.ParseReference(fullTag)
	if err != nil {
		slog.Debug("Failed to parse tag reference", "tag", fullTag, "error", err)
		return false, err
	}

	// Use Head instead of Get for a lighter-weight existence check
	_, err = remote.Head(ref, remote.WithAuthFromKeychain(Keychain))
	if err != nil {
		// Check if it's a "not found" error by examining the error message
		// go-containerregistry doesn't expose a specific ErrNotFound type,
		// but not found errors typically contain "MANIFEST_UNKNOWN" or "not found"
		errStr := err.Error()
		if strings.Contains(errStr, "MANIFEST_UNKNOWN") ||
			strings.Contains(errStr, "not found") ||
			strings.Contains(errStr, "404") ||
			strings.Contains(errStr, "NAME_UNKNOWN") {
			return false, nil
		}
		// Other errors (auth, network, etc.) should be returned
		slog.Debug("Error checking tag existence", "tag", fullTag, "error", err)
		return false, err
	}

	return true, nil
}
