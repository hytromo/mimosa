package actions

import (
	"fmt"
	"os"
	"time"

	"log/slog"

	"github.com/hytromo/mimosa/internal/cacher"
	"github.com/hytromo/mimosa/internal/logger"
)

func (a *Actioner) GetCacheEntry(hash string) cacher.Cache {
	return cacher.Cache{
		Hash:            hash,
		CacheDir:        cacher.CacheDir,
		InMemoryEntries: cacher.GetAllInMemoryEntries(),
	}
}

func (a *Actioner) RemoveCacheEntry(cacheEntry cacher.Cache, dryRun bool) error {
	return cacheEntry.Remove(dryRun)
}

func (a *Actioner) SaveCache(cacheEntry cacher.Cache, tagsByTarget map[string][]string, dryRun bool) error {
	return cacheEntry.Save(tagsByTarget, dryRun)
}

func (a *Actioner) ForgetCacheEntriesOlderThan(duration string, autoApprove bool, dryRun bool) error {
	if duration == "" {
		duration = "0s" // purge
	}

	forgetDuration, err := parseDuration(duration)

	if err != nil {
		slog.Error("Invalid forget duration", "error", err)
		return err
	}

	forgetTime := time.Now().UTC().Add(-forgetDuration)
	if !autoApprove {
		// need to ask for confirmation
		logger.CleanLog.Info(fmt.Sprintf("Are you sure you want to forget cache entries older than %s? (y/n): ", forgetTime))
		var response string
		_, err := fmt.Scanln(&response)
		if err != nil || (response != "yes" && response != "y") {
			slog.Info("Cache forget operation cancelled.")
			return nil
		}
	}

	return cacher.ForgetCacheEntriesOlderThan(forgetTime, cacher.CacheDir, dryRun)
}

func (a *Actioner) PrintCacheDir() {
	logger.CleanLog.Info(cacher.CacheDir)
}

func (a *Actioner) ExportCacheToFile(cacheDir string, filePath string) error {
	file, err := os.Create(filePath)

	if err != nil {
		return err
	}

	diskEntries := cacher.GetDiskCacheToMemoryEntries(cacheDir)

	slog.Debug("-- Disk Cache Entries --")
	for z85Key, value := range diskEntries.AllFromFront() {
		// print the entry to the file
		slog.Debug("entry", "key", z85Key, "value", value)
		_, err = fmt.Fprintf(file, "%s %s\n", z85Key, value)
		if err != nil {
			return err
		}
	}

	slog.Debug("-- Env Cache Entries --")
	for z85Key, value := range cacher.GetSeparatedInMemoryEntries() {
		if _, ok := diskEntries.Get(z85Key); ok {
			slog.Debug("skipping duplicate entry", "key", z85Key, "value", value)
			continue
		}
		slog.Debug("entry", "key", z85Key, "value", value)
		_, err = fmt.Fprintf(file, "%s %s\n", z85Key, value)

		if err != nil {
			return err
		}
	}

	return nil
}
