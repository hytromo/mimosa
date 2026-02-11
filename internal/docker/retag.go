package docker

import (
	"errors"
	"fmt"
	"sync"

	"log/slog"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
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

	// Use descriptor-based tagging: since source and destination are in the same
	// repository, the registry already has all blobs/manifests. We just point
	// the new tag at the existing descriptor (works for both images and indexes).
	dstTag, err := name.NewTag(toTag)
	if err != nil {
		slog.Debug("Failed to parse destination as tag", "toTag", toTag, "error", err)
		return fmt.Errorf("failed to parse destination tag: %w", err)
	}

	if err := remote.Tag(dstTag, fromDesc, remote.WithAuthFromKeychain(Keychain)); err != nil {
		slog.Debug("Failed to tag descriptor", "fromTag", fromTag, "toTag", toTag, "error", err)
		return fmt.Errorf("failed to tag %s -> %s: %w", fromTag, toTag, err)
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
