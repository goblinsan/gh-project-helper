package github

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/google/go-github/v66/github"
	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"
)

// Client wraps both the REST API client (go-github) and GraphQL client (githubv4)
type Client struct {
	REST    *github.Client
	GraphQL *githubv4.Client
}

// NewClient creates a new GitHub client with both REST and GraphQL capabilities
func NewClient() (*Client, error) {
	token, err := GetToken()
	if err != nil {
		return nil, err
	}
	var httpClient *http.Client

	if token != "" {
		// Create an OAuth2 token source
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: token},
		)
		httpClient = oauth2.NewClient(context.Background(), ts)
	} else {
		httpClient = http.DefaultClient
	}

	return &Client{
		REST:    github.NewClient(httpClient),
		GraphQL: githubv4.NewClient(httpClient),
	}, nil
}

// GetToken retrieves the GitHub token from the environment or `gh` CLI
func GetToken() (string, error) {
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return token, nil
	}

	cmd := exec.Command("gh", "auth", "token")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

// GetAuthenticatedUser returns information about the authenticated user
func (c *Client) GetAuthenticatedUser(ctx context.Context) (*github.User, error) {
	user, _, err := c.REST.Users.Get(ctx, "")
	return user, err
}

type RepositoryIDQuery struct {
	Repository struct {
		ID string
	} `graphql:"repository(owner: $owner, name: $name)"`
}

func (c *Client) GetRepositoryID(ctx context.Context, owner, name string) (string, error) {
	var query RepositoryIDQuery
	variables := map[string]interface{}{
		"owner": githubv4.String(owner),
		"name":  githubv4.String(name),
	}
	err := c.GraphQL.Query(ctx, &query, variables)
	if err != nil {
		return "", err
	}
	return query.Repository.ID, nil
}

type ProjectV2IDUserQuery struct {
	User struct {
		ProjectsV2 struct {
			Nodes []struct {
				ID    string
				Title string
			}
		} `graphql:"projectsV2(first: 100)"`
	} `graphql:"user(login: $owner)"`
}

type ProjectV2IDOrgQuery struct {
	Organization struct {
		ProjectsV2 struct {
			Nodes []struct {
				ID    string
				Title string
			}
		} `graphql:"projectsV2(first: 100)"`
	} `graphql:"organization(login: $owner)"`
}

func (c *Client) GetProjectV2ID(ctx context.Context, owner, title string) (string, error) {
	// Try user first
	var userQuery ProjectV2IDUserQuery
	variables := map[string]interface{}{
		"owner": githubv4.String(owner),
	}
	err := c.GraphQL.Query(ctx, &userQuery, variables)
	if err == nil {
		for _, p := range userQuery.User.ProjectsV2.Nodes {
			if p.Title == title {
				return p.ID, nil
			}
		}
	}

	// Fall back to organization
	var orgQuery ProjectV2IDOrgQuery
	err = c.GraphQL.Query(ctx, &orgQuery, variables)
	if err == nil {
		for _, p := range orgQuery.Organization.ProjectsV2.Nodes {
			if p.Title == title {
				return p.ID, nil
			}
		}
	}

	return "", fmt.Errorf("project %q not found for user or organization %q", title, owner)
}

func (c *Client) GetOrCreateMilestone(ctx context.Context, owner, repo, title, description, dueOn string) (*github.Milestone, error) {
	milestones, _, err := c.REST.Issues.ListMilestones(ctx, owner, repo, &github.MilestoneListOptions{})
	if err != nil {
		return nil, err
	}

	for _, m := range milestones {
		if m.GetTitle() == title {
			return m, nil
		}
	}

	var dueOnTimestamp *github.Timestamp
	if dueOn != "" {
		t, err := time.Parse("2006-01-02", dueOn)
		if err != nil {
			return nil, err
		}
		dueOnTimestamp = &github.Timestamp{Time: t}
	}

	newMilestone := &github.Milestone{
		Title:       github.String(title),
		Description: github.String(description),
		DueOn:       dueOnTimestamp,
	}

	createdMilestone, _, err := c.REST.Issues.CreateMilestone(ctx, owner, repo, newMilestone)
	if err != nil {
		return nil, err
	}

	return createdMilestone, nil
}

// FindIssueByTitle searches for an open issue with the exact title in the given repo.
// Returns the issue number and node ID if found, or 0/"" if not found.
func (c *Client) FindIssueByTitle(ctx context.Context, owner, repo, title string) (int, string, error) {
	issues, _, err := c.REST.Search.Issues(ctx, fmt.Sprintf("repo:%s/%s is:issue is:open in:title %q", owner, repo, title), &github.SearchOptions{
		ListOptions: github.ListOptions{PerPage: 10},
	})
	if err != nil {
		return 0, "", err
	}
	for _, issue := range issues.Issues {
		if issue.GetTitle() == title {
			return issue.GetNumber(), issue.GetNodeID(), nil
		}
	}
	return 0, "", nil
}

type CreateIssueMutation struct {
	CreateIssue struct {
		Issue struct {
			ID     githubv4.ID
			Number int
			URL    githubv4.URI
		}
	} `graphql:"createIssue(input: $input)"`
}

