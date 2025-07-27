package main

import (
	"encoding/json"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

	"os"

	"github.com/hytromo/mimosa/internal/argsparser"
	"github.com/hytromo/mimosa/internal/cacher"
	"github.com/hytromo/mimosa/internal/docker"
)

func main() {
	initLogging()

	appOptions, err := argsparser.Parse(os.Args)

	if err != nil {
		// we could not even parse our own arguments, we cannot recover from this
		log.Fatal(err.Error())
	}

	if appOptions.Remember.Enabled {
		parsedBuildCommand, err := docker.ParseBuildCommand(appOptions.Remember.CommandToRun)

		if log.IsLevelEnabled(log.DebugLevel) {
			jsonOfParsedCommand, err := json.MarshalIndent(parsedBuildCommand, "", "  ")
			if err == nil {
				log.Debugln("Parsed build command:")
				log.Debugln(string(jsonOfParsedCommand))
			}
		}

		if err != nil {
			// we run the provided command anyway even if it is not parsable as a valid docker command
			// as the philosophy of mimosa is to not block the user
			log.Errorln(err.Error())
			exitCode := docker.RunCommand(appOptions.Remember.CommandToRun, appOptions.Remember.DryRun)
			os.Exit(exitCode)
			return
		}

		cache, cacheInitError := cacher.GetCache(parsedBuildCommand)

		if cacheInitError != nil {
			log.Errorln(cacheInitError.Error())
		}

		if cacheInitError == nil && cache.Exists() {
			latestCachedTag, err := cache.LatestTag()
			if err == nil {
				log.Debugln("The tag", parsedBuildCommand.FinalTag, "will point now to", latestCachedTag)
				err := docker.Retag(latestCachedTag, parsedBuildCommand.FinalTag, appOptions.Remember.DryRun)
				if err == nil {
					dataFile, err := cache.Save(parsedBuildCommand.FinalTag, appOptions.Remember.DryRun)
					if err != nil {
						log.Errorln("Failed to save to cache:", err)
					}
					log.Debugln("Saved tag in file", dataFile)
					log.Infoln("mimosa-cache-hit: true")
					return
				} else {
					log.Errorln("Failed to retag image:", err)
					log.Errorln("The build will be executed normally")
					// given that we failed to retag, we will run the command anyway
				}
			}
		}

		exitCode := docker.RunCommand(appOptions.Remember.CommandToRun, appOptions.Remember.DryRun)

		if exitCode != 0 {
			// the docker build command itself failed, so we need to follow and exit
			// we choose the same exit status as docker, for compatibility
			os.Exit(exitCode)
			return
		}

		if cacheInitError == nil {
			// build was successful, let's save the cache entry
			dataFile, err := cache.Save(parsedBuildCommand.FinalTag, appOptions.Remember.DryRun)
			if err != nil {
				log.Errorf("Failed to save to cache: %v", err)
			} else {
				log.Debugln("Saved tag in file", dataFile)
			}

			log.Infoln("mimosa-cache-hit: false")
		}
	} else if appOptions.Cache.Enabled {
		if appOptions.Cache.Forget != "" {
			forgetDuration, err := argsparser.ParseDuration(appOptions.Cache.Forget)
			if err != nil {
				log.Errorf("Invalid forget duration: %v", err)
				return
			}

			forgetTime := time.Now().UTC().Add(-forgetDuration)
			if !appOptions.Cache.ForgetYes {
				// need to ask for confirmation
				log.Infof("Are you sure you want to forget cache entries older than %s? (y/n): ", forgetTime)
				var response string
				_, err := fmt.Scanln(&response)
				if err != nil || (response != "yes" && response != "y") {
					log.Infoln("Cache forget operation cancelled.")
					return
				}
			}

			cacher.ForgetCacheEntriesOlderThan(forgetTime)
		} else if appOptions.Cache.Show {
			CleanLog.Infoln(cacher.CacheDir)
		} else if appOptions.Cache.ToEnvValue {
			diskEntries := cacher.GetDiskCacheToMemoryEntries()
			log.Debugln("-- Disk Cache Entries --")
			for key, value := range diskEntries.AllFromFront() {
				CleanLog.Infoln(fmt.Sprintf("%s %s", key, value))
			}

			envEntries := cacher.GetAllInMemoryEntries()
			log.Debugln("-- Env Cache Entries --")
			for key, value := range envEntries.AllFromFront() {
				if _, exists := diskEntries.Get(key); !exists {
					// print only entries that are not in the disk cache
					CleanLog.Infoln(fmt.Sprintf("%s %s", key, value))
				}
			}
		}

	}

}
