package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/goblinsan/gh-project-helper/pkg/engine"
	"github.com/goblinsan/gh-project-helper/pkg/github"
	"github.com/goblinsan/gh-project-helper/pkg/types"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func init() {
	rootCmd.AddCommand(applyCmd)
	applyCmd.Flags().StringP("file", "f", "", "The plan file to apply")
	applyCmd.MarkFlagRequired("file")
	applyCmd.Flags().Bool("dry-run", false, "Preview what would be created without making changes")
}

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply a project plan from a YAML file",
	Long:  `Apply a project plan from a YAML file to create GitHub projects, epics, and issues.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath, _ := cmd.Flags().GetString("file")

		// Read the YAML file
		yamlFile, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}

		// Unmarshal the YAML file into a Plan struct
		var plan types.Plan
		err = yaml.Unmarshal(yamlFile, &plan)
		if err != nil {
			return fmt.Errorf("failed to unmarshal YAML: %w", err)
		}

		// Create a new GitHub client
		client, err := github.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create github client: %w", err)
		}

		dryRun, _ := cmd.Flags().GetBool("dry-run")
		report, err := engine.ApplyPlan(context.Background(), client, plan, engine.Options{
			DryRun: dryRun,
		})
		if err != nil {
			return err
		}
		if report != nil {
			fmt.Println(report)
		}
		return nil
	},
}
