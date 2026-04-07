package commands

import (
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"
)

var (
	// Version is the human-managed base version of the application.
	Version = "1.0.42"
	// Commit is an optional build-time override for the git commit.
	Commit = ""
	// Date is an optional build-time override for the build timestamp.
	Date = ""
)

func init() {
	rootCmd.AddCommand(versionCmd)
}

type versionMetadata struct {
	commit string
	date   string
	dirty  bool
}

func readVersionMetadata() versionMetadata {
	meta := versionMetadata{
		commit: strings.TrimSpace(Commit),
		date:   strings.TrimSpace(Date),
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return meta
	}

	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			if meta.commit == "" {
				meta.commit = setting.Value
			}
		case "vcs.time":
			if meta.date == "" {
				meta.date = setting.Value
			}
		case "vcs.modified":
			meta.dirty = setting.Value == "true"
		}
	}

	return meta
}

func shortCommit(commit string) string {
	if len(commit) > 7 {
		return commit[:7]
	}
	return commit
}

func formatVersionString(base string, meta versionMetadata) string {
	version := strings.TrimSpace(base)
	if version == "" {
		version = "dev"
	}

	if meta.commit != "" {
		version = fmt.Sprintf("%s+g.%s", version, shortCommit(meta.commit))
	}
	if meta.dirty {
		version += ".dirty"
	}

	return version
}

func formatVersionOutput() string {
	meta := readVersionMetadata()
	return fmt.Sprintf(
		"gh-project-helper version %s, commit %s, built at %s\n",
		formatVersionString(Version, meta),
		valueOrUnknown(meta.commit),
		valueOrUnknown(meta.date),
	)
}

func valueOrUnknown(value string) string {
	if strings.TrimSpace(value) == "" {
		return "unknown"
	}
	return value
}

func printVersion() {
	fmt.Print(formatVersionOutput())
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of gh-project-helper",
	Long:  `All software has versions. This is gh-project-helper's`,
	Run: func(cmd *cobra.Command, args []string) {
		printVersion()
	},
}
