package main

import (
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

	if appOptions.Remember.Enabled || appOptions.Forget.Enabled {
		var commandToRun []string
		if appOptions.Remember.Enabled {
			commandToRun = appOptions.Remember.CommandToRun
		} else {
			commandToRun = appOptions.Forget.CommandToRun
		}

		parsedBuildCommand, err := docker.ParseBuildCommand(commandToRun)

		if err != nil {
			if appOptions.Forget.Enabled {
				// we need to be able to parse the command to forget
				log.Fatal(err.Error())
			}
			// we run the provided command anyway even if it is not parsable as a valid docker command
			// as the philosophy of mimosa is to not block the user
			log.Errorln(err.Error())
			exitCode := docker.RunCommand(commandToRun, appOptions.Remember.DryRun)
			os.Exit(exitCode)
			return
		}

		cache, cacheInitError := cacher.GetCache(parsedBuildCommand)

		if cacheInitError != nil {
			if appOptions.Forget.Enabled {
				log.Fatalf("Could not calculate hash: %s", cacheInitError.Error())
			}

			log.Errorln(cacheInitError.Error())
		}

		if cacheInitError == nil && cache.Exists() {
			if appOptions.Forget.Enabled {
				log.Infoln("Removing cache entry", cache.DataPath())
				err := cache.Remove(appOptions.Forget.DryRun)
				if err != nil {
					log.Fatalf("Failed to remove cache entry: %s", err.Error())
				}
				return
			}

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
		} else if appOptions.Forget.Enabled {
			log.Infof("Cache entry %v was not found", cache.FinalHash)
			os.Exit(1)
			return
		}

		exitCode := docker.RunCommand(commandToRun, appOptions.Remember.DryRun)

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
		if appOptions.Cache.Forget != "" || appOptions.Cache.Purge {
			forgetDuration, _ := argsparser.ParseDuration("0s") // purge

			if appOptions.Cache.Forget != "" {
				forgetDuration, err = argsparser.ParseDuration(appOptions.Cache.Forget)
				if err != nil {
					log.Errorf("Invalid forget duration: %v", err)
					return
				}
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
