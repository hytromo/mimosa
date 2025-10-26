package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	// these are set dynamically via ldflags during build time
	Version = "unknown"
	Commit  = "unknown"
	Date    = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   versionFlag,
	Short: "Show the version",
	Long:  `Show the version of mimosa`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("mimosa version %s, commit %s, built at %s\n", Version, Commit, Date)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
