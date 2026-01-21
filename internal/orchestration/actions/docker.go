package actions

import (
	"github.com/hytromo/mimosa/internal/cacher"
	"github.com/hytromo/mimosa/internal/configuration"
	"github.com/hytromo/mimosa/internal/docker"
)

// Retag reads the latest available tags in the cache entry and uses them to push the new tags in the command
func (a *Actioner) Retag(cacheEntry cacher.Cache, parsedCommand configuration.ParsedCommand, dryRun bool) error {
	latestTagByTargetCached, err := cacheEntry.GetLatestTagByTarget()

	if err != nil {
		return err
	}

	return docker.Retag(latestTagByTargetCached, parsedCommand.TagsByTarget, dryRun)
}

// RetagFromCacheTags retags from cache tags (registry-based) to the requested tags
func (a *Actioner) RetagFromCacheTags(cacheTagsByTarget map[string]string, newTagsByTarget map[string][]string, dryRun bool) error {
	return docker.Retag(cacheTagsByTarget, newTagsByTarget, dryRun)
}
