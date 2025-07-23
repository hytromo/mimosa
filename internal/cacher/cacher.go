package cacher

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/apparentlymart/go-userdirs/userdirs"
	"github.com/hytromo/mimosa/internal/docker"
	"github.com/hytromo/mimosa/internal/hasher"
	"github.com/hytromo/mimosa/internal/utils/fileutil"
)

var UserDirs = userdirs.ForApp("mimosa", "hytromo", "mimosa.hytromo.com")

type CacheFile struct {
	Tags          []string  `json:"tags"`
	LastUpdatedAt time.Time `json:"lastUpdatedAt"`
}

type Cache struct {
	CommandHash string // The hash of the docker build command itself
	FilesHash   string // The hash of all the related files
}

func (cache *Cache) DataPath() string {
	return filepath.Join(UserDirs.CacheDir, cache.CommandHash, cache.FilesHash, "mimosa-cache.json")
}

func (cache *Cache) LatestTag() (string, error) {
	filename := cache.DataPath()

	data, err := os.ReadFile(filename)
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

func (cache *Cache) Exists() bool {
	filename := cache.DataPath()

	if _, err := os.Stat(filename); errors.Is(err, os.ErrNotExist) {
		return false
	}

	return true
}

func GetCache(parsedBuildCommand docker.ParsedBuildCommand) (cache Cache, err error) {
	commandInfo := make([]string, len(parsedBuildCommand.CmdWithTagPlaceholder))
	copy(commandInfo, parsedBuildCommand.CmdWithTagPlaceholder)

	commandInfo = append(commandInfo, parsedBuildCommand.RegistryDomain)

	log.Debugf("Deducting command hash from %v\n", commandInfo)

	commandHash := hasher.HashStrings(commandInfo)

	files := docker.IncludedFiles(parsedBuildCommand.ContextPath, parsedBuildCommand.DockerignorePath)

	if parsedBuildCommand.DockerignorePath != "" {
		files = append(files, parsedBuildCommand.DockerignorePath)
	}

	if parsedBuildCommand.DockerfilePath != "" {
		files = append(files, parsedBuildCommand.DockerfilePath)
	}

	filesHash, err := hasher.HashFiles(files)

	if err != nil {
		return Cache{}, err
	}

	return Cache{
		CommandHash: commandHash,
		FilesHash:   filesHash,
	}, nil
}

func (cache *Cache) Save(finalTag string) (dataFile string, err error) {
	dataFile = cache.DataPath()

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
	// Add new tag if unique
	if _, exists := tagSet[finalTag]; !exists {
		tags = append(tags, finalTag)
	}
	// Keep only last 10 unique tags
	if len(tags) > 10 {
		tags = tags[len(tags)-10:]
	}

	return dataFile, fileutil.SaveJSON(dataFile, CacheFile{
		Tags:          tags,
		LastUpdatedAt: time.Now().UTC(),
	}, false)
}

func ForgetCacheEntriesOlderThan(forgetTime time.Time) {
	cacheDir := UserDirs.CacheDir

	log.Debugf("Forgetting cache entries older than %s in %s", forgetTime, cacheDir)

	deletedCount := 0
	err := filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() || !strings.HasSuffix(path, "mimosa-cache.json") {
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

	if deletedCount > 0 {
		fileutil.DeleteEmptyDirectories(cacheDir)
		if err != nil {
			log.Errorf("Failed to delete empty directories in %s: %v", cacheDir, err)
		}
	}

	if err != nil {
		log.Errorf("Failed to forget cache entries: %v", err)
	}

	log.Infoln("Deleted", deletedCount, "cache entries older than", forgetTime)
}
