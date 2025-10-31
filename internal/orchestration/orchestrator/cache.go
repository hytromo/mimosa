package orchestrator

import (
	"github.com/hytromo/mimosa/internal/cacher"
	"github.com/hytromo/mimosa/internal/configuration"
	"github.com/hytromo/mimosa/internal/orchestration/actions"
)

func HandleCacheSubcommand(cacheOptions configuration.CacheSubcommandOptions, act actions.Actions) error {
	if cacheOptions.Show {
		act.PrintCacheDir()
		return nil
	}

	if cacheOptions.ExportToFile != "" {
		return act.ExportCacheToFile(cacher.CacheDir, cacheOptions.ExportToFile)
	}

	return nil
}
