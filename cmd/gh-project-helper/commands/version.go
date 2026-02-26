package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	// Version is the version of the application
	Version = "dev"
	// Commit is the git commit of the build
	Commit = "none"
	// Date is the date of the build
	Date = "unknown"
)

func init() {
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of gh-project-helper",
	Long:  `All software has versions. This is gh-project-helper's`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("gh-project-helper version %s, commit %s, built at %s\n", Version, Commit, Date)
	},
}