func (c *Client) CreateIssue(ctx context.Context, input githubv4.CreateIssueInput) (*CreateIssueMutation, error) {
	var mutation CreateIssueMutation
	err := c.GraphQL.Mutate(ctx, &mutation, input, nil)
	if err != nil {
		return nil, err
	}
	return &mutation, nil
}

type AddProjectV2ItemMutation struct {
	AddProjectV2ItemById struct {
		Item struct {
			ID githubv4.ID
		}
	} `graphql:"addProjectV2ItemById(input: $input)"`
}

func (c *Client) AddIssueToProjectV2(ctx context.Context, projectID, contentID githubv4.ID) (*AddProjectV2ItemMutation, error) {
	var mutation AddProjectV2ItemMutation
	input := githubv4.AddProjectV2ItemByIdInput{
		ProjectID: projectID,
		ContentID: contentID,
	}
	err := c.GraphQL.Mutate(ctx, &mutation, input, nil)
	if err != nil {
		return nil, err
	}
	return &mutation, nil
}

type MilestoneIDQuery struct {
	Repository struct {
		Milestone struct {
			ID string
		} `graphql:"milestone(number: $number)"`
	} `graphql:"repository(owner: $owner, name: $name)"`
}

func (c *Client) GetMilestoneID(ctx context.Context, owner, name string, number int) (string, error) {
	var query MilestoneIDQuery
	variables := map[string]interface{}{
		"owner":  githubv4.String(owner),
		"name":   githubv4.String(name),
		"number": githubv4.Int(number),
	}
	err := c.GraphQL.Query(ctx, &query, variables)
	if err != nil {
		return "", err
	}
	return query.Repository.Milestone.ID, nil
}


func (c *Client) GetOrCreateLabel(ctx context.Context, owner, repo, labelName string) (githubv4.ID, error) {
	label, resp, err := c.REST.Issues.GetLabel(ctx, owner, repo, labelName)
	if err != nil {
		if resp != nil && resp.StatusCode == 404 {
			// Label doesn't exist, create it
			newLabel, _, createErr := c.REST.Issues.CreateLabel(ctx, owner, repo, &github.Label{
				Name: github.String(labelName),
			})
			if createErr != nil {
				return nil, fmt.Errorf("failed to create label %s: %w", labelName, createErr)
			}
			return newLabel.GetNodeID(), nil
		}
		return nil, err
	}
	return label.GetNodeID(), nil
}

type UserIDQuery struct {
	User struct {
		ID githubv4.ID
	} `graphql:"user(login: $login)"`
}

func (c *Client) GetUserID(ctx context.Context, login string) (githubv4.ID, error) {
	var query UserIDQuery
	variables := map[string]interface{}{
		"login": githubv4.String(login),
	}
	err := c.GraphQL.Query(ctx, &query, variables)
	if err != nil {
		return nil, err
	}
	return query.User.ID, nil
}

type ProjectV2FieldQuery struct {
	Node struct {
		ProjectV2 struct {
			Fields struct {
				Nodes []struct {
					ProjectV2SingleSelectField struct {
						ID      string `graphql:"id"`
						Name    string `graphql:"name"`
						Options []struct {
							ID   string `graphql:"id"`
							Name string `graphql:"name"`
						} `graphql:"options"`
					} `graphql:"... on ProjectV2SingleSelectField"`
				}
			} `graphql:"fields(first: 20)"`
		} `graphql:"... on ProjectV2"`
	} `graphql:"node(id: $projectID)"`
}

func (c *Client) GetProjectV2StatusFieldOptions(ctx context.Context, projectID githubv4.ID) (githubv4.ID, map[string]string, error) {
	var query ProjectV2FieldQuery
	variables := map[string]interface{}{
		"projectID": projectID,
	}
	err := c.GraphQL.Query(ctx, &query, variables)
	if err != nil {
		return nil, nil, err
	}

	for _, field := range query.Node.ProjectV2.Fields.Nodes {
		f := field.ProjectV2SingleSelectField
		if f.Name == "Status" {
			statusOptions := make(map[string]string)
			for _, option := range f.Options {
				statusOptions[option.Name] = option.ID
			}
			return githubv4.ID(f.ID), statusOptions, nil
		}
	}

	return nil, nil, fmt.Errorf("status field not found on project")
}

type UpdateProjectV2ItemFieldValueMutation struct {
	UpdateProjectV2ItemFieldValue struct {
		ClientMutationId githubv4.String
	} `graphql:"updateProjectV2ItemFieldValue(input: $input)"`
}

func (c *Client) UpdateProjectV2ItemStatus(ctx context.Context, projectID, itemID, fieldID githubv4.ID, optionID string) error {
	var mutation UpdateProjectV2ItemFieldValueMutation
	optionStr := githubv4.String(optionID)
	input := githubv4.UpdateProjectV2ItemFieldValueInput{
		ProjectID: projectID,
		ItemID:    itemID,
		FieldID:   fieldID,
		Value: githubv4.ProjectV2FieldValue{
			SingleSelectOptionID: &optionStr,
		},
	}
	err := c.GraphQL.Mutate(ctx, &mutation, input, nil)
	return err
}
