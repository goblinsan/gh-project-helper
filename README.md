# gh-project-helper

A Model Context Protocol (MCP) compliant CLI tool to convert plans into GitHub project milestones and issues.

## Features

- MCP-compliant CLI architecture
- GitHub REST API v3 support via `google/go-github`
- GitHub GraphQL API support via `shurcooL/githubv4` (essential for Projects V2)
- Robust CLI framework using `spf13/cobra`
- Configuration management with `spf13/viper`

## Requirements

- Go 1.24 or higher

## Installation

```bash
go build -o gh-project-helper ./cmd/gh-project-helper
```

## Configuration

The tool can be configured via:

1. **Command-line flags**: `--token`, `--config`
2. **Environment variables**: Prefix with `GH_PROJECT_HELPER_` (e.g., `GH_PROJECT_HELPER_TOKEN`)
3. **Config file**: `~/.gh-project-helper.yaml`

### Example Config File

```yaml
token: ghp_yourGitHubPersonalAccessToken
```

## Usage

```bash
# Display help
./gh-project-helper --help

# Check version
./ghp version

# Authenticate and display user info
./ghp whoami --token YOUR_GITHUB_TOKEN
```

## Project Structure

```
.
├── cmd/
│   └── gh-project-helper/
│       ├── commands/          # Cobra commands
│       │   ├── root.go        # Root command with Viper config
│       │   ├── version.go     # Version command
│       │   └── whoami.go      # Example command using GitHub client
│       └── main.go            # Application entry point
├── pkg/
│   └── github/
│       └── client.go          # Unified GitHub client (REST + GraphQL)
├── internal/                  # Internal packages
└── go.mod                     # Go module dependencies
```

## Dependencies

- **github.com/google/go-github/v66** - GitHub REST API v3 client
- **github.com/shurcooL/githubv4** - GitHub GraphQL API client (for Projects V2)
- **github.com/spf13/cobra** - CLI framework
- **github.com/spf13/viper** - Configuration management
- **golang.org/x/oauth2** - OAuth2 authentication

## Development

### Building

```bash
go build -o gh-project-helper ./cmd/gh-project-helper
```

### Testing

```bash
go test ./...
```

## License

See LICENSE file for details.



## Gateway Deployment

On the gateway host, this repo is intended to have its own repo-level GitHub Actions runner. Merges to `main` should rebuild the helper binary into `/home/jimmothy/.local/bin/ghp`. The workflow builds a static Linux binary (`CGO_ENABLED=0`) so the helper can run inside the Alpine-based `gateway-api` container. `gateway-api` mounts that host directory and prefers the runner-managed binary when present, so helper updates can go live without rebuilding the API image.


When `ghp` creates a repository, it initializes the repository with a README. The engine still performs a README check afterward as a safety net for existing repos without one.
