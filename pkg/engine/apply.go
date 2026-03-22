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
	ListAllIssues(ctx context.Context, owner, repo string) (map[string]ghclient.IssueRef, error)
	GetOrCreateLabel(ctx context.Context, owner, repo, labelName string) (githubv4.ID, error)
	GetUserID(ctx context.Context, login string) (githubv4.ID, error)
	CreateIssue(ctx context.Context, input githubv4.CreateIssueInput) (*ghclient.CreateIssueMutation, error)
	AddIssueToProjectV2(ctx context.Context, projectID, contentID githubv4.ID) (*ghclient.AddProjectV2ItemMutation, error)
	AddSubIssue(ctx context.Context, parentIssueID, childIssueID githubv4.ID) error
	UpdateProjectV2ItemStatus(ctx context.Context, projectID, itemID, fieldID githubv4.ID, optionID string) error

	// Resource verification & creation
	RepoExists(ctx context.Context, owner, name string) (bool, error)
	SearchRepos(ctx context.Context, owner, query string) ([]ghclient.RepoSearchResult, error)
	CreateRepo(ctx context.Context, owner, name string) error
	HasReadme(ctx context.Context, owner, name string) (bool, error)
	CreateReadme(ctx context.Context, owner, name string) error
	GetOwnerID(ctx context.Context, login string) (string, error)
	CreateProjectV2(ctx context.Context, ownerID, title string) (string, error)
	LinkProjectV2ToRepo(ctx context.Context, projectID, repoID string) error
}

// Prompter abstracts interactive user prompts so the engine stays testable.
type Prompter interface {
	// Confirm asks a yes/no question and returns true for yes.
	Confirm(msg string) (bool, error)
	// Select presents choices and returns the selected index (-1 for none/cancel).
	Select(msg string, choices []string) (int, error)
}

// Ensure *github.Client satisfies the interface at compile time.
var _ GitHubClient = (*ghclient.Client)(nil)

