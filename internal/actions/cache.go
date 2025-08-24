package actions

import (
	"fmt"
	"time"

	"github.com/hytromo/mimosa/internal/cacher"
	"github.com/hytromo/mimosa/internal/logger"
	log "github.com/sirupsen/logrus"
)

func (a *Actioner) GetCacheEntry(hash string) cacher.Cache {
	return cacher.Cache{
		Hash:            hash,
		InMemoryEntries: cacher.GetAllInMemoryEntries(),
	}
}

func (a *Actioner) RemoveCacheEntry(cacheEntry cacher.Cache, dryRun bool) error {
	return cacheEntry.Remove(dryRun)
}

func (a *Actioner) SaveCache(cacheEntry cacher.Cache, tagsByTarget map[string][]string, dryRun bool) error {
	return cacheEntry.Save(tagsByTarget, dryRun)
}

func (a *Actioner) ForgetCacheEntriesOlderThan(duration string, autoApprove bool) error {
	forgetDuration, err := parseDuration("0s") // purge

	if duration != "" {
		forgetDuration, err = parseDuration(duration)
		if err != nil {
			log.Errorf("Invalid forget duration: %v", err)
			return err
		}
	}

	forgetTime := time.Now().UTC().Add(-forgetDuration)
	if !autoApprove {
		// need to ask for confirmation
		logger.CleanLog.Infof("Are you sure you want to forget cache entries older than %s? (y/n): ", forgetTime)
		var response string
		_, err := fmt.Scanln(&response)
		if err != nil || (response != "yes" && response != "y") {
			log.Infoln("Cache forget operation cancelled.")
			return nil
		}
	}

	return cacher.ForgetCacheEntriesOlderThan(forgetTime)
}

func (a *Actioner) PrintCacheDir() {
	// use CleanLog
	logger.CleanLog.Info(cacher.CacheDir)
}

func (a *Actioner) PrintCacheToEnvValue() {
	diskEntries := cacher.GetDiskCacheToMemoryEntries()
	log.Debugln("-- Disk Cache Entries --")
	for key, value := range diskEntries.AllFromFront() {
		logger.CleanLog.Infoln(fmt.Sprintf("%s %s", key, value))
	}

	envEntries := cacher.GetAllInMemoryEntries()
	log.Debugln("-- Env Cache Entries --")
	for key, value := range envEntries.AllFromFront() {
		if _, exists := diskEntries.Get(key); !exists {
			// print only entries that are not in the disk cache
			logger.CleanLog.Infoln(fmt.Sprintf("%s %s", key, value))
		}
	}
}
