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

// Exists checks if cache tags exist for ALL tags in TagsByTarget
// Returns: (exists bool, cacheTagPairs map[string][]CacheTagPair)
// cacheTagPairs maps target name -> list of (cacheTag, newTag) pairs
// Each new tag must have a corresponding cache tag in the SAME repository
func (rc *RegistryCache) Exists() (bool, map[string][]CacheTagPair, error) {
	if len(rc.TagsByTarget) == 0 {
		return false, nil, fmt.Errorf("no tags to check")
	}

	cacheTagPairs := make(map[string][]CacheTagPair)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var checkErrors []error
	allExist := true

	// For each target, check ALL tags - each must have its cache tag in the same repo
	for target, tags := range rc.TagsByTarget {
		if len(tags) == 0 {
			allExist = false
			continue
		}

		targetPairs := make([]CacheTagPair, 0, len(tags))
		targetAllExist := true

		for _, tag := range tags {
			cacheTag, err := rc.GetCacheTagForRegistry(tag)
			if err != nil {
				slog.Debug("Failed to construct cache tag", "tag", tag, "error", err)
				targetAllExist = false
				continue
			}

			wg.Add(1)
			go func(origTag, cTag string) {
				defer wg.Done()
				exists, err := docker.TagExists(cTag)
				if err != nil {
					mu.Lock()
					checkErrors = append(checkErrors, fmt.Errorf("failed to check cache tag %s: %w", cTag, err))
					mu.Unlock()
					return
				}

				mu.Lock()
				if exists {
					targetPairs = append(targetPairs, CacheTagPair{CacheTag: cTag, NewTag: origTag})
				} else {
					targetAllExist = false
					slog.Debug("Cache tag not found", "cacheTag", cTag, "forNewTag", origTag)
				}
				mu.Unlock()
			}(tag, cacheTag)
		}

		wg.Wait()

		if !targetAllExist || len(targetPairs) != len(tags) {
			allExist = false
			slog.Debug("Not all cache tags found for target", "target", target, "found", len(targetPairs), "expected", len(tags))
		} else {
			cacheTagPairs[target] = targetPairs
		}
	}

	if len(checkErrors) > 0 {
		// Return first error if any occurred
		return false, nil, checkErrors[0]
	}

	if !allExist {
		return false, nil, nil
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
		for _, tags := range rc.TagsByTarget {
			for _, tag := range tags {
				cacheTag, err := rc.GetCacheTagForRegistry(tag)
				if err != nil {
					slog.Debug("Failed to construct cache tag", "tag", tag, "error", err)
					continue
				}
				slog.Info("> DRY RUN: would tag", "from", tag, "to", cacheTag)
			}
		}
		return nil
	}

	var wg sync.WaitGroup
	errChan := make(chan error, 100) // Buffer for errors

	// For each tag, create the corresponding cache tag
	for target, tags := range rc.TagsByTarget {
		for _, tag := range tags {
			cacheTag, err := rc.GetCacheTagForRegistry(tag)
			if err != nil {
				slog.Debug("Failed to construct cache tag", "tag", tag, "error", err)
				continue
			}

			wg.Add(1)
			go func(sourceTag, destTag string) {
				defer wg.Done()
				// Use RetagSingleTag to properly handle manifest lists (multi-platform images)
				err := docker.RetagSingleTag(sourceTag, destTag, false)
				if err != nil {
					errChan <- fmt.Errorf("failed to create cache tag %s from %s: %w", destTag, sourceTag, err)
					return
				}
				slog.Debug("Created cache tag", "from", sourceTag, "to", destTag, "target", target)
			}(tag, cacheTag)
		}
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
