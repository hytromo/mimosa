package cmd

import (
	"log/slog"

	"github.com/hytromo/mimosa/internal/configuration"
	"github.com/hytromo/mimosa/internal/orchestration/actions"
	"github.com/hytromo/mimosa/internal/orchestration/orchestrator"
	"github.com/spf13/cobra"
)

var forgetCmd = &cobra.Command{
	Use:   "forget [flags] [-- <docker buildx build/bake command>]",
	Short: "Forget cache entries",
	Long: `Forget can be used to forget cache entries - either by using the same syntax as the "remember" subcommand or by passing one of the corresponding flags.

  Examples:
    mimosa forget --dry-run -- docker buildx build --platform linux/amd64,linux/arm64 --push -t org/image:v1 .
    mimosa forget --everything
    mimosa forget --older-than 1h
  `,
	Run: func(cmd *cobra.Command, positionalArguments []string) {
		dryRun, _ := cmd.Flags().GetBool(dryRunFlag)

		if len(positionalArguments) > 0 {
			err := orchestrator.HandleRememberOrForgetSubcommands(configuration.RememberSubcommandOptions{},
				configuration.ForgetSubcommandOptions{
					Enabled:      true,
					DryRun:       dryRun,
					CommandToRun: positionalArguments,
				}, actions.New())

			if err != nil {
				slog.Error(err.Error())
			}
			return
		}

		everything, _ := cmd.Flags().GetBool(everythingFlag)
		olderThan, _ := cmd.Flags().GetString(olderThanFlag)
		yes, _ := cmd.Flags().GetBool(yesFlag)
		err := orchestrator.HandleForgetPeriodOrEverything(configuration.ForgetSubcommandOptions{
			Enabled:    true,
			DryRun:     dryRun,
			Everything: everything,
			Period:     olderThan,
			AutoYes:    yes,
		}, actions.New())

		if err != nil {
			panic(err)
		}

	},
}

func init() {
	rootCmd.AddCommand(forgetCmd)

	forgetCmd.Flags().BoolP(dryRunFlag, "", false, "Dry run - do not actually delete any cache entry; just show what would happen")
	forgetCmd.Flags().BoolP(everythingFlag, "", false, "Forget all cache entries")
	forgetCmd.Flags().StringP(olderThanFlag, "", "", "Forget cache entries older than the given age, e.g. 1h, 2d etc.")
	forgetCmd.Flags().BoolP(yesFlag, "y", false, "Do not ask for user confirmation before cache deletion")
}
