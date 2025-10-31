package cmd

import (
	"log/slog"

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
		exportToFile, _ := cmd.Flags().GetString(exportCacheToFileFlag)

		err := orchestrator.HandleCacheSubcommand(configuration.CacheSubcommandOptions{
			Enabled:      true,
			Show:         cacheShow,
			ExportToFile: exportToFile,
		}, actions.New())

		if err != nil {
			slog.Error(err.Error())
		}
	},
}

func init() {
	rootCmd.AddCommand(cacheCmd)

	cacheCmd.Flags().BoolP(showFlag, "s", false, "Show the cache directory")
	cacheCmd.Flags().StringP(exportCacheToFileFlag, "", "", "Export the mimosa cache to a file using z85 encoding")
}
