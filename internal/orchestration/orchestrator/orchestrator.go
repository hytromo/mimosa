package orchestrator

import (
	"github.com/hytromo/mimosa/internal/configuration"
	"github.com/hytromo/mimosa/internal/orchestration/actions"
)

func Run(appOptions configuration.AppOptions, act actions.Actions) error {
	if appOptions.Remember.Enabled || appOptions.Forget.Enabled {
		return handleRememberOrForgetSubcommands(appOptions, act)
	}

	if appOptions.Cache.Enabled {
		return handleCacheSubcommand(appOptions, act)
	}

	return nil
}
