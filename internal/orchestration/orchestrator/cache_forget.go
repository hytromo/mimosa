package orchestrator

import (
	"github.com/hytromo/mimosa/internal/configuration"
	"github.com/hytromo/mimosa/internal/orchestration/actions"
)

func HandleForgetPeriodOrEverything(forgetOptions configuration.ForgetSubcommandOptions, act actions.Actions) error {
	if forgetOptions.Period != "" || forgetOptions.Everything {
		return act.ForgetCacheEntriesOlderThan(forgetOptions.Period, forgetOptions.AutoYes)
	}

	return nil
}
