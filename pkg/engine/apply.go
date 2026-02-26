package engine

import (
	"context"
	"fmt"
	"strings"

	ghclient "github.com/goblinsan/gh-project-helper/pkg/github"
	"github.com/goblinsan/gh-project-helper/pkg/types"
	gogithub "github.com/google/go-github/v66/github"
	"github.com/shurcooL/githubv4"
)

// GitHubClient defines the interface for GitHub operations needed by the engine.
type GitHubClient interface {
	GetRepositoryID(ctx context.Context, owner, name string) (string, error)
	GetProjectV2ID(ctx context.Context, owner, title string) (string, error)
	GetProjectV2StatusFieldOptions(ctx context.Context, projectID githubv4.ID) (githubv4.ID, map[string]string, error)
	GetOrCreateMilestone(ctx context.Context, owner, repo, title, description, dueOn string) (*gogithub.Milestone, error)
	GetMilestoneID(ctx context.Context, owner, name string, number int) (string, error)
	FindIssueByTitle(ctx context.Context, owner, repo, title string) (int, string, error)
	GetOrCreateLabel(ctx context.Context, owner, repo, labelName string) (githubv4.ID, error)
	GetUserID(ctx context.Context, login string) (githubv4.ID, error)
	CreateIssue(ctx context.Context, input githubv4.CreateIssueInput) (*ghclient.CreateIssueMutation, error)
	AddIssueToProjectV2(ctx context.Context, projectID, contentID githubv4.ID) (*ghclient.AddProjectV2ItemMutation, error)
	UpdateProjectV2ItemStatus(ctx context.Context, projectID, itemID, fieldID githubv4.ID, optionID string) error
}

// Ensure *github.Client satisfies the interface at compile time.
var _ GitHubClient = (*ghclient.Client)(nil)

// Options configures the behavior of ApplyPlan.
type Options struct {
	DryRun bool
}

// Report summarizes the results of an ApplyPlan execution.
type Report struct {
	MilestonesCreated int      `json:"milestones_created"`
	EpicsCreated      int      `json:"epics_created"`
	EpicsSkipped      int      `json:"epics_skipped"`
	IssuesCreated     int      `json:"issues_created"`
	IssuesSkipped     int      `json:"issues_skipped"`
	EpicURLs          []string `json:"epic_urls,omitempty"`
}

func (r *Report) String() string {
	return fmt.Sprintf("Summary: %d milestones synced, %d epics created (%d skipped), %d issues created (%d skipped)",
		r.MilestonesCreated, r.EpicsCreated, r.EpicsSkipped, r.IssuesCreated, r.IssuesSkipped)
}

