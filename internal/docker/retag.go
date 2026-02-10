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

		// Get the manifest descriptors for each platform (preserving platform metadata)
		manifestList, err := index.IndexManifest()
		if err != nil {
			slog.Debug("Failed to get manifest list", "fromTag", fromTag, "error", err)
			return err
		}
		if len(manifestList.Manifests) == 0 {
			return fmt.Errorf("no manifests to repush from %v", fromTag)
		}

		// Same repository for source and target (enforced above)
		imageName := fmt.Sprintf("%s/%s", fromRef.Registry, fromRef.ImageName)
		bareNewTagName := toRef.Tag

		slog.Debug("image will be created", "imageName", imageName, "tag", bareNewTagName, "manifests", len(manifestList.Manifests))

		err = PublishManifestsUnderTag(imageName, bareNewTagName, manifestList.Manifests)

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
		slog.Info("Retagging", "from", fromTag, "to", toTag)
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

	// Fetch the descriptor to determine if it's an index or a single image.
	// Modern Docker Desktop may create OCI indexes even for single-platform builds
	// (e.g., with attestation manifests), so we must handle both cases.
	desc, err := Get(srcRef)
	if err != nil {
		slog.Debug("Failed to get descriptor from source reference", "error", err)
		return fmt.Errorf("failed to get descriptor from source reference: %w", err)
	}

	switch desc.MediaType {
	case types.OCIImageIndex, types.DockerManifestList:
		idx, err := desc.ImageIndex()
		if err != nil {
			slog.Debug("Failed to get image index from source reference", "error", err)
			return fmt.Errorf("failed to get image index from source reference: %w", err)
		}
		if err := WriteIndex(dstRef, idx); err != nil {
			slog.Debug("Failed to write index to new tag", "error", err)
			return fmt.Errorf("failed to write index to new tag: %w", err)
		}
	default:
		img, err := desc.Image()
		if err != nil {
			slog.Debug("Failed to get image from source reference", "error", err)
			return fmt.Errorf("failed to get image from source reference: %w", err)
		}
		if err := remote.Write(dstRef, img, remote.WithAuthFromKeychain(Keychain)); err != nil {
			slog.Debug("Failed to write image to new tag", "error", err)
			return fmt.Errorf("failed to write image to new tag: %w", err)
		}
	}

	return nil
}
