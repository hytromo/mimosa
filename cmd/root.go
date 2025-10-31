package cmd

import (
	"os"

	"github.com/hytromo/mimosa/internal/logger"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "mimosa",
	Short: "Zero-config docker image promotion",
	Long:  `Mimosa saves a unique hash for each docker build - if it bumps into the same exact build, it will simply retag your image instead of rebuilding it.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		forceDebug, _ := cmd.Flags().GetBool(debugFlag)
		logger.InitLogging(forceDebug)
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().Bool(debugFlag, false, "Show debug logs")
}
