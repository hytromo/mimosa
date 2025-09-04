package cacher

import (
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/samber/lo"

	"github.com/apparentlymart/go-userdirs/userdirs"
	"github.com/hytromo/mimosa/internal/hasher"
	"github.com/hytromo/mimosa/internal/utils/fileutil"
)

var CacheDir = userdirs.ForApp("mimosa", "hytromo", "mimosa.hytromo.com").CacheDir

type CacheFile struct {
	TagsByTarget  map[string][]string `json:"tagsByTarget"`
	LastUpdatedAt time.Time           `json:"lastUpdatedAt"`
}

// Cache represents the final hash of the currently running command and files
// and the current available in-memory cache entries
type Cache struct {
	InMemoryEntries *InMemoryCache // populated by the "envVarName" environment variable and taking precedence over the cache directory
	CacheDir        string         // the directory where the cache files are stored - defaults to CacheDir
	Hash            string         // the final hash of the current command and files
}

func (cache *Cache) DataPath() string {
	return filepath.Join(cache.CacheDir, cache.Hash+".json")
}

func (cache *Cache) GetLatestTagByTarget() (map[string]string, error) {
	// read the cache file and for each of the targets get the most recent cached tag:
	data, err := os.ReadFile(cache.DataPath())
	if err != nil {
		return nil, err
	}

	var cacheFile CacheFile
	err = json.Unmarshal(data, &cacheFile)
	if err != nil {
		return nil, err
	}

	latestTagByTarget := make(map[string]string)

	for target, tags := range cacheFile.TagsByTarget {
		if len(tags) > 0 {
			latestTagByTarget[target] = tags[len(tags)-1]
		}
	}

	return latestTagByTarget, nil
}

func (cache *Cache) ExistsInFilesystem() bool {
	if _, err := os.Stat(cache.DataPath()); errors.Is(err, os.ErrNotExist) {
		return false
	}
	return true
}

func (cache *Cache) Remove(dryRun bool) error {
	if dryRun {
		slog.Info("> DRY RUN: cache entry would be removed from", "path", cache.DataPath())
		return nil
	}

	return os.Remove(cache.DataPath())
}

func (cache *Cache) GetInMemoryEntry() (CacheFile, bool) {
	if cache.InMemoryEntries.Len() == 0 {
		return CacheFile{}, false
	}

	z85Hash, err := hasher.HexToZ85(cache.Hash)
	if err != nil {
		slog.Warn("Failed to convert final hash to Z85", "error", err)
		return CacheFile{}, false
	}
	if entry, ok := cache.InMemoryEntries.Get(z85Hash); ok {
		return entry, true
	}

	return CacheFile{}, false
}

func (cache *Cache) Exists() bool {
	if _, ok := cache.GetInMemoryEntry(); ok {
		slog.Debug("Cache hit in memory for hash", "hash", cache.Hash)
		return true
	}

	if _, err := os.Stat(cache.DataPath()); errors.Is(err, os.ErrNotExist) {
		return false
	}

	slog.Debug("Cache hit on disk for hash", "hash", cache.Hash)

	return true
}

func (cache *Cache) Save(tagsByTarget map[string][]string, dryRun bool) error {
	dataFile := cache.DataPath()

	if dryRun {
		slog.Info("> DRY RUN: cache entry would be saved to", "path", dataFile)
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(dataFile), 0755); err != nil {
		return err
	}

	var td CacheFile

	// Read existing tags from the cache file if it exists
	if data, err := os.ReadFile(dataFile); err == nil {
		if err := json.Unmarshal(data, &td); err != nil {
			slog.Debug("Failed to unmarshal cache file", "path", dataFile, "error", err)
		}
	}

	if td.TagsByTarget == nil {
		td.TagsByTarget = make(map[string][]string)
	}

	// add the new tags to the existing tags
	for target, tags := range tagsByTarget {
		for _, tag := range tags {
			if _, exists := td.TagsByTarget[target]; !exists {
				td.TagsByTarget[target] = []string{tag}
			} else {
				td.TagsByTarget[target] = append(td.TagsByTarget[target], tag)
			}

			// keep at most 10 tags per target
			if len(td.TagsByTarget[target]) > 10 {
				td.TagsByTarget[target] = td.TagsByTarget[target][len(td.TagsByTarget[target])-10:]
			}
		}
		td.TagsByTarget[target] = lo.Uniq(td.TagsByTarget[target])
	}

	td.LastUpdatedAt = time.Now().UTC()

	return fileutil.SaveJSON(dataFile, td)
}

func ForgetCacheEntriesOlderThan(forgetTime time.Time, cacheDir string) error {
	slog.Debug("Forgetting cache entries older than", "forgetTime", forgetTime, "cacheDir", cacheDir)

	deletedCount := 0
	err := filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		slog.Debug("Checking cache file", "path", path)

		var cacheFile CacheFile
		if err := json.Unmarshal(data, &cacheFile); err != nil {
			slog.Error("Failed to unmarshal cache file", "path", path, "error", err)
			return nil
		}

		if cacheFile.LastUpdatedAt.After(forgetTime) {
			slog.Debug("Cache file is newer than forget time, skipping deletion", "path", path)
			return nil
		}

		slog.Debug("Cache file is older than forget time, deleting", "path", path)
		if err := os.Remove(path); err != nil {
			slog.Error("Failed to delete cache file", "path", path, "error", err)
			return nil
		}

		deletedCount++
		return nil
	})

	slog.Info("Deleted cache entries older than", "count", deletedCount, "forgetTime", forgetTime)

	return err
}
