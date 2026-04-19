package commands

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/goblinsan/gh-project-helper/pkg/engine"
	"github.com/goblinsan/gh-project-helper/pkg/github"
	"github.com/goblinsan/gh-project-helper/pkg/types"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// cliPrompter implements engine.Prompter using stdin/stdout.
type cliPrompter struct {
	reader *bufio.Reader
}

func newCLIPrompter() *cliPrompter {
	return &cliPrompter{reader: bufio.NewReader(os.Stdin)}
}

func (p *cliPrompter) Confirm(msg string) (bool, error) {
	fmt.Printf("%s [y/N]: ", msg)
	line, err := p.reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	line = strings.TrimSpace(strings.ToLower(line))
	return line == "y" || line == "yes", nil
}

func (p *cliPrompter) Select(msg string, choices []string) (int, error) {
	fmt.Println(msg)
	for i, c := range choices {
		fmt.Printf("  [%d] %s\n", i+1, c)
	}
	fmt.Print("Enter number (0 to cancel): ")
	line, err := p.reader.ReadString('\n')
	if err != nil {
		return -1, err
	}
	line = strings.TrimSpace(line)
	n, err := strconv.Atoi(line)
	if err != nil || n < 0 || n > len(choices) {
		return -1, fmt.Errorf("invalid selection: %s", line)
	}
	if n == 0 {
		return -1, nil
	}
	return n - 1, nil
}

func init() {
	rootCmd.AddCommand(applyCmd)
	applyCmd.Flags().StringP("file", "f", "", "The plan file to apply")
	_ = applyCmd.MarkFlagRequired("file")
	applyCmd.Flags().Bool("dry-run", false, "Preview what would be created without making changes")
	applyCmd.Flags().Bool("create-repo-if-missing", false, "Create the requested repository automatically when it does not exist and no similar matches are found")
	applyCmd.Flags().String("repository-override", "", "Override the plan repository with an explicit owner/repo target")
	applyCmd.Flags().Bool("json", false, "Emit the apply report as JSON instead of a human summary")
}

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply a project plan from a YAML file",
	Long:  `Apply a project plan from a YAML file to create GitHub projects, epics, and issues.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath, _ := cmd.Flags().GetString("file")

		yamlFile, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}

		var plan types.Plan
		err = yaml.Unmarshal(yamlFile, &plan)
		if err != nil {
			return fmt.Errorf("failed to unmarshal YAML: %w", err)
		}

		repositoryOverride, _ := cmd.Flags().GetString("repository-override")
		if repositoryOverride != "" {
			plan.Repository = repositoryOverride
		}

		client, err := github.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create github client: %w", err)
		}

		dryRun, _ := cmd.Flags().GetBool("dry-run")
		createRepoIfMissing, _ := cmd.Flags().GetBool("create-repo-if-missing")
		jsonOutput, _ := cmd.Flags().GetBool("json")
		report, err := engine.ApplyPlan(context.Background(), client, plan, engine.Options{
			DryRun:              dryRun,
			Prompter:            newCLIPrompter(),
			CreateRepoIfMissing: createRepoIfMissing,
		})
		if err != nil {
			return err
		}
		if report != nil {
			if jsonOutput {
				return json.NewEncoder(os.Stdout).Encode(report)
			}
			fmt.Println(report)
		}
		return nil
	},
}
