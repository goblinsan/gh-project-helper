# gh-project-helper

A Model Context Protocol (MCP) compliant CLI tool to convert plans into GitHub project milestones and issues.

## Features

- MCP-compliant CLI architecture
- GitHub REST API v3 support via `google/go-github`
- GitHub GraphQL API support via `shurcooL/githubv4` (essential for Projects V2)
- Robust CLI framework using `spf13/cobra`
- Configuration management with `spf13/viper`

## Requirements

- Go 1.23 or higher

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
./gh-project-helper version

# Authenticate and display user info
./gh-project-helper whoami --token YOUR_GITHUB_TOKEN
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

