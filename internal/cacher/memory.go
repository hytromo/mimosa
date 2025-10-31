package cacher

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"log/slog"

	"github.com/elliotchance/orderedmap/v3"
	"github.com/hytromo/mimosa/internal/hasher"
	"github.com/hytromo/mimosa/internal/logger"
)

const (
	// variable is of the form key->cache with optional target name if only one (default)
	// the cache key is z85 encoded
	// key1 (target1.1=)value1.1,(target1.2=)value1.2,...
	// key2 (target2.1=)value2.1,...
	// ... etc
	targetAndTagSeparator     = "="
	targetsSeparator          = ","
	cachesSeparator           = "\n"
	cacheKeyAndValueSeparator = " "

	InjectCacheEnvVarName = "MIMOSA_CACHE"
)

type CacheFileWithHash struct {
	HexHash string `json:"hash"`
	CacheFile
}

// the cache loaded in memory as z85 key->cache
type InMemoryCache = orderedmap.OrderedMap[string, CacheFile]

// GetSeparatedInMemoryEntries extracts the "z85Key -> full cache value" cache entries from the environment variable "InjectCacheEnvVarName"
func GetSeparatedInMemoryEntries() map[string]string {
	allEntries := make(map[string]string)

	if mimosaEnvCache := os.Getenv(InjectCacheEnvVarName); mimosaEnvCache != "" {
		for _, line := range strings.Split(mimosaEnvCache, cachesSeparator) {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			parts := strings.Split(line, cacheKeyAndValueSeparator)
			if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
				continue
			}
			z85CacheKey := parts[0]
			cacheValue := parts[1]

			allEntries[z85CacheKey] = cacheValue
		}
	}

	return allEntries
}

// GetInMemoryEntries retrieves all in-memory cache entries from the environment variable in a structured ordered map
func GetAllInMemoryEntries() *InMemoryCache {
	inMemoryEntries := orderedmap.NewOrderedMap[string, CacheFile]()

	if mimosaEnvCache := os.Getenv(InjectCacheEnvVarName); mimosaEnvCache != "" {
		for _, line := range strings.Split(mimosaEnvCache, cachesSeparator) {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			parts := strings.Split(line, cacheKeyAndValueSeparator)
			if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
				continue
			}
			z85CacheKey := parts[0]
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

			inMemoryEntries.Set(z85CacheKey, cacheFile)
		}

		if logger.IsDebugEnabled() {
			for z85CacheKey, value := range inMemoryEntries.AllFromFront() {
				hexKey, err := hasher.Z85ToHex(z85CacheKey)
				if err != nil {
					continue
				}
				slog.Debug("In-memory cache entry", "z85Key", z85CacheKey, "hexKey", hexKey, "value", value)
			}
		}
	}

	return inMemoryEntries
}

// GetDiskCacheToMemoryEntries retrieves all disk cache entries and returns them in in-memory representation.
// The entries are ordered from the newest to the oldest and only the latest tag for each hash is kept.
func GetDiskCacheToMemoryEntries(cacheDir string) *orderedmap.OrderedMap[string, string] {
	diskEntries := make([]*CacheFileWithHash, 0)
	err := filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			slog.Debug("Failed to walk cache directory", "error", err)
			return nil
		}
		if info.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil // skip directories and non-json files
		}

		data, err := os.ReadFile(path)
		if err != nil {
			slog.Debug("Failed to read cache file", "path", path, "error", err)
			return nil
		}

		var cacheFile CacheFile
		err = json.Unmarshal(data, &cacheFile)
		if err != nil {
			slog.Debug("Failed to unmarshal cache file", "path", path, "error", err)
			return nil
		}

		slog.Debug("Cache file", "file", cacheFile)

		// the cache hexHash is the filename without the extension
		hexHash := strings.TrimSuffix(filepath.Base(path), ".json")
		diskEntries = append(diskEntries, &CacheFileWithHash{
			HexHash:   hexHash,
			CacheFile: cacheFile,
		})

		return nil
	})

	if (err) != nil {
		slog.Debug("Failed to walk cache directory", "error", err)
	}

	// Sort by LastUpdatedAt (newest first)
	sort.Slice(diskEntries, func(i, j int) bool {
		return diskEntries[i].LastUpdatedAt.After(diskEntries[j].LastUpdatedAt)
	})

	// convert the disk cache to in-memory cache
	// z85Key -> cache
	z85InMemoryEntries := orderedmap.NewOrderedMap[string, string]()

	for _, entry := range diskEntries {
		z85Hash, err := hasher.HexToZ85(entry.HexHash)
		if err != nil {
			slog.Debug("Failed to convert hash to z85", "error", err)
			continue
		}

		// Get the latest tag from the first target
		if _, exists := entry.TagsByTarget["default"]; exists && len(entry.TagsByTarget) == 1 {
			// only the default target is present, simplest form: z85Hash -> latestTag
			if len(entry.TagsByTarget["default"]) > 0 {
				latestTag := entry.TagsByTarget["default"][len(entry.TagsByTarget["default"])-1]
				z85InMemoryEntries.Set(z85Hash, latestTag)
			}
		} else {
			// multiple targets are present, more complex form:
			// z85Hash -> target1:tag1,target2:tag2,...
			accumulatingValues := []string{}
			for targetName, tags := range entry.TagsByTarget {
				if len(tags) > 0 {
					accumulatingValues = append(accumulatingValues, fmt.Sprintf("%s%s%s", targetName, targetAndTagSeparator, tags[len(tags)-1]))
				}
			}
			if len(accumulatingValues) > 0 {
				z85InMemoryEntries.Set(z85Hash, strings.Join(accumulatingValues, targetsSeparator))
			}
		}

	}
	slog.Debug("Loaded cache entries from disk and decoded to in-memory entries", "diskCount", len(diskEntries), "memoryCount", z85InMemoryEntries.Len())

	return z85InMemoryEntries
}
