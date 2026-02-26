package engine

import (
	"context"
	"net/url"
	"testing"

	ghclient "github.com/goblinsan/gh-project-helper/pkg/github"
	"github.com/goblinsan/gh-project-helper/pkg/types"
	gogithub "github.com/google/go-github/v66/github"
	"github.com/shurcooL/githubv4"
)

// mockClient implements GitHubClient for testing.
type mockClient struct {
	issueCounter   int
	createdIssues  []string
	projectItems   []string
	statusUpdates  []string
	labelRequests  []string
}

func newMockClient() *mockClient {
	return &mockClient{}
}

func (m *mockClient) GetRepositoryID(_ context.Context, _, _ string) (string, error) {
	return "repo-node-id", nil
}

func (m *mockClient) GetProjectV2ID(_ context.Context, _, _ string) (string, error) {
	return "project-node-id", nil
}

func (m *mockClient) GetProjectV2StatusFieldOptions(_ context.Context, _ githubv4.ID) (githubv4.ID, map[string]string, error) {
	return githubv4.ID("status-field-id"), map[string]string{
		"Todo":        "todo-option-id",
		"In Progress": "in-progress-option-id",
		"Done":        "done-option-id",
	}, nil
}

func (m *mockClient) GetOrCreateMilestone(_ context.Context, _, _, title, _, _ string) (*gogithub.Milestone, error) {
	num := 1
	return &gogithub.Milestone{
		Number: &num,
		Title:  &title,
	}, nil
}

func (m *mockClient) GetMilestoneID(_ context.Context, _, _ string, _ int) (string, error) {
	return "milestone-node-id", nil
}

func (m *mockClient) FindIssueByTitle(_ context.Context, _, _, _ string) (int, string, error) {
	return 0, "", nil // No existing issues by default
}

func (m *mockClient) GetOrCreateLabel(_ context.Context, _, _, labelName string) (githubv4.ID, error) {
	m.labelRequests = append(m.labelRequests, labelName)
	return githubv4.ID("label-" + labelName), nil
}

func (m *mockClient) GetUserID(_ context.Context, login string) (githubv4.ID, error) {
	return githubv4.ID("user-" + login), nil
}

func (m *mockClient) CreateIssue(_ context.Context, input githubv4.CreateIssueInput) (*ghclient.CreateIssueMutation, error) {
	m.issueCounter++
	title := string(input.Title)
	m.createdIssues = append(m.createdIssues, title)

	result := &ghclient.CreateIssueMutation{}
	result.CreateIssue.Issue.ID = githubv4.ID("issue-id-" + title)
	result.CreateIssue.Issue.Number = m.issueCounter
	result.CreateIssue.Issue.URL = githubv4.URI{URL: &url.URL{Scheme: "https", Host: "github.com", Path: "/test/repo/issues/" + title}}
	return result, nil
}

func (m *mockClient) AddIssueToProjectV2(_ context.Context, _, contentID githubv4.ID) (*ghclient.AddProjectV2ItemMutation, error) {
	m.projectItems = append(m.projectItems, contentID.(string))
	result := &ghclient.AddProjectV2ItemMutation{}
	result.AddProjectV2ItemById.Item.ID = githubv4.ID("project-item-" + contentID.(string))
	return result, nil
}

func (m *mockClient) UpdateProjectV2ItemStatus(_ context.Context, _, _, _ githubv4.ID, optionID string) error {
	m.statusUpdates = append(m.statusUpdates, optionID)
	return nil
}

func TestApplyPlan_BasicPlan(t *testing.T) {
	mock := newMockClient()
	plan := types.Plan{
		Project:    "Test Project",
		Repository: "owner/repo",
		Milestones: []types.Milestone{
			{Title: "Phase 1", DueOn: "2026-04-01", Description: "First phase"},
		},
		Epics: []types.Epic{
			{
				Title:     "Epic 1",
				Body:      "Epic body",
				Milestone: "Phase 1",
				Status:    "Todo",
				Labels:    []string{"backend"},
				Assignees: []string{"dev1"},
				Children: []types.Issue{
					{Title: "Child 1", Body: "Child body 1", Labels: []string{"database"}},
					{Title: "Child 2", Body: "Child body 2"},
				},
			},
		},
	}

	report, err := ApplyPlan(context.Background(), mock, plan, Options{})
	if err != nil {
		t.Fatalf("ApplyPlan failed: %v", err)
	}

	if report.MilestonesCreated != 1 {
		t.Errorf("expected 1 milestone, got %d", report.MilestonesCreated)
	}
	if report.EpicsCreated != 1 {
		t.Errorf("expected 1 epic, got %d", report.EpicsCreated)
	}
	if report.IssuesCreated != 2 {
		t.Errorf("expected 2 issues, got %d", report.IssuesCreated)
	}
	if report.EpicsSkipped != 0 {
		t.Errorf("expected 0 skipped epics, got %d", report.EpicsSkipped)
	}

	// 2 children + 1 epic = 3 created issues
	if len(mock.createdIssues) != 3 {
		t.Errorf("expected 3 created issues, got %d: %v", len(mock.createdIssues), mock.createdIssues)
	}

	// All 3 issues should be added to project
	if len(mock.projectItems) != 3 {
		t.Errorf("expected 3 project items, got %d", len(mock.projectItems))
	}

	// Status should be updated for all 3 items (2 children + 1 epic)
	if len(mock.statusUpdates) != 3 {
		t.Errorf("expected 3 status updates, got %d", len(mock.statusUpdates))
	}

	// Labels: "database" for child1, "backend" for epic
	if len(mock.labelRequests) != 2 {
		t.Errorf("expected 2 label requests, got %d: %v", len(mock.labelRequests), mock.labelRequests)
	}
}

