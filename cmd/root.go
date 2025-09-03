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
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	logger.InitLogging(nil)

	rootCmd.Flags().BoolP(versionFlag, "v", false, "Show the version")
	rootCmd.Flags().BoolP(debugFlag, "", false, "Show debug logs")
}
