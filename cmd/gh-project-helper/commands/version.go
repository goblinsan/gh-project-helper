package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

func init() {
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version information",
	Long:  `Print the version, commit hash, and build date of gh-project-helper.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("gh-project-helper version %s\n", Version)
		fmt.Printf("  commit: %s\n", Commit)
		fmt.Printf("  built: %s\n", BuildDate)
	},
}
