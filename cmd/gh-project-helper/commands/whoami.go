package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/goblinsan/gh-project-helper/pkg/github"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	rootCmd.AddCommand(whoamiCmd)
}

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Display information about the authenticated GitHub user",
	Long:  `Display information about the authenticated GitHub user using the provided token.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		token := viper.GetString("token")
		if token == "" {
			return fmt.Errorf("GitHub token is required. Set it via --token flag, GH_PROJECT_HELPER_TOKEN environment variable, or config file")
		}

		client := github.NewClient(token)
		user, err := client.GetAuthenticatedUser(context.Background())
		if err != nil {
			return fmt.Errorf("failed to get authenticated user: %w", err)
		}

		fmt.Fprintf(os.Stdout, "Logged in as: %s\n", user.GetLogin())
		if user.GetName() != "" {
			fmt.Fprintf(os.Stdout, "Name: %s\n", user.GetName())
		}
		if user.GetEmail() != "" {
			fmt.Fprintf(os.Stdout, "Email: %s\n", user.GetEmail())
		}
		
		return nil
	},
}
