package docker

import (
	"fmt"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

func PublishManifestsUnderTag(imageName string, tag string, manifests []string) error {
	// imageName is the repository (e.g., "registry.example.com/myapp")
	// tag is the new tag to push to (e.g. "v1.0.0")
	// manifests are digests that exist in the same repository

	if len(manifests) == 0 {
		return fmt.Errorf("no manifests provided")
	}

	targetRef, err := name.NewTag(fmt.Sprintf("%s:%s", imageName, tag))
	if err != nil {
		return fmt.Errorf("creating tag ref: %w", err)
	}

	var indexManifests []mutate.IndexAddendum

	for _, digest := range manifests {
		// Fetch manifests from the same repository
		ref, err := name.NewDigest(fmt.Sprintf("%s@%s", imageName, digest))
		if err != nil {
			return fmt.Errorf("creating digest ref: %w", err)
		}

		desc, err := Get(ref)
		if err != nil {
			return fmt.Errorf("fetching descriptor: %w", err)
		}

		var add mutate.Appendable
		switch desc.MediaType {
		case types.OCIImageIndex, types.DockerManifestList:
			add, err = desc.ImageIndex()
		default:
			add, err = desc.Image()
		}
		if err != nil {
			return fmt.Errorf("getting appendable: %w", err)
		}

		indexManifests = append(indexManifests, mutate.IndexAddendum{
			Add: add,
		})
	}

	// Create a new image index from the given descriptors
	index := mutate.IndexMediaType(empty.Index, types.OCIImageIndex) // Start with an empty OCI index
	index = mutate.AppendManifests(index, indexManifests...)

	// Push the new index under the given tag
	err = WriteIndex(targetRef, index)
	if err != nil {
		return fmt.Errorf("pushing index: %w", err)
	}

	return nil
}
