package orchestrator

import (
	"github.com/hytromo/mimosa/internal/cacher"
	"github.com/hytromo/mimosa/internal/configuration"
	"github.com/hytromo/mimosa/internal/orchestration/actions"
)

func handleCacheSubcommand(appOptions configuration.AppOptions, act actions.Actions) error {
	if appOptions.Cache.Forget != "" || appOptions.Cache.Purge {
		return act.ForgetCacheEntriesOlderThan(appOptions.Cache.Forget, appOptions.Cache.ForgetYes)
	}

	if appOptions.Cache.Show {
		act.PrintCacheDir()
		return nil
	}

	if appOptions.Cache.ToEnvValue {
		act.PrintCacheToEnvValue(cacher.CacheDir)
		return nil
	}

	return nil
}
