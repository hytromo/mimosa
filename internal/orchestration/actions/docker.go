package actions

import (
	"github.com/hytromo/mimosa/internal/cacher"
	"github.com/hytromo/mimosa/internal/docker"
)

// RetagFromCacheTags retags from cache tags to new tags.
// Each cache tag pair contains a cache tag and its corresponding new tag in the SAME repository.
func (a *Actioner) RetagFromCacheTags(cacheTagPairsByTarget map[string][]cacher.CacheTagPair, dryRun bool) error {
	// Convert cacher.CacheTagPair to docker.CacheTagPair
	dockerPairs := make(map[string][]docker.CacheTagPair)
	for target, pairs := range cacheTagPairsByTarget {
		dockerPairs[target] = make([]docker.CacheTagPair, len(pairs))
		for i, p := range pairs {
			dockerPairs[target][i] = docker.CacheTagPair{CacheTag: p.CacheTag, NewTag: p.NewTag}
		}
	}
	return docker.Retag(dockerPairs, dryRun)
}

func (a *Actioner) CheckRegistryCacheExists(hash string, tagsByTarget map[string][]string) (bool, map[string][]cacher.CacheTagPair, error) {
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
