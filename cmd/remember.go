package cmd

import (
	"log/slog"

	"github.com/hytromo/mimosa/internal/configuration"
	"github.com/hytromo/mimosa/internal/orchestration/actions"
	"github.com/hytromo/mimosa/internal/orchestration/orchestrator"
	"github.com/spf13/cobra"
)

var rememberCmd = &cobra.Command{
	Use:   "remember [flags] -- <docker buildx build/bake command>",
	Short: "Build new images, or retag existing ones",
	Long: `The remember subcommand will run the provided command as is and store the hash-tag association in the cache. If the same command is run again under the same context, mimosa will retag the docker image instead of rebuilding it.

  * buildx build
    If the same hash has been seen before, mimosa will use the cached tag to retag your image instead of building it.

    Example:
      # mimosa is seeing this command under this context for the first time, so it doesn't remember!
      # it runs normally the command after the "--" and it saves the result in the cache
      mimosa remember -- docker buildx build --platform linux/amd64,linux/arm64 --push -t org/image:v1 .

      # ... introduce changes in .dockerignored files (or other irrelevant files) ...

      # mimosa now remembers! This retags v1 to v2 without rebuilding the image and the cache is updated to contain v2 as the latest known tag for this command
      mimosa remember -- docker buildx build --platform linux/amd64,linux/arm64 --push -t org/image:v2 .

  * buildx bake
    Bake works the same as build - a single hash is generated for the bake command regardless of how many targets are defined inside the bake file. This means that either all targets are retagged (cache hit) all the whole "docker buildx bake" command is run and cached (cache miss). This follows mimosa's philosophy of not changing the original command's behavior on cache miss (like breaking down a single bake command into multiple build commands).

    Example:
      # mimosa doesn't remember! - it runs normally the command following it and it saves it in its cache
      mimosa remember -- docker buildx bake -f docker-bake.hcl

      # ... introduce changes in .dockerignored-files (or other irrelevant files) ...

      # mimosa now remembers! This retags all the targets to their new tags and the cache is updated to contain all the new tags as the latest ones for this entry
      mimosa remember -- docker buildx bake -f docker-bake.hcl`,
	Run: func(cmd *cobra.Command, positionalArgs []string) {
		dryRun, _ := cmd.Flags().GetBool(dryRunFlag)

		err := orchestrator.HandleRememberOrForgetSubcommands(
			configuration.RememberSubcommandOptions{
				Enabled:      true,
				DryRun:       dryRun,
				CommandToRun: positionalArgs,
			}, configuration.ForgetSubcommandOptions{},
			actions.New())

		if err != nil {
			slog.Error(err.Error())
		}
	},
}

func init() {
	rootCmd.AddCommand(rememberCmd)

	rememberCmd.Flags().BoolP(dryRunFlag, "", false, "Dry run - do not really build or push anything - just show if it would be a cache hit or not")
}