func TestApplyPlan_InvalidRepository(t *testing.T) {
	mock := newMockClient()
	plan := types.Plan{
		Project:    "Test",
		Repository: "invalid-no-slash",
	}

	_, err := ApplyPlan(context.Background(), mock, plan, Options{})
	if err == nil {
		t.Fatal("expected error for invalid repository format")
	}
}

func TestApplyPlan_DryRun(t *testing.T) {
	mock := newMockClient()
	plan := types.Plan{
		Project:    "Test Project",
		Repository: "owner/repo",
		Milestones: []types.Milestone{
			{Title: "Phase 1", DueOn: "2026-04-01"},
		},
		Epics: []types.Epic{
			{
				Title:  "Epic 1",
				Body:   "Epic body",
				Status: "Todo",
				Children: []types.Issue{
					{Title: "Child 1", Body: "Child body"},
				},
			},
		},
	}

	report, err := ApplyPlan(context.Background(), mock, plan, Options{DryRun: true})
	if err != nil {
		t.Fatalf("ApplyPlan dry-run failed: %v", err)
	}

	// Dry run should not create anything
	if report.MilestonesCreated != 0 {
		t.Errorf("dry-run should create 0 milestones, got %d", report.MilestonesCreated)
	}
	if report.EpicsCreated != 0 {
		t.Errorf("dry-run should create 0 epics, got %d", report.EpicsCreated)
	}
	if report.IssuesCreated != 0 {
		t.Errorf("dry-run should create 0 issues, got %d", report.IssuesCreated)
	}
	if len(mock.createdIssues) != 0 {
		t.Errorf("dry-run should not call CreateIssue, got %d calls", len(mock.createdIssues))
	}
}

func TestApplyPlan_Idempotency(t *testing.T) {
	mock := newMockClient()
	// Override FindIssueByTitle to simulate existing issues
	existingMock := &idempotentMockClient{mockClient: mock, existingIssues: map[string]int{
		"Child 1": 42,
		"Epic 1":  99,
	}}

	plan := types.Plan{
		Project:    "Test Project",
		Repository: "owner/repo",
		Epics: []types.Epic{
			{
				Title:  "Epic 1",
				Body:   "Epic body",
				Status: "Todo",
				Children: []types.Issue{
					{Title: "Child 1", Body: "Child body"},
				},
			},
		},
	}

	report, err := ApplyPlan(context.Background(), existingMock, plan, Options{})
	if err != nil {
		t.Fatalf("ApplyPlan failed: %v", err)
	}

	if report.IssuesSkipped != 1 {
		t.Errorf("expected 1 skipped issue, got %d", report.IssuesSkipped)
	}
	if report.EpicsSkipped != 1 {
		t.Errorf("expected 1 skipped epic, got %d", report.EpicsSkipped)
	}
	if report.IssuesCreated != 0 {
		t.Errorf("expected 0 created issues, got %d", report.IssuesCreated)
	}
	if report.EpicsCreated != 0 {
		t.Errorf("expected 0 created epics, got %d", report.EpicsCreated)
	}
}

// idempotentMockClient wraps mockClient but returns existing issues for specified titles.
type idempotentMockClient struct {
	*mockClient
	existingIssues map[string]int // title -> issue number
}

func (m *idempotentMockClient) FindIssueByTitle(_ context.Context, _, _, title string) (int, string, error) {
	if num, ok := m.existingIssues[title]; ok {
		return num, "existing-node-" + title, nil
	}
	return 0, "", nil
}

func TestReport_String(t *testing.T) {
	r := &Report{
		MilestonesCreated: 2,
		EpicsCreated:      3,
		EpicsSkipped:      1,
		IssuesCreated:     10,
		IssuesSkipped:     2,
	}
	expected := "Summary: 2 milestones synced, 3 epics created (1 skipped), 10 issues created (2 skipped)"
	if r.String() != expected {
		t.Errorf("expected %q, got %q", expected, r.String())
	}
}
