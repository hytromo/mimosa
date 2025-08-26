package cacher

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/apparentlymart/go-userdirs/userdirs"
	"github.com/hytromo/mimosa/internal/docker"
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
	Hash            string         // the final hash of the current command and files
}

func (cache *Cache) DataPath() string {
	return filepath.Join(CacheDir, cache.Hash+".json")
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
		latestTagByTarget[target] = tags[len(tags)-1]
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
		log.Infoln("> DRY RUN: cache entry would be removed from", cache.DataPath())
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
		log.Warnf("Failed to convert final hash to Z85: %v", err)
		return CacheFile{}, false
	}
	if entry, ok := cache.InMemoryEntries.Get(z85Hash); ok {
		return entry, true
	}

	return CacheFile{}, false
}

func (cache *Cache) Exists() bool {
	if _, ok := cache.GetInMemoryEntry(); ok {
		log.Debugf("Cache hit in memory for hash %s", cache.Hash)
		return true
	}

	if _, err := os.Stat(cache.DataPath()); errors.Is(err, os.ErrNotExist) {
		return false
	}

	log.Debugf("Cache hit on disk for hash %s", cache.Hash)

	return true
}

func GetCache(parsedBuildCommand docker.ParsedBuildCommand) (cache Cache, err error) {
	commandInfo := make([]string, len(parsedBuildCommand.CmdWithTagPlaceholder))
	copy(commandInfo, parsedBuildCommand.CmdWithTagPlaceholder)

	commandInfo = append(commandInfo, parsedBuildCommand.RegistryDomain)

	commandHash := hasher.HashStrings(commandInfo)

	log.Debugf("Command hash: %v - deducted from %v", commandHash, commandInfo)

	if _, err := os.Stat(parsedBuildCommand.ContextPath); errors.Is(err, os.ErrNotExist) {
		return cache, err
	}

	if _, err := os.Stat(parsedBuildCommand.DockerfilePath); errors.Is(err, os.ErrNotExist) {
		return cache, err
	}

	if parsedBuildCommand.DockerignorePath != "" {
		if _, err := os.Stat(parsedBuildCommand.DockerignorePath); errors.Is(err, os.ErrNotExist) {
			return cache, err
		}
	}

	files, err := fileutil.IncludedFiles(parsedBuildCommand.ContextPath, parsedBuildCommand.DockerignorePath)

	if err != nil {
		return cache, err
	}

	if parsedBuildCommand.DockerignorePath != "" {
		files = append(files, parsedBuildCommand.DockerignorePath)
	}

	if parsedBuildCommand.DockerfilePath != "" {
		files = append(files, parsedBuildCommand.DockerfilePath)
	}

	filesHash := hasher.HashFiles(files, runtime.NumCPU()-1)

	log.Debugf("Files hash: %v - deducted from %v files", filesHash, len(files))

	cache.Hash = hasher.HashStrings([]string{commandHash, filesHash})

	cache.Hash = "TODO"

	log.Debugf("Final hash of command and files: %v", cache.Hash)

	cache.InMemoryEntries = GetAllInMemoryEntries()

	return cache, nil
}

func (cache *Cache) Save(tagsByTarget map[string][]string, dryRun bool) error {
	dataFile := cache.DataPath()

	if dryRun {
		log.Infoln("> DRY RUN: cache entry would be saved to", dataFile)
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(dataFile), 0755); err != nil {
		return err
	}

	var td CacheFile

	// Read existing tags from mimosa-cache.json if it exists
	if data, err := os.ReadFile(dataFile); err == nil {
		if err := json.Unmarshal(data, &td); err != nil {
			log.Debugf("Failed to unmarshal cache file %s: %v", dataFile, err)
		}
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
	}

	td.LastUpdatedAt = time.Now().UTC()

	return fileutil.SaveJSON(dataFile, td)
}

func ForgetCacheEntriesOlderThan(forgetTime time.Time) error {
	cacheDir := CacheDir

	log.Debugf("Forgetting cache entries older than %s in %s", forgetTime, cacheDir)

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

		log.Debugln("Checking cache file:", path)

		var cacheFile CacheFile
		if err := json.Unmarshal(data, &cacheFile); err != nil {
			log.Errorf("Failed to unmarshal cache file %s: %v", path, err)
			return nil
		}

		if cacheFile.LastUpdatedAt.After(forgetTime) {
			log.Debugf("Cache file %s is newer than forget time, skipping deletion", path)
			return nil
		}

		log.Debugf("Cache file %s is older than forget time, deleting", path)
		if err := os.Remove(path); err != nil {
			log.Errorf("Failed to delete cache file %s: %v", path, err)
			return nil
		}

		deletedCount++
		return nil
	})

	log.Infoln("Deleted", deletedCount, "cache entries older than", forgetTime)

	return err
}
