package engine

import (
	"context"
	"fmt"
	"strings"

	ghclient "github.com/goblinsan/gh-project-helper/pkg/github"
	"github.com/goblinsan/gh-project-helper/pkg/types"
)

type PreflightStatus string

const (
	PreflightStatusReady                          PreflightStatus = "ready"
	PreflightStatusInvalid                        PreflightStatus = "invalid"
	PreflightStatusRepoResolutionRequired         PreflightStatus = "repo_resolution_required"
	PreflightStatusCreateRepoConfirmationRequired PreflightStatus = "create_repo_confirmation_required"
)

type PreflightReport struct {
	Status     PreflightStatus     `json:"status"`
	Errors     []string            `json:"errors,omitempty"`
	Project    ProjectPreflight    `json:"project"`
	Repository RepositoryPreflight `json:"repository"`
}

type ProjectPreflight struct {
	Name string `json:"name"`
}

type RepositoryPreflight struct {
	Requested        string                      `json:"requested"`
	Owner            string                      `json:"owner,omitempty"`
	Name             string                      `json:"name,omitempty"`
	ExactMatch       bool                        `json:"exactMatch"`
	ResolvedFullName string                      `json:"resolvedFullName,omitempty"`
	Similar          []ghclient.RepoSearchResult `json:"similar,omitempty"`
}

func PreflightPlan(ctx context.Context, client GitHubClient, plan types.Plan) (*PreflightReport, error) {
	parts := strings.Split(plan.Repository, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("invalid repository format: %s", plan.Repository)
	}

	owner, repo := parts[0], parts[1]
	report := &PreflightReport{
		Status:  PreflightStatusReady,
		Project: ProjectPreflight{Name: plan.Project},
		Repository: RepositoryPreflight{
			Requested: plan.Repository,
			Owner:     owner,
			Name:      repo,
		},
	}

	exists, err := client.RepoExists(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to check if repo exists: %w", err)
	}

	if exists {
		report.Repository.ExactMatch = true
		report.Repository.ResolvedFullName = plan.Repository
		return report, nil
	}

	similar, err := client.SearchRepos(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to search for similar repos: %w", err)
	}

	report.Repository.Similar = similar
	if len(similar) > 0 {
		report.Status = PreflightStatusRepoResolutionRequired
		return report, nil
	}

	report.Status = PreflightStatusCreateRepoConfirmationRequired
	return report, nil
}
