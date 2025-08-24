package actions

import (
	"github.com/hytromo/mimosa/internal/cacher"
	"github.com/hytromo/mimosa/internal/docker"
)

// Retag reads the latest available tags in the cache entry and uses them to push the new tags in the command
func (a *Actioner) Retag(cacheEntry cacher.Cache, parsedCommand ParsedCommand, dryRun bool) error {
	return docker.Retag(cacheEntry.GetLatestTagByTarget(), parsedCommand.TagsByTarget, dryRun)
}
