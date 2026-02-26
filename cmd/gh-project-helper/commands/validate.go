package commands

import (
	"fmt"
	"os"

	"github.com/goblinsan/gh-project-helper/pkg/types"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func init() {
	rootCmd.AddCommand(validateCmd)
	validateCmd.Flags().StringP("file", "f", "", "The plan file to validate")
	validateCmd.MarkFlagRequired("file")
}

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate a plan file without making any changes",
	Long:  `Validate a plan YAML file for correctness. Checks structure, required fields, and referential integrity (e.g. epic milestones reference defined milestones).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath, _ := cmd.Flags().GetString("file")

		yamlFile, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}

		var plan types.Plan
		if err := yaml.Unmarshal(yamlFile, &plan); err != nil {
			return fmt.Errorf("invalid YAML: %w", err)
		}

		errs := validatePlan(plan)
		if len(errs) > 0 {
			fmt.Fprintf(os.Stderr, "Validation failed with %d error(s):\n", len(errs))
			for i, e := range errs {
				fmt.Fprintf(os.Stderr, "  %d. %s\n", i+1, e)
			}
			os.Exit(1)
		}

		fmt.Println("Plan is valid.")
		return nil
	},
}

func validatePlan(plan types.Plan) []string {
	var errs []string

	if plan.Repository == "" {
		errs = append(errs, "repository is required")
	} else {
		parts := splitRepo(plan.Repository)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			errs = append(errs, fmt.Sprintf("repository %q must be in owner/repo format", plan.Repository))
		}
	}

	if plan.Project == "" {
		errs = append(errs, "project is required")
	}

	// Build milestone index for referential integrity checks
	milestoneSet := make(map[string]bool)
	for i, m := range plan.Milestones {
		if m.Title == "" {
			errs = append(errs, fmt.Sprintf("milestones[%d]: title is required", i))
			continue
		}
		if milestoneSet[m.Title] {
			errs = append(errs, fmt.Sprintf("milestones[%d]: duplicate title %q", i, m.Title))
		}
		milestoneSet[m.Title] = true
	}

	epicTitles := make(map[string]bool)
	for i, epic := range plan.Epics {
		if epic.Title == "" {
			errs = append(errs, fmt.Sprintf("epics[%d]: title is required", i))
			continue
		}
		if epicTitles[epic.Title] {
			errs = append(errs, fmt.Sprintf("epics[%d]: duplicate title %q", i, epic.Title))
		}
		epicTitles[epic.Title] = true

		if epic.Milestone != "" && !milestoneSet[epic.Milestone] {
			errs = append(errs, fmt.Sprintf("epics[%d] %q: milestone %q is not defined in milestones section", i, epic.Title, epic.Milestone))
		}

		childTitles := make(map[string]bool)
		for j, child := range epic.Children {
			if child.Title == "" {
				errs = append(errs, fmt.Sprintf("epics[%d].children[%d]: title is required", i, j))
				continue
			}
			if childTitles[child.Title] {
				errs = append(errs, fmt.Sprintf("epics[%d].children[%d]: duplicate title %q", i, j, child.Title))
			}
			childTitles[child.Title] = true
		}
	}

	return errs
}

func splitRepo(repo string) []string {
	for i, c := range repo {
		if c == '/' {
			return []string{repo[:i], repo[i+1:]}
		}
	}
	return []string{repo}
}
