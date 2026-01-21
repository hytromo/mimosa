package actions

import (
	"github.com/hytromo/mimosa/internal/cacher"
	"github.com/hytromo/mimosa/internal/docker"
)

// RetagFromCacheTags retags from cache tags (registry-based) to the requested tags
func (a *Actioner) RetagFromCacheTags(cacheTagsByTarget map[string]string, newTagsByTarget map[string][]string, dryRun bool) error {
	return docker.Retag(cacheTagsByTarget, newTagsByTarget, dryRun)
}

func (a *Actioner) CheckRegistryCacheExists(hash string, tagsByTarget map[string][]string) (bool, map[string]string, error) {
	registryCache := &cacher.RegistryCache{
		Hash:         hash,
		TagsByTarget: tagsByTarget,
	}
	return registryCache.Exists()
}

func (a *Actioner) SaveRegistryCacheTags(hash string, tagsByTarget map[string][]string, dryRun bool) error {
	registryCache := &cacher.RegistryCache{
		Hash:         hash,
		TagsByTarget: tagsByTarget,
	}
	return registryCache.SaveCacheTags(dryRun)
}
