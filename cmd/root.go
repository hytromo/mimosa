package cmd

import (
	"os"

	"github.com/hytromo/mimosa/internal/logger"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "mimosa",
	Short: "Zero-config docker image promotion",
	Long:  `Mimosa creates a unique hash<->tag association for each docker build - for the same build, mimosa will retag your image, instead of building it again!`,
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
