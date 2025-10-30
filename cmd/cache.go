package cmd

import (
	"github.com/hytromo/mimosa/internal/configuration"
	"github.com/hytromo/mimosa/internal/orchestration/actions"
	"github.com/hytromo/mimosa/internal/orchestration/orchestrator"
	"github.com/spf13/cobra"
)

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Cache related utilities",
	Long: `Find where the mimosa cache is stored and how it can be exported as an environment variable.
Use the MIMOSA_CACHE_DIR environment variable to override the default cache location.`,
	Run: func(cmd *cobra.Command, args []string) {
		cacheShow, _ := cmd.Flags().GetBool(showFlag)
		cacheToEnvValue, _ := cmd.Flags().GetBool(toEnvValueFlag)

		err := orchestrator.HandleCacheSubcommand(configuration.CacheSubcommandOptions{
			Enabled:    true,
			Show:       cacheShow,
			ToEnvValue: cacheToEnvValue,
		}, actions.New())

		if err != nil {
			panic(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(cacheCmd)

	cacheCmd.Flags().BoolP(showFlag, "s", false, "Show the cache directory")
	cacheCmd.Flags().BoolP(toEnvValueFlag, "", false, "Print the mimosa cache as a string to stdout")
}
