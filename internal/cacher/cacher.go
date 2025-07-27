package cacher

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/elliotchance/orderedmap/v3"
	log "github.com/sirupsen/logrus"

	"github.com/apparentlymart/go-userdirs/userdirs"
	"github.com/hytromo/mimosa/internal/docker"
	"github.com/hytromo/mimosa/internal/hasher"
	"github.com/hytromo/mimosa/internal/utils/fileutil"
)

var CacheDir = userdirs.ForApp("mimosa", "hytromo", "mimosa.hytromo.com").CacheDir

type CacheFile struct {
	Tags          []string  `json:"tags"`
	LastUpdatedAt time.Time `json:"lastUpdatedAt"`
}

type Cache struct {
	InMemoryEntries *orderedmap.OrderedMap[string, string] // populated by the MIMOSA_CACHE environment variable and taking precedence over the cache directory
	FinalHash       string                                 // the final hash of the current command and files
}

func (cache *Cache) DataPath() string {
	return filepath.Join(CacheDir, cache.FinalHash+".json")
}

func (cache *Cache) Remove(dryRun bool) error {
	if dryRun {
		log.Infoln("> DRY RUN: cache entry would be removed from", cache.DataPath())
		return nil
	}

	return os.Remove(cache.DataPath())
}

func (cache *Cache) LatestTag() (string, error) {
	inMemoryEntry, ok := cache.GetInMemoryEntry()

	if ok {
		log.Debugf("Returning in-memory cache entry for hash %s: %s", cache.FinalHash, inMemoryEntry)
		return inMemoryEntry, nil
	}

	cachedFilePath := cache.DataPath()

	data, err := os.ReadFile(cachedFilePath)
	if err != nil {
		return "", err
	}

	var cacheFile CacheFile
	err = json.Unmarshal(data, &cacheFile)
	if err != nil {
		return "", err
	}
	if len(cacheFile.Tags) == 0 {
		return "", nil
	}

	return cacheFile.Tags[len(cacheFile.Tags)-1], nil
}

func (cache *Cache) GetInMemoryEntry() (string, bool) {
	if cache.InMemoryEntries.Len() == 0 {
		return "", false
	}

	// first checking the in-memory cache
	z85Hash, err := hasher.HexToZ85(cache.FinalHash)
	if err != nil {
		log.Warnf("Failed to convert final hash to Z85: %v", err)
		return "", false
	}
	if entry, ok := cache.InMemoryEntries.Get(z85Hash); ok {
		return entry, true
	}

	return "", false
}

func (cache *Cache) Exists() bool {
	if _, ok := cache.GetInMemoryEntry(); ok {
		log.Debugf("Cache hit in memory for hash %s", cache.FinalHash)
		return true
	}

	if _, err := os.Stat(cache.DataPath()); errors.Is(err, os.ErrNotExist) {
		return false
	}

	log.Debugf("Cache hit on disk for hash %s", cache.FinalHash)

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

	files := docker.IncludedFiles(parsedBuildCommand.ContextPath, parsedBuildCommand.DockerignorePath)

	if parsedBuildCommand.DockerignorePath != "" {
		files = append(files, parsedBuildCommand.DockerignorePath)
	}

	if parsedBuildCommand.DockerfilePath != "" {
		files = append(files, parsedBuildCommand.DockerfilePath)
	}

	filesHash, err := hasher.HashFiles(files)

	log.Debugf("Files hash: %v - deducted from %v files", filesHash, len(files))

	if err != nil {
		return cache, err
	}

	cache.FinalHash = hasher.HashStrings([]string{commandHash, filesHash})

	log.Debugf("Final hash of command and files: %v", cache.FinalHash)

	cache.InMemoryEntries = GetAllInMemoryEntries()

	return cache, nil
}

func (cache *Cache) Save(finalTag string, dryRun bool) (dataFile string, err error) {
	dataFile = cache.DataPath()

	if dryRun {
		log.Infoln("> DRY RUN: cache entry would be saved to", dataFile)
		return dataFile, nil
	}

	if err := os.MkdirAll(filepath.Dir(dataFile), 0755); err != nil {
		return dataFile, err
	}

	tags := make([]string, 0, 10)
	tagSet := make(map[string]struct{})

	// Read existing tags from mimosa-cache.json if it exists
	if data, err := os.ReadFile(dataFile); err == nil {
		var td CacheFile
		if err := json.Unmarshal(data, &td); err == nil {
			for _, tag := range td.Tags {
				if tag == "" {
					continue
				}
				if _, exists := tagSet[tag]; !exists {
					tags = append(tags, tag)
					tagSet[tag] = struct{}{}
				}
			}
		}
	}

	// Add new tag if unique at the end
	if _, exists := tagSet[finalTag]; !exists {
		tags = append(tags, finalTag)
	}
	// Keep only last 10 unique tags (= the last 10 tags of the list)
	if len(tags) > 10 {
		tags = tags[len(tags)-10:]
	}

	return dataFile, fileutil.SaveJSON(dataFile, CacheFile{
		Tags:          tags,
		LastUpdatedAt: time.Now().UTC(),
	})
}

func ForgetCacheEntriesOlderThan(forgetTime time.Time) {
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

	if err != nil {
		log.Errorf("Failed to forget cache entries: %v", err)
	}

	log.Infoln("Deleted", deletedCount, "cache entries older than", forgetTime)
}
