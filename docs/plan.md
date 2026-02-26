# Project: `gh-architect` Implementation Plan

## 1. High-Level Architecture

**Goal:** A lightweight, single-binary CLI tool that automates the creation and linkage of GitHub Projects (V2), Epics, and Issues from a simple definition file.

* **Language:** Go (Golang) 1.23+
* **Interface:**
    * **CLI Mode:** Human-readable YAML input for manual execution.
    * **MCP Mode:** JSON-RPC over Stdio for AI Agent execution (Claude, Gemini, etc.).
* **Primary API:** GitHub GraphQL API v4 (Required for Projects V2 mutations).
* **Epic Strategy:** "Tracking Issues" (Parent Issue containing a dynamic Tasklist of Child Issues).


---

## 2. The Schema (`plan.yaml`)

This is the contract between the user (or AI agent) and the tool.

```yaml
# plan.yaml
project: "Platform Migration 2026"  # The specific Project V2 Board Title
repository: "my-org/core-backend"   # Owner/Repo

milestones:
  - title: "Phase 1: Database"
    due_on: "2026-04-01"
    description: "Schema consolidation and migration."

epics:
  - title: "User Schema Refactor"
    body: "Refactoring the user table to support multi-tenancy."
    milestone: "Phase 1: Database"
    status: "Todo"        # Maps to Project V2 Status Field
    labels: ["backend", "high-priority"]
    assignees: ["octocat"]

    # Child Issues are created first, then linked to the Epic
    children:
      - title: "Create migration script"
        body: "Write SQL to alter table users..."
        labels: ["database"]
      - title: "Update ORM models"
        body: "Update Gorm structs..."

---

## 3. Implementation Phases

### Phase 1: Core Scaffolding & Auth

Set up the Go module and authentication. Do not build a custom auth flow; leverage the existing `gh` CLI environment.

* **Dependencies:**
    * `github.com/spf13/cobra` (CLI commands)
    * `github.com/shurcooL/githubv4` (GraphQL client)
    * `golang.org/x/oauth2` (Token handling)
* **Auth Logic:**
    1.  Check `GITHUB_TOKEN` env var.
    2.  If missing, try to read from `gh auth token` (shell out or read config).
    3.  Fail if no token found.

### Phase 2: The Logic Engine (`apply` command)

This is the heart of the tool. The `apply` command processes the YAML file.

**Logic Flow:**

1.  **Resolve Context:**
    * Fetch `Repository ID` (Node ID).
    * Fetch `Project V2 ID` by matching the title.
    * *Cache these IDs to avoid repeated lookups.*
2.  **Milestone Sync:**
    * Check if defined milestones exist.
    * If yes -> Get ID. If no -> Create -> Get ID.
3.  **Execution Loop (Per Epic):**
    * **Step A (Children):** Iterate through `children`. Create each Issue via GraphQL. Store the returned `Issue Number` and `Node ID`.
    * **Step B (Epic Body):** Construct the Epic's markdown body. Append the Tasklist:
        - [ ] #14 Create migration script
        - [ ] #15 Update ORM models
    * **Step C (Create Epic):** Create the Parent Issue with the `[Epic]` label and the constructed body.
    * **Step D (Project Linkage):**
        * Add the Epic and all Children to the Project V2 Board.
        * Update the `Status` field for all items to "Todo" (or as defined).

### Phase 3: The "MCP" Mode (`serve` command)

Enable AI agents to use this tool natively.

* **Command:** `gh-architect serve`
* **Protocol:** Model Context Protocol (Stdio)
* **Tool Definition:**
    * **Name:** `apply_project_plan`
    * **Description:** "Takes a JSON structure defining Epics and Issues and creates them in GitHub."
    * **Input Schema:** JSON equivalent of the YAML above.
* **Behavior:** When called, it runs the exact same logic as `apply`, but reads from the JSON argument instead of a file, and returns a JSON success/fail report.

### Phase 4: Distribution

* **Repo:** `github.com/yourname/homebrew-tap`
* **Formula:** Create a `gh-architect.rb` formula that builds from source or downloads the binary release.

---