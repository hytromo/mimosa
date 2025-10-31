package docker

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"log/slog"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/hytromo/mimosa/internal/utils/dockerutil"
)

func RetagSingle(fromTag string, toTag string, dryRun bool) error {
	fromRef, err := dockerutil.ParseTag(fromTag)
	if err != nil {
		return err
	}
	toRef, err := dockerutil.ParseTag(toTag)
	if err != nil {
		return err
	}

	// Fetch the descriptor from the remote registry
	fromDesc, err := Get(fromRef.Ref)
	if err != nil {
		slog.Debug("Failed to get descriptor", "fromTag", fromTag, "error", err)
		return fmt.Errorf("failed to get descriptor: %w", err)
	}

	// If dry run, just return success without doing anything
	if dryRun {
		slog.Debug("DRY RUN: Would retag", "fromTag", fromTag, "toTag", toTag)
		return nil
	}

	// Check if it's an index (manifest list)
	if fromDesc.MediaType == types.OCIImageIndex || fromDesc.MediaType == types.DockerManifestList {
		index, err := fromDesc.ImageIndex()
		if err != nil {
			slog.Debug("Failed to get image index", "fromTag", fromTag, "error", err)
			return err
		}

		// Get the manifest descriptors for each platform
		manifestList, err := index.IndexManifest()
		if err != nil {
			slog.Debug("Failed to get manifest list", "fromTag", fromTag, "error", err)
			return err
		}
		var manifestsToRepush []string
		for _, manifest := range manifestList.Manifests {
			manifestsToRepush = append(manifestsToRepush, manifest.Digest.String())
		}
		if len(manifestsToRepush) == 0 {
			return fmt.Errorf("no manifests to repush from %v", fromTag)
		}

		imageNameWithoutTag := fmt.Sprintf("%s/%s", toRef.Registry, toRef.ImageName)
		bareNewTagName := toRef.Tag

		slog.Debug("image will be created", "name", imageNameWithoutTag, "tag", bareNewTagName, "manifests", manifestsToRepush)

		err = PublishManifestsUnderTag(imageNameWithoutTag, bareNewTagName, manifestsToRepush)

		if err != nil {
			slog.Debug("Failed to repush manifests", "fromTag", fromTag, "error", err)
			return err
		}
	} else {
		// this means that the tag does not point to an image index, so a simple retagging is enough
		err = SimpleRetag(fromTag, toTag)
		if err != nil {
			slog.Debug("Failed to retag", "fromTag", fromTag, "toTag", toTag, "error", err)
			return err
		}
	}

	return nil
}

func getTargetsCommaSeparated[V any](m map[string]V) string {
	var targets []string
	for k := range m {
		targets = append(targets, k)
	}
	return strings.Join(targets, ",")
}

// Retag an image by fetching its descriptor and pushing it under a new tag.
// If the image is a manifest list, it will repush all manifests under the new tag
// latestTagByTarget is the map of target->latest cached tag
// newTagsByTarget is the map of target->new tags to push based on the cached entries
func Retag(cachedLatestTagByTarget map[string]string, newTagsByTarget map[string][]string, dryRun bool) error {
	if len(cachedLatestTagByTarget) != len(newTagsByTarget) {
		return fmt.Errorf("different amount of targets between cache and new tags (cache=%s - new=%s)", getTargetsCommaSeparated(cachedLatestTagByTarget), getTargetsCommaSeparated(newTagsByTarget))
	}

	for target := range cachedLatestTagByTarget {
		if _, ok := newTagsByTarget[target]; !ok {
			return fmt.Errorf("different targets between cache and new tags (cache=%s - new=%s)", getTargetsCommaSeparated(cachedLatestTagByTarget), getTargetsCommaSeparated(newTagsByTarget))
		}
	}

	if dryRun {
		slog.Info("> DRY RUN: would be retagged", "from", cachedLatestTagByTarget, "to", newTagsByTarget)
		return nil
	}

	slog.Info("Retagging", "from", cachedLatestTagByTarget, "to", newTagsByTarget)

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
		if fromTag == toTag {
			slog.Info("Skipping retagging to itself", "tag", fromTag)
			return
		}
		if err := RetagSingle(fromTag, toTag, dryRun); err != nil {
			errChan <- err
		}
	}

	// Launch workers
	for target, latestTag := range cachedLatestTagByTarget {
		for _, newTag := range newTagsByTarget[target] {
			slog.Debug("Starting retag worker", "from", latestTag, "to", newTag)
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
		slog.Debug("Failed to parse source reference", "error", err)
		return fmt.Errorf("failed to parse source reference: %w", err)
	}

	dstRef, err := name.ParseReference(target)
	if err != nil {
		slog.Debug("Failed to parse destination reference", "error", err)
		return fmt.Errorf("failed to parse destination reference: %w", err)
	}

	// Get the image from the source tag
	img, err := remote.Image(srcRef, remote.WithAuthFromKeychain(Keychain))
	if err != nil {
		slog.Debug("Failed to get image from source reference", "error", err)
		return fmt.Errorf("failed to get image from source reference: %w", err)
	}

	// Write the same image to the new tag
	if err := remote.Write(dstRef, img, remote.WithAuthFromKeychain(Keychain)); err != nil {
		slog.Debug("Failed to write image to new tag", "error", err)
		return err
	}

	return nil
}
