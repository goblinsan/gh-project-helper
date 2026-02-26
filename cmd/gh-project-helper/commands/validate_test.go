package commands

import (
	"testing"

	"github.com/goblinsan/gh-project-helper/pkg/types"
)

func TestValidatePlan_Valid(t *testing.T) {
	plan := types.Plan{
		Project:    "Test",
		Repository: "owner/repo",
		Milestones: []types.Milestone{
			{Title: "Phase 1"},
		},
		Epics: []types.Epic{
			{
				Title:     "Epic 1",
				Milestone: "Phase 1",
				Children: []types.Issue{
					{Title: "Child 1"},
				},
			},
		},
	}
	errs := validatePlan(plan)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidatePlan_MissingRequired(t *testing.T) {
	plan := types.Plan{}
	errs := validatePlan(plan)
	if len(errs) < 2 {
		t.Errorf("expected at least 2 errors for missing repository and project, got %d: %v", len(errs), errs)
	}
}

func TestValidatePlan_InvalidRepoFormat(t *testing.T) {
	plan := types.Plan{
		Project:    "Test",
		Repository: "invalid-no-slash",
	}
	errs := validatePlan(plan)
	found := false
	for _, e := range errs {
		if e == `repository "invalid-no-slash" must be in owner/repo format` {
			found = true
		}
	}
	if !found {
		t.Errorf("expected repo format error, got %v", errs)
	}
}

func TestValidatePlan_UndefinedMilestone(t *testing.T) {
	plan := types.Plan{
		Project:    "Test",
		Repository: "owner/repo",
		Epics: []types.Epic{
			{
				Title:     "Epic 1",
				Milestone: "Nonexistent Phase",
			},
		},
	}
	errs := validatePlan(plan)
	found := false
	for _, e := range errs {
		if e == `epics[0] "Epic 1": milestone "Nonexistent Phase" is not defined in milestones section` {
			found = true
		}
	}
	if !found {
		t.Errorf("expected undefined milestone error, got %v", errs)
	}
}

func TestValidatePlan_DuplicateTitles(t *testing.T) {
	plan := types.Plan{
		Project:    "Test",
		Repository: "owner/repo",
		Milestones: []types.Milestone{
			{Title: "Phase 1"},
			{Title: "Phase 1"},
		},
		Epics: []types.Epic{
			{Title: "Epic 1", Children: []types.Issue{
				{Title: "Child 1"},
				{Title: "Child 1"},
			}},
			{Title: "Epic 1"},
		},
	}
	errs := validatePlan(plan)
	if len(errs) != 3 {
		t.Errorf("expected 3 duplicate errors (milestone, epic, child), got %d: %v", len(errs), errs)
	}
}

func TestValidatePlan_MissingChildTitle(t *testing.T) {
	plan := types.Plan{
		Project:    "Test",
		Repository: "owner/repo",
		Epics: []types.Epic{
			{Title: "Epic 1", Children: []types.Issue{
				{Title: ""},
			}},
		},
	}
	errs := validatePlan(plan)
	found := false
	for _, e := range errs {
		if e == "epics[0].children[0]: title is required" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected child title error, got %v", errs)
	}
}
