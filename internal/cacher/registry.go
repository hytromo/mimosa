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

// Exists checks if cache tags exist in all registries for all tags in TagsByTarget
// Returns: (exists bool, cacheTagsByTarget map[string]string)
// cacheTagsByTarget maps target name -> cache tag to use for that target
// For each target, we check if at least one of its tags has a corresponding cache tag
func (rc *RegistryCache) Exists() (bool, map[string]string, error) {
	if len(rc.TagsByTarget) == 0 {
		return false, nil, fmt.Errorf("no tags to check")
	}

	cacheTagsByTarget := make(map[string]string)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var checkErrors []error
	allExist := true

	// For each target, check if at least one of its tags has a cache tag
	for target, tags := range rc.TagsByTarget {
		if len(tags) == 0 {
			allExist = false
			continue
		}

		// Check all tags for this target - we need at least one cache tag to exist
		targetHasCache := false
		var targetCacheTag string

		for _, tag := range tags {
			cacheTag, err := rc.GetCacheTagForRegistry(tag)
			if err != nil {
				slog.Debug("Failed to construct cache tag", "tag", tag, "error", err)
				continue
			}

			wg.Add(1)
			go func(checkTag string) {
				defer wg.Done()
				exists, err := docker.TagExists(checkTag)
				if err != nil {
					mu.Lock()
					checkErrors = append(checkErrors, fmt.Errorf("failed to check cache tag %s: %w", checkTag, err))
					mu.Unlock()
					return
				}

				mu.Lock()
				if exists && !targetHasCache {
					targetHasCache = true
					targetCacheTag = checkTag
				}
				mu.Unlock()
			}(cacheTag)
		}

		wg.Wait()

		if !targetHasCache {
			allExist = false
			slog.Debug("Cache tag not found for target", "target", target)
		} else {
			cacheTagsByTarget[target] = targetCacheTag
		}
	}

	if len(checkErrors) > 0 {
		// Return first error if any occurred
		return false, nil, checkErrors[0]
	}

	if !allExist {
		return false, nil, nil
	}

	return true, cacheTagsByTarget, nil
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
				// Use SimpleRetag to create the cache tag from the source tag
				err := docker.SimpleRetag(sourceTag, destTag)
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
