package cacher

import (
	"fmt"
	"sync"

	"log/slog"

	"github.com/hytromo/mimosa/internal/docker"
	"github.com/hytromo/mimosa/internal/utils/dockerutil"
)

const CacheTagPrefix = "mimosa-content-hash-"

// RegistryCache handles cache operations using Docker registry tags
type RegistryCache struct {
	Hash         string
	TagsByTarget map[string][]string // from parsed command
}

// GetCacheTagForRegistry constructs the cache tag for a given full tag (registry/image:tag)
// Returns: registry/image:mimosa-content-hash-<hash>
func (rc *RegistryCache) GetCacheTagForRegistry(fullTag string) (string, error) {
	parsed, err := dockerutil.ParseTag(fullTag)
	if err != nil {
		return "", fmt.Errorf("failed to parse tag %s: %w", fullTag, err)
	}

	// Construct cache tag: registry/image:mimosa-content-hash-<hash>
	cacheTag := fmt.Sprintf("%s/%s:%s%s", parsed.Registry, parsed.ImageName, CacheTagPrefix, rc.Hash)
	return cacheTag, nil
}

type existsResult struct {
	cacheTag string
	exists   bool
	err      error
}

// Exists checks if cache tags exist for ALL tags in TagsByTarget
// Returns: (exists bool, cacheTagPairs map[string][]CacheTagPair, error)
// cacheTagPairs maps target name -> list of (cacheTag, newTag) pairs
// Each new tag must have a corresponding cache tag in the SAME repository
func (registryCache *RegistryCache) Exists() (bool, map[string][]CacheTagPair, error) {
	if len(registryCache.TagsByTarget) == 0 {
		return false, nil, fmt.Errorf("no tags to check")
	}

	cacheTagPairs := make(map[string][]CacheTagPair)

	// For each target, check ALL tags - each must have its cache tag in the same repo
	for targetName, tagsForTarget := range registryCache.TagsByTarget {
		if len(tagsForTarget) == 0 {
			return false, nil, nil
		}

		// Group original tags by cache tag to avoid duplicate registry checks
		cacheTagToOrigTags := make(map[string][]string)
		for _, originalTagRef := range tagsForTarget {
			computedCacheTag, err := registryCache.GetCacheTagForRegistry(originalTagRef)
			if err != nil {
				slog.Debug("Failed to construct cache tag", "tag", originalTagRef, "error", err)
				return false, nil, nil
			}
			cacheTagToOrigTags[computedCacheTag] = append(cacheTagToOrigTags[computedCacheTag], originalTagRef)
		}

		// Only check unique cache tags (buffered channel ensures goroutines won't block on early return)
		existsResultChan := make(chan existsResult, len(cacheTagToOrigTags))
		for uniqueCacheTag := range cacheTagToOrigTags {
			go func() {
				slog.Debug("Checking existence of", "cacheTag", uniqueCacheTag)
				exists, err := docker.TagExists(uniqueCacheTag)
				existsResultChan <- existsResult{cacheTag: uniqueCacheTag, exists: exists, err: err}
			}()
		}

		targetPairs := make([]CacheTagPair, 0, len(tagsForTarget))
		for range cacheTagToOrigTags {
			checkResult := <-existsResultChan
			if checkResult.err != nil {
				return false, nil, fmt.Errorf("failed to check cache tag %s: %w", checkResult.cacheTag, checkResult.err)
			}
			if !checkResult.exists {
				slog.Debug("Cache tag not found", "cacheTag", checkResult.cacheTag)
				return false, nil, nil
			}
			// Add pairs for all original tags that share this cache tag
			for _, originalTag := range cacheTagToOrigTags[checkResult.cacheTag] {
				targetPairs = append(targetPairs, CacheTagPair{CacheTag: checkResult.cacheTag, NewTag: originalTag})
			}
		}

		cacheTagPairs[targetName] = targetPairs
	}

	return true, cacheTagPairs, nil
}

// CacheTagPair represents a pair of cache tag and new tag (always in the same repository)
type CacheTagPair struct {
	CacheTag string
	NewTag   string
}

// SaveCacheTags creates cache tags for all images in TagsByTarget
// For each tag in TagsByTarget, it creates a corresponding cache tag pointing to the same image
func (rc *RegistryCache) SaveCacheTags(dryRun bool) error {
	if len(rc.TagsByTarget) == 0 {
		return fmt.Errorf("no tags to save")
	}

	if dryRun {
		slog.Info("> DRY RUN: would create cache tags")
		seen := make(map[string]bool)
		for _, tags := range rc.TagsByTarget {
			for _, tag := range tags {
				cacheTag, err := rc.GetCacheTagForRegistry(tag)
				if err != nil {
					slog.Debug("Failed to construct cache tag", "tag", tag, "error", err)
					continue
				}
				if !seen[cacheTag] {
					seen[cacheTag] = true
					slog.Info("> DRY RUN: would tag", "from", tag, "to", cacheTag)
				}
			}
		}
		return nil
	}

	// Collect unique cache tag operations (multiple tags may map to the same cache tag)
	type retagOp struct {
		sourceTag string
		cacheTag  string
		target    string
	}
	seen := make(map[string]bool)
	var ops []retagOp

	for target, tags := range rc.TagsByTarget {
		for _, tag := range tags {
			cacheTag, err := rc.GetCacheTagForRegistry(tag)
			if err != nil {
				slog.Debug("Failed to construct cache tag", "tag", tag, "error", err)
				continue
			}
			// Only add if we haven't seen this cache tag yet
			if !seen[cacheTag] {
				seen[cacheTag] = true
				ops = append(ops, retagOp{sourceTag: tag, cacheTag: cacheTag, target: target})
			}
		}
	}

	var wg sync.WaitGroup
	errChan := make(chan error, len(ops))

	for _, op := range ops {
		wg.Add(1)
		go func(op retagOp) {
			defer wg.Done()
			// Use RetagSingleTag to properly handle manifest lists (multi-platform images)
			err := docker.RetagSingleTag(op.sourceTag, op.cacheTag, false)
			if err != nil {
				errChan <- fmt.Errorf("failed to create cache tag %s from %s: %w", op.cacheTag, op.sourceTag, err)
				return
			}
			slog.Debug("Created cache tag", "from", op.sourceTag, "to", op.cacheTag, "target", op.target)
		}(op)
	}

	wg.Wait()
	close(errChan)

	// Collect errors
	var allErrs []error
	for err := range errChan {
		allErrs = append(allErrs, err)
	}

	if len(allErrs) > 0 {
		return fmt.Errorf("failed to create some cache tags: %v", allErrs)
	}

	return nil
}
