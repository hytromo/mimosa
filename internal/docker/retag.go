package docker

import (
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/types"
	log "github.com/sirupsen/logrus"
)

func Retag(previousTag string, newTag string) error {
	ref, err := name.ParseReference(previousTag)
	if err != nil {
		return err
	}

	// Fetch the descriptor from the remote registry
	desc, err := Get(ref)
	if err != nil {
		return err
	}

	// Check if it's an index (manifest list)
	if desc.Descriptor.MediaType == types.OCIImageIndex || desc.Descriptor.MediaType == types.DockerManifestList {
		index, err := desc.ImageIndex()
		if err != nil {
			return err
		}

		// Get the manifest descriptors for each platform
		manifestList, err := index.IndexManifest()
		if err != nil {
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
			return err
		}
	} else {
		return fmt.Errorf("not an image index")
	}

	return nil
}
