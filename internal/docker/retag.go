package docker

import (
	"errors"
	"fmt"
	"sync"

	"log/slog"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/hytromo/mimosa/internal/utils/dockerutil"
)

func RetagSingleTag(fromTag string, toTag string, dryRun bool) error {
	fromRef, err := dockerutil.ParseTag(fromTag)
	if err != nil {
		return err
	}
	toRef, err := dockerutil.ParseTag(toTag)
	if err != nil {
		return err
	}

	// Retagging MUST be within the same repository
	if fromRef.Registry != toRef.Registry || fromRef.ImageName != toRef.ImageName {
		return fmt.Errorf("retagging across repositories is not supported: %s -> %s", fromTag, toTag)
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

		// Same repository for source and target (enforced above)
		imageName := fmt.Sprintf("%s/%s", fromRef.Registry, fromRef.ImageName)
		bareNewTagName := toRef.Tag

		slog.Debug("image will be created", "imageName", imageName, "tag", bareNewTagName, "manifests", manifestsToRepush)

		err = PublishManifestsUnderTag(imageName, bareNewTagName, manifestsToRepush)

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

// CacheTagPair represents a pair of cache tag and new tag (always in the same repository)
type CacheTagPair struct {
	CacheTag string
	NewTag   string
}

// Retag creates new tags from cache tags.
// Each CacheTagPair contains a cache tag and its corresponding new tag - both MUST be in the same repository.
// cacheTagPairsByTarget maps target name -> list of (cacheTag, newTag) pairs
func Retag(cacheTagPairsByTarget map[string][]CacheTagPair, dryRun bool) error {
	if len(cacheTagPairsByTarget) == 0 {
		return fmt.Errorf("no cache tag pairs provided")
	}

	// Count total retag operations
	nWorkers := 0
	for _, pairs := range cacheTagPairsByTarget {
		nWorkers += len(pairs)
	}

	if dryRun {
		slog.Info("> DRY RUN: would retag", "pairs", cacheTagPairsByTarget)
		return nil
	}

	slog.Info("Retagging from cache", "targets", len(cacheTagPairsByTarget), "totalOperations", nWorkers)

	// let's print explicitly the FROM->TO tags
	for target, pairs := range cacheTagPairsByTarget {
		for _, pair := range pairs {
			slog.Info("Retagging", "target", target, "from", pair.CacheTag, "to", pair.NewTag)
		}
	}

	var wg sync.WaitGroup
	wg.Add(nWorkers)

	// Create error channel to collect errors from workers
	errChan := make(chan error, nWorkers)

	// Worker function - retag within the same repository
	worker := func(fromTag string, toTag string) {
		defer wg.Done()
		if fromTag == toTag {
			slog.Info("Skipping retagging to itself", "tag", fromTag)
			return
		}
		slog.Debug("Retagging", "from", fromTag, "to", toTag)
		if err := RetagSingleTag(fromTag, toTag, dryRun); err != nil {
			errChan <- fmt.Errorf("failed to retag %s -> %s: %w", fromTag, toTag, err)
		}
	}

	// Launch workers - each pair is cache tag -> new tag in the SAME repository
	for target, pairs := range cacheTagPairsByTarget {
		for _, pair := range pairs {
			slog.Debug("Starting retag worker", "target", target, "from", pair.CacheTag, "to", pair.NewTag)
			go worker(pair.CacheTag, pair.NewTag)
		}
	}

	// Wait for all workers to complete
	wg.Wait()
	close(errChan)

	// Check for any errors
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
