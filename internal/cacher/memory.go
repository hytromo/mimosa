package cacher

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/elliotchance/orderedmap/v3"
	"github.com/hytromo/mimosa/internal/hasher"
	log "github.com/sirupsen/logrus"
)

type CacheFileWithHash struct {
	Hash string `json:"hash"`
	CacheFile
}

// GetInMemoryEntries retrieves all in-memory cache entries from the MIMOSA_CACHE environment variable.
func GetAllInMemoryEntries() *orderedmap.OrderedMap[string, string] {
	inMemoryEntries := orderedmap.NewOrderedMap[string, string]()

	if mimosaEnvCache := os.Getenv("MIMOSA_CACHE"); mimosaEnvCache != "" {
		// MIMOSA_CACHE is of the form "key1 value1\nkey2 value2\n...", where keys are z85 encoded and each value is a tag.
		for _, line := range strings.Split(mimosaEnvCache, "\n") {
			if line == "" {
				continue
			}
			parts := strings.Split(line, " ")
			if len(parts) != 2 {
				log.Warnln("Invalid MIMOSA_CACHE entry:", line)
				continue
			}
			key := parts[0]
			value := parts[1]
			inMemoryEntries.Set(key, value)
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

		log.Debugf("Loaded cache file %s with tags: %v", path, cacheFile.Tags)

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

	inMemoryEntries := orderedmap.NewOrderedMap[string, string]()

	for _, entry := range diskEntries {
		latestTag := entry.Tags[len(entry.Tags)-1]
		z85Hash, err := hasher.HexToZ85(entry.Hash)
		if err != nil {
			log.Debugf("Failed to convert hash to z85: %v", err)
			continue
		}

		inMemoryEntries.Set(z85Hash, latestTag)
	}
	log.Debugf("Loaded %d cache entries from disk and were decoded to %d in-memory entries", len(diskEntries), inMemoryEntries.Len())

	return inMemoryEntries
}
