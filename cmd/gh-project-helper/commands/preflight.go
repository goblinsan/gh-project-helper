package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/goblinsan/gh-project-helper/pkg/engine"
	"github.com/goblinsan/gh-project-helper/pkg/github"
	"github.com/goblinsan/gh-project-helper/pkg/types"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func init() {
	rootCmd.AddCommand(preflightCmd)
	preflightCmd.Flags().StringP("file", "f", "", "The plan file to preflight")
	_ = preflightCmd.MarkFlagRequired("file")
}

var preflightCmd = &cobra.Command{
	Use:   "preflight",
	Short: "Resolve plan readiness and repository targeting without applying changes",
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath, _ := cmd.Flags().GetString("file")
		yamlFile, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}

		var plan types.Plan
		if err := yaml.Unmarshal(yamlFile, &plan); err != nil {
			report := engine.PreflightReport{
				Status:     engine.PreflightStatusInvalid,
				Errors:     []string{fmt.Sprintf("invalid YAML: %v", err)},
				Project:    engine.ProjectPreflight{Name: plan.Project},
				Repository: engine.RepositoryPreflight{Requested: plan.Repository},
			}
			return json.NewEncoder(os.Stdout).Encode(report)
		}

		if errs := validatePlan(plan); len(errs) > 0 {
			report := engine.PreflightReport{
				Status:     engine.PreflightStatusInvalid,
				Errors:     errs,
				Project:    engine.ProjectPreflight{Name: plan.Project},
				Repository: engine.RepositoryPreflight{Requested: plan.Repository},
			}
			return json.NewEncoder(os.Stdout).Encode(report)
		}

		client, err := github.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create github client: %w", err)
		}

		report, err := engine.PreflightPlan(context.Background(), client, plan)
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(report)
	},
}
