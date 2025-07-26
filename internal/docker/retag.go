package docker

import (
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/types"
	log "github.com/sirupsen/logrus"
)

// Retag an image by fetching its descriptor and pushing it under a new tag.
// If the image is a manifest list, it will repush all manifests under the new tag
// previousTag is the full tag of the image to retag
// newTag is the full new tag to push
func Retag(previousTag string, newTag string) error {
	ref, err := name.ParseReference(previousTag)
	if err != nil {
		return err
	}

	// Fetch the descriptor from the remote registry
	desc, err := Get(ref)
	if err != nil {
		log.Debugln("Failed to get descriptor for", previousTag, ":", err)
		return err
	}

	// Check if it's an index (manifest list)
	if desc.MediaType == types.OCIImageIndex || desc.MediaType == types.DockerManifestList {
		index, err := desc.ImageIndex()
		if err != nil {
			log.Debugln("Failed to get image index for", previousTag, ":", err)
			return err
		}

		// Get the manifest descriptors for each platform
		manifestList, err := index.IndexManifest()
		if err != nil {
			log.Debugln("Failed to get manifest list for", previousTag, ":", err)
			return err
		}
		var manifestsToRepush []string
		for _, manifest := range manifestList.Manifests {
			manifestsToRepush = append(manifestsToRepush, manifest.Digest.String())
		}
		if len(manifestsToRepush) == 0 {
			return fmt.Errorf("no manifests to repush from %v", previousTag)
		}
		bareImageName := strings.Split(previousTag, ":")[0]
		bareNewTagName := strings.Split(newTag, ":")[1]

		log.Debugln("image with name", bareImageName, "and tag", bareNewTagName, "will be created, using the manifests", manifestsToRepush)
		err = PublishManifestsUnderTag(bareImageName, bareNewTagName, manifestsToRepush)

		if err != nil {
			log.Debugln("Failed to repush manifests for", previousTag, ":", err)
			return err
		}
	} else {
		// this means that the tag does not point to an image index, so a simple retagging is enough
		err = SimpleRetag(previousTag, newTag)
		if err != nil {
			log.Debugln("Failed to retag", previousTag, "to", newTag, ":", err)
			return err
		}
	}

	return nil
}
