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
		log.Fatal(err.Error())
	}

	if appOptions.Remember.Enabled {
		parsedBuildCommand, err := docker.ParseBuildCommand(appOptions.Remember.CommandToRun)

		if err != nil {
			log.Fatal(err.Error())
		}

		cache, cacheInitError := cacher.GetCache(parsedBuildCommand)

		if cacheInitError != nil {
			log.Errorln(cacheInitError.Error())
		}

		if cacheInitError == nil && cache.Exists() {
			latestCachedTag, err := cache.LatestTag()
			if err == nil {
				log.Debugln("Retagging", latestCachedTag, "to", parsedBuildCommand.FinalTag)
				err := docker.Retag(latestCachedTag, parsedBuildCommand.FinalTag)
				if err == nil {
					dataFile, err := cache.Save(parsedBuildCommand.FinalTag)
					if err != nil {
						log.Errorln("Failed to save to cache:", err)
					}
					log.Debugln("Saved tag in file", dataFile)
					log.Infoln("mimosa-cache-hit: true")
				} else {
					log.Errorln("Failed to retag image:", err)
				}

				return
			}
		}

		err = docker.RunCommand(parsedBuildCommand)

		if err != nil {
			log.Fatal(err.Error())
		}

		log.Debugln("Parsed command:", parsedBuildCommand)

		if cacheInitError == nil {
			dataFile, err := cache.Save(parsedBuildCommand.FinalTag)
			if err != nil {
				log.Fatalf("Failed to save to cache: %v", err)
			}

			log.Debugln("Saved tag in file", dataFile)
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
		}

		if appOptions.Cache.Show {
			CleanLog.Infoln(cacher.CacheDir)
		}

	}

}
