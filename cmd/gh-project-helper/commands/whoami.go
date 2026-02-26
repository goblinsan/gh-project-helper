package commands

import (
	"context"
	"fmt"

	"github.com/goblinsan/gh-project-helper/pkg/github"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(whoamiCmd)
}

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Prints the logged in user's login",
	Long:  `This command prints the login of the user that is currently logged in.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := github.NewClient()
		if err != nil {
			return err
		}
		user, err := client.GetAuthenticatedUser(context.Background())
		if err != nil {
			return err
		}
		fmt.Println(*user.Login)
		return nil
	},
}
