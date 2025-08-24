package docker

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	log "github.com/sirupsen/logrus"
)

func RetagSingle(fromTag string, toTag string, dryRun bool) error {
	ref, err := name.ParseReference(fromTag)
	if err != nil {
		return err
	}

	// Fetch the descriptor from the remote registry
	desc, err := Get(ref)
	if err != nil {
		log.Debugln("Failed to get descriptor for", fromTag, ":", err)
		return err
	}

	// Check if it's an index (manifest list)
	if desc.MediaType == types.OCIImageIndex || desc.MediaType == types.DockerManifestList {
		index, err := desc.ImageIndex()
		if err != nil {
			log.Debugln("Failed to get image index for", fromTag, ":", err)
			return err
		}

		// Get the manifest descriptors for each platform
		manifestList, err := index.IndexManifest()
		if err != nil {
			log.Debugln("Failed to get manifest list for", fromTag, ":", err)
			return err
		}
		var manifestsToRepush []string
		for _, manifest := range manifestList.Manifests {
			manifestsToRepush = append(manifestsToRepush, manifest.Digest.String())
		}
		if len(manifestsToRepush) == 0 {
			return fmt.Errorf("no manifests to repush from %v", fromTag)
		}
		bareImageName := strings.Split(fromTag, ":")[0]
		bareNewTagName := strings.Split(toTag, ":")[1]

		log.Debugln("image with name", bareImageName, "and tag", bareNewTagName, "will be created, using the manifests", manifestsToRepush)
		err = PublishManifestsUnderTag(bareImageName, bareNewTagName, manifestsToRepush)

		if err != nil {
			log.Debugln("Failed to repush manifests for", fromTag, ":", err)
			return err
		}
	} else {
		// this means that the tag does not point to an image index, so a simple retagging is enough
		err = SimpleRetag(fromTag, toTag)
		if err != nil {
			log.Debugln("Failed to retag", fromTag, "to", toTag, ":", err)
			return err
		}
	}

	return nil
}

// Retag an image by fetching its descriptor and pushing it under a new tag.
// If the image is a manifest list, it will repush all manifests under the new tag
// latestTagByTarget is the map of target->latest cached tag
// newTagsByTarget is the map of target->new tags to push based on the cached entries
func Retag(latestTagByTarget map[string]string, newTagsByTarget map[string][]string, dryRun bool) error {
	if len(latestTagByTarget) != len(newTagsByTarget) {
		return fmt.Errorf("different amount of targets between cache and new tags")
	}

	for target := range latestTagByTarget {
		if _, ok := newTagsByTarget[target]; !ok {
			return fmt.Errorf("different targets between cache and new tags")
		}
	}

	if dryRun {
		log.Infoln("> DRY RUN:", fmt.Sprintf("%+v", latestTagByTarget), "would be retagged to", fmt.Sprintf("%+v", newTagsByTarget))
		return nil
	}

	// each worker will do 1 retag operation, so the total workers needs to be len(newTagsByTarget[*])
	nWorkers := 0
	for _, tags := range newTagsByTarget {
		nWorkers += len(tags)
	}

	var wg sync.WaitGroup
	wg.Add(nWorkers)

	// Create error channel to collect errors from workers
	errChan := make(chan error, nWorkers)

	// Worker function
	worker := func(fromTag string, toTag string) {
		defer wg.Done()
		if err := RetagSingle(fromTag, toTag, dryRun); err != nil {
			errChan <- err
		}
	}

	// Launch workers
	for target, latestTag := range latestTagByTarget {
		for _, newTag := range newTagsByTarget[target] {
			go worker(latestTag, newTag)
		}
	}

	// Wait for all workers to complete
	wg.Wait()
	close(errChan)

	// Check for any allErrs
	var allErrs []error
	for err := range errChan {
		if err != nil {
			allErrs = append(allErrs, err)
		}
	}

	if len(allErrs) > 0 {
		return errors.Join(allErrs...)
	}

	return nil
}

func SimpleRetag(source, target string) error {
	srcRef, err := name.ParseReference(source)
	if err != nil {
		log.Debugln("Failed to parse source reference:", err)
		return err
	}

	dstRef, err := name.ParseReference(target)
	if err != nil {
		log.Debugln("Failed to parse destination reference:", err)
		return err
	}

	// Get the image from the source tag
	img, err := remote.Image(srcRef, remote.WithAuthFromKeychain(Keychain))
	if err != nil {
		log.Debugln("Failed to get image from source reference:", err)
		return err
	}

	// Write the same image to the new tag
	if err := remote.Write(dstRef, img, remote.WithAuthFromKeychain(Keychain)); err != nil {
		log.Debugln("Failed to write image to new tag:", err)
		return err
	}

	return nil
}
