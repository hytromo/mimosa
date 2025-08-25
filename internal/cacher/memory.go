package cacher

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/elliotchance/orderedmap/v3"
	"github.com/hytromo/mimosa/internal/hasher"
	log "github.com/sirupsen/logrus"
)

const (
	// variable is of the form key->cache with optional target name if only one (default)
	// the cache key is z85 encoded
	// key1 (target1.1:)value1.1,(target1.2:)value1.2,...
	// key2 (target2.1:)value2.1,...
	// ... etc
	targetAndTagSeparator     = ":"
	targetsSeparator          = ","
	cachesSeparator           = "\n"
	cacheKeyAndValueSeparator = " "

	envVarName = "MIMOSA_CACHE"
)

type CacheFileWithHash struct {
	Hash string `json:"hash"`
	CacheFile
}

// the cache loaded in memory as key->cache
type InMemoryCache = orderedmap.OrderedMap[string, CacheFile]

// GetInMemoryEntries retrieves all in-memory cache entries from the environment variable.
func GetAllInMemoryEntries() *InMemoryCache {
	inMemoryEntries := orderedmap.NewOrderedMap[string, CacheFile]()

	if mimosaEnvCache := os.Getenv(envVarName); mimosaEnvCache != "" {
		for _, line := range strings.Split(mimosaEnvCache, cachesSeparator) {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			parts := strings.Split(line, cacheKeyAndValueSeparator)
			if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
				log.Warnln("Invalid", envVarName, "entry:", line)
				continue
			}
			cacheKey, err := hasher.Z85ToHex(parts[0])
			if err != nil {
				log.Debugf("Failed to convert Z85 cache key (%v) to string: %v", parts[0], err)
				continue
			}
			allTargetsWithTags := strings.Split(parts[1], targetsSeparator)
			cacheFile := CacheFile{
				TagsByTarget:  make(map[string][]string),
				LastUpdatedAt: time.Now(),
			}
			for _, targetWithTag := range allTargetsWithTags {
				// target name is optional, if there is no : then the target name is "default"
				targetName := "default"
				tag := targetWithTag
				if strings.Contains(targetWithTag, targetAndTagSeparator) {
					targetName = strings.Split(targetWithTag, targetAndTagSeparator)[0]
					tag = strings.Split(targetWithTag, targetAndTagSeparator)[1]
				}
				trimmedTag := strings.TrimSpace(tag)

				cacheFile.TagsByTarget[targetName] = []string{trimmedTag}
			}

			inMemoryEntries.Set(cacheKey, cacheFile)
		}
		// print json representation of the in-memory entries:
		if log.IsLevelEnabled(log.DebugLevel) {
			for key, value := range inMemoryEntries.AllFromFront() {
				hexKey, err := hasher.Z85ToHex(key)
				if err != nil {
					log.Warnf("Failed to convert key to hex: %v", err)
					continue
				}
				log.Debugf("In-memory cache entry: %s (%s) -> %s", key, hexKey, value)
			}
		}
	}

	return inMemoryEntries
}

// GetDiskCacheToMemoryEntries retrieves all disk cache entries and returns them in in-memory representation.
// The entries are ordered from the newest to the oldest and only the latest tag for each hash is kept.
func GetDiskCacheToMemoryEntries() *orderedmap.OrderedMap[string, string] {
	diskEntries := make([]*CacheFileWithHash, 0)
	err := filepath.Walk(CacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Debugf("Failed to walk cache directory: %v", err)
			return nil
		}
		if info.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil // skip directories and non-json files
		}

		data, err := os.ReadFile(path)
		if err != nil {
			log.Debugf("Failed to read cache file %s: %v", path, err)
			return nil
		}

		var cacheFile CacheFile
		err = json.Unmarshal(data, &cacheFile)
		if err != nil {
			log.Debugf("Failed to unmarshal cache file %s: %v", path, err)
			return nil
		}

		log.Debugf("Cache file: %+v", cacheFile)

		// the cache hash is the filename without the extension
		hash := strings.TrimSuffix(filepath.Base(path), ".json")
		diskEntries = append(diskEntries, &CacheFileWithHash{
			Hash:      hash,
			CacheFile: cacheFile,
		})

		return nil
	})

	if (err) != nil {
		log.Debugf("Failed to walk cache directory: %v", err)
	}

	// Sort by LastUpdatedAt (newest first)
	sort.Slice(diskEntries, func(i, j int) bool {
		return diskEntries[i].LastUpdatedAt.After(diskEntries[j].LastUpdatedAt)
	})

	// convert the disk cache to in-memory cache
	// key -> cache (both z85 encoded)
	z85InMemoryEntries := orderedmap.NewOrderedMap[string, string]()

	for _, entry := range diskEntries {
		z85Hash, err := hasher.HexToZ85(entry.Hash)
		if err != nil {
			log.Debugf("Failed to convert hash to z85: %v", err)
			continue
		}

		// Get the latest tag from the first target (assuming single target for now)
		if _, exists := entry.TagsByTarget["default"]; exists && len(entry.TagsByTarget) == 1 {
			// only the default target is present, simplest form: z85Hash -> latestTag
			latestTag := entry.TagsByTarget["default"][len(entry.TagsByTarget["default"])-1]

			z85InMemoryEntries.Set(z85Hash, latestTag)
		} else {
			// multiple targets are present, more complex form:
			// z85Hash -> target1:tag1,target2:tag2,...
			accumulatingValues := []string{}
			for targetName, tags := range entry.TagsByTarget {
				accumulatingValues = append(accumulatingValues, fmt.Sprintf("%s%s%s", targetName, targetAndTagSeparator, tags[len(tags)-1]))
			}
			z85InMemoryEntries.Set(z85Hash, strings.Join(accumulatingValues, targetsSeparator))
		}

	}
	log.Debugf("Loaded %d cache entries from disk and were decoded to %d in-memory entries", len(diskEntries), z85InMemoryEntries.Len())

	return z85InMemoryEntries
}
