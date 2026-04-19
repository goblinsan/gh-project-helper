package github

import gogithub "github.com/google/go-github/v66/github"

// IssueRef holds the identifying details of a GitHub issue.
type IssueRef struct {
	Number int
	NodeID string
	URL    string
}

// IssueSyncRequest describes the desired state for an existing issue.
type IssueSyncRequest struct {
	Title           string
	Body            string
	MilestoneNumber *int
	Labels          []string
	Assignees       []string
}

// MilestoneSyncResult reports how milestone sync changed repository state.
type MilestoneSyncResult struct {
	Milestone *gogithub.Milestone
	Created   bool
	Updated   bool
}