// ApplyPlan executes a plan against the GitHub API, creating milestones, epics, and child issues.
func ApplyPlan(ctx context.Context, client GitHubClient, plan types.Plan, opts Options) (*Report, error) {
	report := &Report{}
	// Get owner and repo from repository string
	repoParts := strings.Split(plan.Repository, "/")
	if len(repoParts) != 2 {
		return nil, fmt.Errorf("invalid repository format: %s", plan.Repository)
	}
	owner, repo := repoParts[0], repoParts[1]

	if opts.DryRun {
		fmt.Println("[dry-run] Validating plan...")
		fmt.Printf("[dry-run] Repository: %s/%s\n", owner, repo)
		fmt.Printf("[dry-run] Project: %s\n", plan.Project)
	}

	// Resolve Context
	repoID, err := client.GetRepositoryID(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository id: %w", err)
	}

	projectID, err := client.GetProjectV2ID(ctx, owner, plan.Project)
	if err != nil {
		return nil, fmt.Errorf("failed to get project id: %w", err)
	}

	// Get project status field options
	statusFieldID, statusOptions, err := client.GetProjectV2StatusFieldOptions(ctx, githubv4.ID(projectID))
	if err != nil {
		return nil, fmt.Errorf("failed to get project status field options: %w", err)
	}

	// Milestone Sync
	milestones := make(map[string]string)
	for _, m := range plan.Milestones {
		if opts.DryRun {
			fmt.Printf("[dry-run] Would create/sync milestone: %s (due: %s)\n", m.Title, m.DueOn)
			continue
		}
		milestone, err := client.GetOrCreateMilestone(ctx, owner, repo, m.Title, m.Description, m.DueOn)
		if err != nil {
			return nil, fmt.Errorf("failed to get or create milestone: %w", err)
		}
		milestoneID, err := client.GetMilestoneID(ctx, owner, repo, milestone.GetNumber())
		if err != nil {
			return nil, fmt.Errorf("failed to get milestone id: %w", err)
		}
		milestones[m.Title] = milestoneID
		report.MilestonesCreated++
	}

	// Execution Loop (Per Epic)
	for _, epic := range plan.Epics {
		if opts.DryRun {
			fmt.Printf("[dry-run] Would create epic: %s\n", epic.Title)
			if epic.Milestone != "" {
				fmt.Printf("[dry-run]   Milestone: %s\n", epic.Milestone)
			}
			if epic.Status != "" {
				if _, ok := statusOptions[epic.Status]; !ok {
					fmt.Printf("[dry-run]   WARNING: Status %q not found in project\n", epic.Status)
				} else {
					fmt.Printf("[dry-run]   Status: %s\n", epic.Status)
				}
			}
			for _, label := range epic.Labels {
				fmt.Printf("[dry-run]   Label: %s\n", label)
			}
			for _, child := range epic.Children {
				fmt.Printf("[dry-run]   Would create child issue: %s\n", child.Title)
				for _, label := range child.Labels {
					fmt.Printf("[dry-run]     Label: %s\n", label)
				}
			}
			continue
		}
		// Step A (Children)
		var childIssues []string
		for _, child := range epic.Children {
			// Idempotency: check if child issue already exists
			existingNum, existingNodeID, err := client.FindIssueByTitle(ctx, owner, repo, child.Title)
			if err != nil {
				return nil, fmt.Errorf("failed to check for existing issue %q: %w", child.Title, err)
			}
			if existingNum > 0 {
				fmt.Printf("  Skipping child issue (already exists): #%d %s\n", existingNum, child.Title)
				childIssues = append(childIssues, fmt.Sprintf("- [ ] #%d", existingNum))
				report.IssuesSkipped++

				// Still ensure it's on the project board
				projectItem, err := client.AddIssueToProjectV2(ctx, githubv4.ID(projectID), githubv4.ID(existingNodeID))
				if err != nil {
					return nil, fmt.Errorf("failed to add existing child issue to project: %w", err)
				}
				if epic.Status != "" {
					if statusID, ok := statusOptions[epic.Status]; ok {
						_ = client.UpdateProjectV2ItemStatus(ctx, githubv4.ID(projectID), projectItem.AddProjectV2ItemById.Item.ID, statusFieldID, statusID)
					}
				}
				continue
			}

			// Resolve label IDs
			var labelIDs []githubv4.ID
			for _, labelName := range child.Labels {
				labelID, err := client.GetOrCreateLabel(ctx, owner, repo, labelName)
				if err != nil {
					return nil, fmt.Errorf("failed to get or create label %s: %w", labelName, err)
				}
				labelIDs = append(labelIDs, labelID)
			}

			childBody := githubv4.String(child.Body)
			issue, err := client.CreateIssue(ctx, githubv4.CreateIssueInput{
				RepositoryID: githubv4.ID(repoID),
				Title:        githubv4.String(child.Title),
				Body:         &childBody,
				LabelIDs:     &labelIDs,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to create child issue: %w", err)
			}
			childIssues = append(childIssues, fmt.Sprintf("- [ ] #%d", issue.CreateIssue.Issue.Number))
			report.IssuesCreated++

			// Add child issue to project
			projectItem, err := client.AddIssueToProjectV2(ctx, githubv4.ID(projectID), issue.CreateIssue.Issue.ID)
			if err != nil {
				return nil, fmt.Errorf("failed to add child issue to project: %w", err)
			}

			// Update status
			if epic.Status != "" {
				if statusID, ok := statusOptions[epic.Status]; ok {
					err := client.UpdateProjectV2ItemStatus(ctx, githubv4.ID(projectID), projectItem.AddProjectV2ItemById.Item.ID, statusFieldID, statusID)
					if err != nil {
						return nil, fmt.Errorf("failed to update status for child issue: %w", err)
					}
				}
			}
		}

		// Idempotency: check if epic issue already exists
		existingEpicNum, existingEpicNodeID, err := client.FindIssueByTitle(ctx, owner, repo, epic.Title)
		if err != nil {
			return nil, fmt.Errorf("failed to check for existing epic %q: %w", epic.Title, err)
		}
		if existingEpicNum > 0 {
			fmt.Printf("Skipping epic (already exists): #%d %s\n", existingEpicNum, epic.Title)
			report.EpicsSkipped++
			// Still ensure it's on the project board
			projectItem, err := client.AddIssueToProjectV2(ctx, githubv4.ID(projectID), githubv4.ID(existingEpicNodeID))
			if err != nil {
				return nil, fmt.Errorf("failed to add existing epic to project: %w", err)
			}
			if epic.Status != "" {
				if statusID, ok := statusOptions[epic.Status]; ok {
					_ = client.UpdateProjectV2ItemStatus(ctx, githubv4.ID(projectID), projectItem.AddProjectV2ItemById.Item.ID, statusFieldID, statusID)
				}
			}
			continue
		}

		// Step B (Epic Body)
		epicBody := epic.Body + "\n\n" + strings.Join(childIssues, "\n")

		// Step C (Create Epic)
		var milestoneID *githubv4.ID
		if epic.Milestone != "" {
			if mID, ok := milestones[epic.Milestone]; ok {
				id := githubv4.ID(mID)
				milestoneID = &id
			}
		}

		// Resolve label IDs
		var labelIDs []githubv4.ID
		for _, labelName := range epic.Labels {
			labelID, err := client.GetOrCreateLabel(ctx, owner, repo, labelName)
			if err != nil {
				return nil, fmt.Errorf("failed to get or create label %s: %w", labelName, err)
			}
			labelIDs = append(labelIDs, labelID)
		}

		// Resolve assignee IDs
		var assigneeIDs []githubv4.ID
		for _, assigneeLogin := range epic.Assignees {
			assigneeID, err := client.GetUserID(ctx, assigneeLogin)
			if err != nil {
				return nil, fmt.Errorf("failed to get user id for %s: %w", assigneeLogin, err)
			}
			assigneeIDs = append(assigneeIDs, assigneeID)
		}

		epicBodyStr := githubv4.String(epicBody)
		epicIssue, err := client.CreateIssue(ctx, githubv4.CreateIssueInput{
			RepositoryID: githubv4.ID(repoID),
			Title:        githubv4.String(epic.Title),
			Body:         &epicBodyStr,
			MilestoneID:  milestoneID,
			LabelIDs:     &labelIDs,
			AssigneeIDs:  &assigneeIDs,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create epic issue: %w", err)
		}

		// Step D (Project Linkage)
		projectItem, err := client.AddIssueToProjectV2(ctx, githubv4.ID(projectID), epicIssue.CreateIssue.Issue.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to add epic issue to project: %w", err)
		}

		// Update status
		if epic.Status != "" {
			if statusID, ok := statusOptions[epic.Status]; ok {
				err := client.UpdateProjectV2ItemStatus(ctx, githubv4.ID(projectID), projectItem.AddProjectV2ItemById.Item.ID, statusFieldID, statusID)
				if err != nil {
					return nil, fmt.Errorf("failed to update status for epic issue: %w", err)
				}
			}
		}

		report.EpicsCreated++
		report.EpicURLs = append(report.EpicURLs, epicIssue.CreateIssue.Issue.URL.String())
		fmt.Printf("Created epic: %s (%s)\n", epic.Title, epicIssue.CreateIssue.Issue.URL.String())
	}

	return report, nil
}