// Options configures the behavior of ApplyPlan.
type Options struct {
	DryRun   bool
	Prompter Prompter
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

	// ── Pre-flight: ensure repository exists ──
	repoID, err := ensureRepo(ctx, client, owner, repo, opts)
	if err != nil {
		return nil, err
	}

	// ── Pre-flight: ensure repo has a README ──
	if err := ensureReadme(ctx, client, owner, repo, opts); err != nil {
		return nil, err
	}

	// ── Pre-flight: ensure project exists ──
	projectID, err := ensureProject(ctx, client, owner, plan.Project, opts)
	if err != nil {
		return nil, err
	}

	// Get project status field options
	var statusFieldID githubv4.ID
	var statusOptions map[string]string
	if opts.DryRun {
		statusOptions = make(map[string]string)
		fmt.Println("[dry-run] Skipping project status field lookup")
	} else {
		statusFieldID, statusOptions, err = client.GetProjectV2StatusFieldOptions(ctx, githubv4.ID(projectID))
		if err != nil {
			return nil, fmt.Errorf("failed to get project status field options: %w", err)
		}
	}

	// Pre-fetch all existing issues once to avoid Search API rate limits
	issueCache := make(map[string]ghclient.IssueRef)
	if !opts.DryRun {
		issueCache, err = client.ListAllIssues(ctx, owner, repo)
		if err != nil {
			return nil, fmt.Errorf("failed to list existing issues: %w", err)
		}
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
			fmt.Printf("[dry-run]   Label: Epic (auto)\n")
			for _, label := range epic.Labels {
				fmt.Printf("[dry-run]   Label: %s\n", label)
			}
			for _, child := range epic.Children {
				fmt.Printf("[dry-run]   Would create child issue: %s (sub-issue of epic)\n", child.Title)
				for _, label := range child.Labels {
					fmt.Printf("[dry-run]     Label: %s\n", label)
				}
			}
			continue
		}
		// Step A (Children)
		var childIssues []string
		var childNodeIDs []githubv4.ID
		for _, child := range epic.Children {
			// Idempotency: check if child issue already exists (from pre-fetched cache)
			var existingNum int
			var existingNodeID string
			if ref, ok := issueCache[child.Title]; ok {
				existingNum, existingNodeID = ref.Number, ref.NodeID
			}
			if existingNum > 0 {
				fmt.Printf("  Skipping child issue (already exists): #%d %s\n", existingNum, child.Title)
				childIssues = append(childIssues, fmt.Sprintf("- [ ] #%d", existingNum))
				childNodeIDs = append(childNodeIDs, githubv4.ID(existingNodeID))
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
			childNodeIDs = append(childNodeIDs, issue.CreateIssue.Issue.ID)
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

		// Idempotency: check if epic issue already exists (from pre-fetched cache)
		var existingEpicNum int
		var existingEpicNodeID string
		if ref, ok := issueCache[epic.Title]; ok {
			existingEpicNum, existingEpicNodeID = ref.Number, ref.NodeID
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
		epicBody := epic.Body + "\n\n## Sub-issues\n\n" + strings.Join(childIssues, "\n")

		// Step C (Create Epic)
		var milestoneID *githubv4.ID
		if epic.Milestone != "" {
			if mID, ok := milestones[epic.Milestone]; ok {
				id := githubv4.ID(mID)
				milestoneID = &id
			}
		}

		// Resolve label IDs — always include "Epic" label for visual identification
		epicLabelID, err := client.GetOrCreateLabel(ctx, owner, repo, "Epic")
		if err != nil {
			return nil, fmt.Errorf("failed to get or create Epic label: %w", err)
		}
		labelIDs := []githubv4.ID{epicLabelID}
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

		// Step E (Sub-issue Relationships)
		for _, childNodeID := range childNodeIDs {
			err := client.AddSubIssue(ctx, epicIssue.CreateIssue.Issue.ID, childNodeID)
			if err != nil {
				fmt.Printf("  Warning: failed to add sub-issue relationship: %v\n", err)
			}
		}

		report.EpicsCreated++
		report.EpicURLs = append(report.EpicURLs, epicIssue.CreateIssue.Issue.URL.String())
		fmt.Printf("Created epic: %s (%s)\n", epic.Title, epicIssue.CreateIssue.Issue.URL.String())
	}

	// ── Post-flight: link project to repository ──
	if !opts.DryRun {
		if err := client.LinkProjectV2ToRepo(ctx, projectID, repoID); err != nil {
			fmt.Printf("Warning: failed to link project to repo (may already be linked): %v\n", err)
		} else {
			fmt.Printf("Linked project %q to repository %s/%s\n", plan.Project, owner, repo)
		}
	} else {
		fmt.Printf("[dry-run] Would link project %q to repository %s/%s\n", plan.Project, owner, repo)
	}

	return report, nil
}

// ensureRepo verifies the repository exists. If not, it searches for similar names,
// prompts the user, and optionally creates a new repo.
func ensureRepo(ctx context.Context, client GitHubClient, owner, repo string, opts Options) (string, error) {
	exists, err := client.RepoExists(ctx, owner, repo)
	if err != nil {
		return "", fmt.Errorf("failed to check if repo exists: %w", err)
	}

	if exists {
		repoID, err := client.GetRepositoryID(ctx, owner, repo)
		if err != nil {
			return "", fmt.Errorf("failed to get repository id: %w", err)
		}
		return repoID, nil
	}

	fmt.Printf("Repository %s/%s does not exist.\n", owner, repo)

	if opts.DryRun {
		fmt.Printf("[dry-run] Would search for similar repos and potentially create %s/%s\n", owner, repo)
		return "dry-run-repo-id", nil
	}

	if opts.Prompter == nil {
		return "", fmt.Errorf("repository %s/%s not found and no interactive prompter available", owner, repo)
	}

	// Search for similar repos
	similar, err := client.SearchRepos(ctx, owner, repo)
	if err != nil {
		fmt.Printf("Warning: could not search for similar repos: %v\n", err)
	}

	if len(similar) > 0 {
		fmt.Println("Found similar repositories:")
		choices := make([]string, 0, len(similar)+1)
		for _, r := range similar {
			desc := ""
			if r.Description != "" {
				desc = " — " + r.Description
			}
			choices = append(choices, r.FullName+desc)
		}
		choices = append(choices, fmt.Sprintf("Create new repo: %s/%s", owner, repo))

		idx, err := opts.Prompter.Select("Choose an existing repo or create a new one:", choices)
		if err != nil {
			return "", fmt.Errorf("prompt failed: %w", err)
		}
		if idx < 0 {
			return "", fmt.Errorf("cancelled by user")
		}

		if idx < len(similar) {
			// User chose an existing similar repo — update owner/repo from full name
			parts := strings.SplitN(similar[idx].FullName, "/", 2)
			if len(parts) == 2 {
				fmt.Printf("Using existing repository: %s\n", similar[idx].FullName)
				repoID, err := client.GetRepositoryID(ctx, parts[0], parts[1])
				if err != nil {
					return "", fmt.Errorf("failed to get repository id: %w", err)
				}
				return repoID, nil
			}
		}
	} else {
		ok, err := opts.Prompter.Confirm(fmt.Sprintf("No similar repos found. Create %s/%s?", owner, repo))
		if err != nil {
			return "", fmt.Errorf("prompt failed: %w", err)
		}
		if !ok {
			return "", fmt.Errorf("cancelled by user")
		}
	}

	// Create the repo
	fmt.Printf("Creating repository %s/%s...\n", owner, repo)
	if err := client.CreateRepo(ctx, owner, repo); err != nil {
		return "", err
	}
	fmt.Printf("Created repository %s/%s\n", owner, repo)

	repoID, err := client.GetRepositoryID(ctx, owner, repo)
	if err != nil {
		return "", fmt.Errorf("failed to get repository id after creation: %w", err)
	}
	return repoID, nil
}

// ensureReadme makes sure the repository has a README file.
func ensureReadme(ctx context.Context, client GitHubClient, owner, repo string, opts Options) error {
	if opts.DryRun {
		fmt.Printf("[dry-run] Would check and ensure README exists in %s/%s\n", owner, repo)
		return nil
	}

	has, err := client.HasReadme(ctx, owner, repo)
	if err != nil {
		return fmt.Errorf("failed to check for README: %w", err)
	}
	if has {
		return nil
	}

	fmt.Printf("Repository %s/%s has no README — creating one...\n", owner, repo)
	if err := client.CreateReadme(ctx, owner, repo); err != nil {
		return fmt.Errorf("failed to create README: %w", err)
	}
	fmt.Printf("Created README.md in %s/%s\n", owner, repo)
	return nil
}

// ensureProject verifies the project exists. If not, creates it.
func ensureProject(ctx context.Context, client GitHubClient, owner, title string, opts Options) (string, error) {
	projectID, err := client.GetProjectV2ID(ctx, owner, title)
	if err == nil {
		return projectID, nil
	}

	fmt.Printf("Project %q not found for %s — creating it...\n", title, owner)

	if opts.DryRun {
		fmt.Printf("[dry-run] Would create project %q under %s\n", title, owner)
		return "dry-run-project-id", nil
	}

	ownerID, err := client.GetOwnerID(ctx, owner)
	if err != nil {
		return "", fmt.Errorf("failed to resolve owner ID for %s: %w", owner, err)
	}

	projectID, err = client.CreateProjectV2(ctx, ownerID, title)
	if err != nil {
		return "", err
	}
	fmt.Printf("Created project %q (ID: %s)\n", title, projectID)
	return projectID, nil
}
