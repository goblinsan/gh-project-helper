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
	"github.com/spf13/viper"
	"golang.org/x/oauth2"
)

// RepoSearchResult holds a similar repository match.
type RepoSearchResult struct {
	FullName    string
	Description string
}

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

// GetToken retrieves the GitHub token from env/config or falls back to `gh auth token`.
func GetToken() (string, error) {
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return token, nil
	}
	if token := os.Getenv("GH_PROJECT_HELPER_TOKEN"); token != "" {
		return token, nil
	}
	if token := strings.TrimSpace(viper.GetString("token")); token != "" {
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

func (c *Client) SyncMilestone(ctx context.Context, owner, repo, title, description, dueOn string) (*MilestoneSyncResult, error) {
	milestones, _, err := c.REST.Issues.ListMilestones(ctx, owner, repo, &github.MilestoneListOptions{})
	if err != nil {
		return nil, err
	}

	var dueOnTimestamp *github.Timestamp
	if dueOn != "" {
		t, err := time.Parse("2006-01-02", dueOn)
		if err != nil {
			return nil, err
		}
		dueOnTimestamp = &github.Timestamp{Time: t}
	}

	for _, m := range milestones {
		if m.GetTitle() == title {
			needsDescriptionUpdate := m.GetDescription() != description
			needsDueOnUpdate := !timestampsEqual(m.DueOn, dueOnTimestamp)
			if !needsDescriptionUpdate && !needsDueOnUpdate {
				return &MilestoneSyncResult{Milestone: m}, nil
			}

			req := &github.Milestone{
				Title:       github.String(title),
				Description: github.String(description),
				DueOn:       dueOnTimestamp,
			}
			updated, _, err := c.REST.Issues.EditMilestone(ctx, owner, repo, m.GetNumber(), req)
			if err != nil {
				return nil, err
			}
			return &MilestoneSyncResult{Milestone: updated, Updated: true}, nil
		}
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

	return &MilestoneSyncResult{Milestone: createdMilestone, Created: true}, nil
}

func timestampsEqual(a, b *github.Timestamp) bool {
	switch {
	case a == nil && b == nil:
		return true
	case a == nil || b == nil:
		return false
	default:
		return a.Time.Equal(b.Time)
	}
}

// ListAllIssues fetches all issues from the repo and returns a map keyed by title.
// Using the Issues list endpoint avoids the Search API's strict rate limit of 30 req/min.
func (c *Client) ListAllIssues(ctx context.Context, owner, repo string) (map[string]IssueRef, error) {
	result := make(map[string]IssueRef)
	opts := &github.IssueListByRepoOptions{
		State:       "all",
		ListOptions: github.ListOptions{PerPage: 100},
	}
	for {
		issues, resp, err := c.REST.Issues.ListByRepo(ctx, owner, repo, opts)
		if err != nil {
			return nil, err
		}
		for _, issue := range issues {
			result[issue.GetTitle()] = IssueRef{
				Number: issue.GetNumber(),
				NodeID: issue.GetNodeID(),
				URL:    issue.GetHTMLURL(),
			}
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return result, nil
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

func (c *Client) UpdateIssue(ctx context.Context, owner, repo string, number int, req IssueSyncRequest) (IssueRef, error) {
	labels := append([]string{}, req.Labels...)
	assignees := append([]string{}, req.Assignees...)

	var milestone *int
	if req.MilestoneNumber != nil {
		value := *req.MilestoneNumber
		milestone = &value
	}

	edit := &github.IssueRequest{
		Title:     github.String(req.Title),
		Body:      github.String(req.Body),
		Labels:    &labels,
		Assignees: &assignees,
		Milestone: milestone,
	}
	issue, _, err := c.REST.Issues.Edit(ctx, owner, repo, number, edit)
	if err != nil {
		return IssueRef{}, err
	}
	return IssueRef{
		Number: issue.GetNumber(),
		NodeID: issue.GetNodeID(),
		URL:    issue.GetHTMLURL(),
	}, nil
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
		if !strings.Contains(strings.ToLower(err.Error()), "already exists") {
			return nil, err
		}
		itemID, lookupErr := c.findProjectItemIDByContent(ctx, projectID, contentID)
		if lookupErr != nil {
			return nil, lookupErr
		}
		mutation.AddProjectV2ItemById.Item.ID = itemID
	}
	return &mutation, nil
}

type projectItemsByContentQuery struct {
	Node struct {
		ProjectV2 struct {
			Items struct {
				Nodes []struct {
					ID      githubv4.ID
					Content struct {
						Issue struct {
							ID githubv4.ID
						} `graphql:"... on Issue"`
					}
				}
				PageInfo struct {
					HasNextPage bool
					EndCursor   githubv4.String
				}
			} `graphql:"items(first: 100, after: $cursor)"`
		} `graphql:"... on ProjectV2"`
	} `graphql:"node(id: $projectID)"`
}

func (c *Client) findProjectItemIDByContent(ctx context.Context, projectID, contentID githubv4.ID) (githubv4.ID, error) {
	var cursor *githubv4.String
	for {
		var query projectItemsByContentQuery
		variables := map[string]interface{}{
			"projectID": projectID,
			"cursor":    cursor,
		}
		if err := c.GraphQL.Query(ctx, &query, variables); err != nil {
			return nil, fmt.Errorf("failed to locate existing project item: %w", err)
		}
		for _, node := range query.Node.ProjectV2.Items.Nodes {
			if node.Content.Issue.ID == contentID {
				return node.ID, nil
			}
		}
		if !query.Node.ProjectV2.Items.PageInfo.HasNextPage {
			break
		}
		next := query.Node.ProjectV2.Items.PageInfo.EndCursor
		cursor = &next
	}
	return nil, fmt.Errorf("project item for content %s not found after duplicate add", contentID)
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
				// If another run created the label first, refetch and treat it as success.
				if strings.Contains(strings.ToLower(createErr.Error()), "already_exists") {
					existing, _, fetchErr := c.REST.Issues.GetLabel(ctx, owner, repo, labelName)
					if fetchErr == nil {
						return existing.GetNodeID(), nil
					}
				}
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

// AddSubIssueInput is the input for the addSubIssue mutation.
// This type is not yet in the shurcooL/githubv4 library but the library
// derives the GraphQL type name from the Go type name via reflection,
// so naming it AddSubIssueInput maps to the correct GraphQL input type.
type AddSubIssueInput struct {
	// The ID of the parent issue. (Required.)
	IssueID githubv4.ID `json:"issueId"`
	// The ID of the sub-issue to add. (Optional — provide this or SubIssueURL.)
	SubIssueID *githubv4.ID `json:"subIssueId,omitempty"`
	// Whether to replace the existing parent if the sub-issue already has one. (Optional.)
	ReplaceParent *githubv4.Boolean `json:"replaceParent,omitempty"`

	ClientMutationID *githubv4.String `json:"clientMutationId,omitempty"`
}

type AddSubIssueMutation struct {
	AddSubIssue struct {
		SubIssue struct {
			ID githubv4.ID
		}
		Issue struct {
			ID githubv4.ID
		}
	} `graphql:"addSubIssue(input: $input)"`
}

func (c *Client) AddSubIssue(ctx context.Context, parentIssueID, childIssueID githubv4.ID) error {
	var mutation AddSubIssueMutation
	input := AddSubIssueInput{
		IssueID:    parentIssueID,
		SubIssueID: &childIssueID,
	}
	return c.GraphQL.Mutate(ctx, &mutation, input, nil)
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

// RepoExists checks whether the repository owner/name exists.
func (c *Client) RepoExists(ctx context.Context, owner, name string) (bool, error) {
	_, resp, err := c.REST.Repositories.Get(ctx, owner, name)
	if err != nil {
		if resp != nil && resp.StatusCode == 404 {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// SearchRepos returns repositories under the given owner whose names are similar to the query.
func (c *Client) SearchRepos(ctx context.Context, owner, query string) ([]RepoSearchResult, error) {
	q := fmt.Sprintf("%s in:name org:%s", query, owner)
	// Also try user: qualifier in case owner is a user, not an org
	results, _, err := c.REST.Search.Repositories(ctx, q, &github.SearchOptions{
		ListOptions: github.ListOptions{PerPage: 10},
	})
	if err != nil {
		// Retry with user qualifier
		q = fmt.Sprintf("%s in:name user:%s", query, owner)
		results, _, err = c.REST.Search.Repositories(ctx, q, &github.SearchOptions{
			ListOptions: github.ListOptions{PerPage: 10},
		})
		if err != nil {
			return nil, err
		}
	}

	var matches []RepoSearchResult
	for _, r := range results.Repositories {
		matches = append(matches, RepoSearchResult{
			FullName:    r.GetFullName(),
			Description: r.GetDescription(),
		})
	}
	return matches, nil
}

// CreateRepo creates a new repository under the given owner.
// If owner matches the authenticated user, it creates a user repo; otherwise an org repo.
func (c *Client) CreateRepo(ctx context.Context, owner, name string) error {
	user, _, err := c.REST.Users.Get(ctx, "")
	if err != nil {
		return fmt.Errorf("failed to get authenticated user: %w", err)
	}

	repo := &github.Repository{
		Name:     github.String(name),
		Private:  github.Bool(false),
		AutoInit: github.Bool(true),
	}

	if user.GetLogin() == owner {
		_, _, err = c.REST.Repositories.Create(ctx, "", repo)
	} else {
		_, _, err = c.REST.Repositories.Create(ctx, owner, repo)
	}
	if err != nil {
		return fmt.Errorf("failed to create repo %s/%s: %w", owner, name, err)
	}
	return nil
}

// HasReadme checks whether the repository has a README file.
func (c *Client) HasReadme(ctx context.Context, owner, name string) (bool, error) {
	_, _, resp, err := c.REST.Repositories.GetContents(ctx, owner, name, "README.md", nil)
	if err != nil {
		if resp != nil && resp.StatusCode == 404 {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// CreateReadme creates a default README.md in the repository.
func (c *Client) CreateReadme(ctx context.Context, owner, name string) error {
	content := fmt.Sprintf("# %s\n", name)
	_, _, err := c.REST.Repositories.CreateFile(ctx, owner, name, "README.md", &github.RepositoryContentFileOptions{
		Message: github.String("docs: add README"),
		Content: []byte(content),
	})
	return err
}

// CreateProjectV2 mutation types

type CreateProjectV2Mutation struct {
	CreateProjectV2 struct {
		ProjectV2 struct {
			ID    string
			Title string
		}
	} `graphql:"createProjectV2(input: $input)"`
}

type CreateProjectV2Input struct {
	OwnerID githubv4.ID     `json:"ownerId"`
	Title   githubv4.String `json:"title"`
}

// GetOwnerID returns the node ID of the given user or organization.
func (c *Client) GetOwnerID(ctx context.Context, login string) (string, error) {
	// Try user first
	var userQ struct {
		User struct {
			ID string
		} `graphql:"user(login: $login)"`
	}
	vars := map[string]interface{}{"login": githubv4.String(login)}
	if err := c.GraphQL.Query(ctx, &userQ, vars); err == nil && userQ.User.ID != "" {
		return userQ.User.ID, nil
	}

	// Fall back to organization
	var orgQ struct {
		Organization struct {
			ID string
		} `graphql:"organization(login: $login)"`
	}
	if err := c.GraphQL.Query(ctx, &orgQ, vars); err == nil && orgQ.Organization.ID != "" {
		return orgQ.Organization.ID, nil
	}

	return "", fmt.Errorf("owner %q not found as user or organization", login)
}

// CreateProjectV2 creates a new GitHub Projects V2 board under the given owner.
func (c *Client) CreateProjectV2(ctx context.Context, ownerID, title string) (string, error) {
	var mutation CreateProjectV2Mutation
	input := CreateProjectV2Input{
		OwnerID: githubv4.ID(ownerID),
		Title:   githubv4.String(title),
	}
	err := c.GraphQL.Mutate(ctx, &mutation, input, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create project %q: %w", title, err)
	}
	return mutation.CreateProjectV2.ProjectV2.ID, nil
}

// LinkProjectV2ToRepo mutation types

type LinkProjectV2ToRepoMutation struct {
	LinkProjectV2ToRepository struct {
		Repository struct {
			ID string
		}
	} `graphql:"linkProjectV2ToRepository(input: $input)"`
}

type LinkProjectV2ToRepositoryInput struct {
	ProjectID    githubv4.ID `json:"projectId"`
	RepositoryID githubv4.ID `json:"repositoryId"`
}

// LinkProjectV2ToRepo links a Projects V2 board to a repository.
func (c *Client) LinkProjectV2ToRepo(ctx context.Context, projectID, repoID string) error {
	var mutation LinkProjectV2ToRepoMutation
	input := LinkProjectV2ToRepositoryInput{
		ProjectID:    githubv4.ID(projectID),
		RepositoryID: githubv4.ID(repoID),
	}
	err := c.GraphQL.Mutate(ctx, &mutation, input, nil)
	if err != nil {
		return fmt.Errorf("failed to link project to repo: %w", err)
	}
	return nil
}
