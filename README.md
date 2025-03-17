# useful1: Automated GitHub Issue Resolution System

`useful1` is a powerful automation tool that takes GitHub issue resolution to the next level. When developers identify trivial or straightforward issues, they can assign them to useful1, which will automatically read the context, implement a fix, create a branch, and submit a pull request—transforming issue discussions into code contributions without manual intervention.

## Core Concepts

### The Problem We're Solving

Development teams regularly encounter issues that:
1. Are trivial or straightforward to fix
2. Have clear solutions already identified in comments
3. Require standard, repeatable implementation patterns
4. Take valuable developer time away from more complex tasks

These issues create unnecessary context-switching for developers and slow down the overall development process, despite having well-understood solutions.

### Our Solution

`useful1` transforms issue resolution by:

1. **Monitoring repositories** for issues assigned to it
2. **Reading the full context** including issue description and all comments
3. **Understanding the requested fix** from developer explanations
4. **Automatically implementing changes** by creating a new branch
5. **Submitting a pull request** with the implemented fix
6. **Notifying the original developer** for review

This automated workflow converts issue discussions directly into code changes, dramatically reducing the overhead for routine fixes.

## Workflow Diagram

The following diagram illustrates the complete workflow of useful1:

![useful1 Automated Issue Resolution Workflow](docs/workflow-diagram.svg)

## Project Structure

The project follows a modular Go architecture with clear separation of concerns:

```
useful1/
├── cmd/
│   └── useful1/
│       └── main.go         # Entry point and command-line interface
├── internal/
│   ├── anthropic/
│   │   └── analysis.go     # Integration with Anthropic's API
│   ├── auth/
│   │   └── auth.go         # Authentication for GitHub and Anthropic
│   ├── budget/
│   │   └── budget.go       # Budget management for API usage
│   ├── cli/
│   │   └── executor.go     # Executes the wrapped CLI tool
│   ├── common/
│   │   └── vcs/            # VCS abstractions and interfaces
│   │       ├── issue.go
│   │       ├── monitor.go
│   │       └── service.go
│   ├── config/
│   │   └── config.go       # Configuration management
│   ├── github/
│   │   ├── client.go       # GitHub API client
│   │   ├── provider.go     # GitHub service provider
│   │   └── github_adapter.go # GitHub implementation of VCS interfaces
│   ├── models/
│   │   └── models.go       # Common data models
│   ├── tui/
│   │   └── app.go          # Terminal user interface
│   └── workflow/
│       ├── workflow.go     # Implementation workflow orchestration
│       └── services/       # Implementation workflow services
│           ├── github.go
│           └── provider.go
├── Dockerfile              # Container definition
├── docker-compose.yml      # Container orchestration
├── go.mod                  # Go module definition
└── Makefile                # Build and development commands
```

### Key Components

#### 1. Command-Line Interface (cmd/useful1/main.go)

The main entry point that provides these commands:
- `config`: Interactive setup for authentication and preferences
- `monitor`: Continuous monitoring for assigned issues
- `respond`: Manual response to specific issues
- `pr`: Create pull requests
- `test`: Run tests through the CLI tool

#### 2. Configuration Manager (internal/config/config.go)

Handles loading, saving, and validating application settings, stored in `~/.useful1/config.json` with credentials securely base64 encoded.

#### 3. VCS Abstractions (internal/common/vcs)

Provides platform-agnostic interfaces and abstractions for version control systems:
- Monitoring and issue discovery
- Repository operations
- Platform-agnostic data models

#### 4. GitHub Implementation (internal/github)

Implements the VCS interfaces for GitHub:
- `github_adapter.go`: Implements the VCS service interface
- `client.go`: Low-level GitHub API operations
- `provider.go`: Creates GitHub service instances

#### 5. Workflow Orchestration (internal/workflow)

Handles the implementation process for issues:
- `workflow.go`: Orchestrates the complete implementation workflow
- `services/github.go`: Implements GitHub-specific workflow services
- Responsible for generating branch names, implementation plans, and executing fixes

#### 6. CLI Executor (internal/cli/executor.go)

Bridges the workflow and Claude CLI tool, handling execution and code generation.

#### 7. Authentication (internal/auth/auth.go)

Manages GitHub and Anthropic authentication through interactive setup.

#### 8. Budget Management (internal/budget/budget.go)

Controls API usage costs by setting and enforcing budget limits.

#### 9. Anthropic Integration (internal/anthropic)

Handles communication with Anthropic's API for generating implementation plans.

## Installation

### Prerequisites

- Go 1.22 or higher
- Git
- A GitHub Personal Access Token with `repo`, `workflow`, and `read:org` scopes
- An Anthropic API Key (for using Claude to understand issues and generate fixes)

### From Source

```bash
# Clone the repository
git clone https://github.com/hellausefulsoftware/useful1.git
cd useful1

# Build the binary
make build

# The binary will be in ./bin/useful1
```


## Configuration

Before using `useful1`, run the interactive configuration:

```bash
./bin/useful1 config
```

The configuration process will guide you through:

1. **GitHub Authentication**: Setting up your GitHub token
2. **Anthropic Authentication**: Configuring your Anthropic API key
3. **Budget Configuration**: Setting spending limits for different operations
4. **Monitoring Settings**: Configuring which repositories to monitor
5. **CLI Tool Configuration**: Specifying the path to your CLI tool

All settings are stored in `~/.useful1/config.json` for future use with credentials securely base64 encoded.

## Example Implementation Workflow

Here's a concrete example of how useful1 resolves an issue:

### 1. Original Issue

```
Title: Button color inconsistency in dark mode
#42

The primary button in the header maintains its light blue color even when dark mode is active.
This causes poor contrast and accessibility issues.

According to our design system, primary buttons should change to a darker blue (#1a56e8) in dark mode.
```

### 2. Developer Comment

```
@devleader:
This is a simple CSS fix. We need to modify the ThemeProvider component to apply the correct color
in dark mode. The button class should have its color changed to #1a56e8 when the theme is set to dark.
This can be found in src/components/ThemeProvider.tsx.

Assigning to @useful1 to implement this fix.
```

### 3. Bot Takes Action

useful1 automatically:
- Creates a branch: `bugfix/useful1-42`
- Modifies the ThemeProvider.tsx file to update the color
- Commits with a detailed message
- Creates a PR referencing the issue
- Tags the developer for review
- Comments on the original issue with a link to the PR

For the complete workflow with code examples, check out our [example implementation workflow](docs/example-workflow.md).

## Usage

### Monitoring for Assigned Issues

Start the continuous monitoring process:

```bash
./bin/useful1 monitor
```

This will:
- Check for GitHub issues assigned to the bot
- Process each issue by reading the context and understanding the requested fix
- Create a new branch following the naming convention: `bugfix|chore|feature/useful1-[issue-number]`
- Implement the necessary changes
- Create a pull request with the changes
- Tag the original assignor for review
- Comment on the issue with a link to the PR

For a one-time check:

```bash
./bin/useful1 monitor --once
```

You can specify a specific repository to monitor:

```bash
./bin/useful1 monitor --repo owner/repo --once
```

### Manual Issue Processing

Process a specific issue:

```bash
./bin/useful1 respond 123 --implement
```

Where `123` is the issue number and `--implement` flag triggers the fix implementation.

### Creating Pull Requests Manually

```bash
./bin/useful1 pr feature-branch "New feature implementation"
```

### Running Tests

```bash
./bin/useful1 test integration
```

### Command-Line Interface

The default mode is CLI with structured JSON output, making it suitable for integration with scripts or other programs:

```bash
# Get response for an issue in JSON format
./bin/useful1 respond --issue 123 --owner myorg --repo myrepo

# Create a PR with structured output
./bin/useful1 pr --branch feature-branch --base main --title "New feature"

# Run tests with structured results
./bin/useful1 test --suite integration
```

### Terminal User Interface (TUI)

For interactive usage, you can enable the TUI with the `--tui` flag:

```bash
# Launch the TUI
./bin/useful1 --tui

# Go directly to the respond screen in TUI
./bin/useful1 --tui respond
```

The CLI mode returns structured JSON responses:

```json
{
  "status": "success",
  "issue_number": 123,
  "owner": "myorg",
  "repo": "myrepo",
  "response_length": 1423,
  "timestamp": "2025-03-15T14:32:45Z",
  "url": "https://github.com/myorg/myrepo/issues/123"
}
```

Error responses follow a consistent format:

```json
{
  "status": "error",
  "message": "Error message details",
  "timestamp": "2025-03-15T14:32:45Z"
}
```

## Technical Implementation

### Issue Resolution Workflow

The system follows a functional chain pattern with clear separation of concerns:

#### VCS Monitoring
1. The monitor identifies issues assigned to the bot
2. For each assigned issue, it:
   - Retrieves the full issue details including all comments
   - Returns these issues to the main execution flow
   - Does NOT directly initiate workflow processing

#### Main Execution Flow
1. Receives discovered issues from the VCS monitor
2. For each issue:
   - Applies filtering and validation logic
   - Determines if implementation is required
   - Invokes the workflow orchestrator with validated issues

#### Implementation Workflow
1. The workflow orchestrator processes each issue it receives:
   - Analyzes the context to understand the required fix
   - Determines the appropriate branch type (bugfix, chore, feature)
   - Creates a new branch from the repository's default branch
   - Implements the necessary changes using the CLI tool
   - Commits the changes with a detailed commit message
   - Creates a pull request targeting the default branch
   - Tags the original assignor for review
   - Comments on the issue with a link to the PR

This functional chain ensures each component has a single responsibility and can be tested independently. VCS operations are kept separate from business logic implementation, allowing for clean extension and testing.

### Branch Naming Convention

The system follows a strict branch naming convention:
```
[type]/useful1-[issue-number]
```

Where `[type]` is one of:
- `bugfix` for bug fixes
- `chore` for maintenance tasks
- `feature` for new features

This convention ensures consistency and traceability between issues and PRs.

### Pull Request Creation

Pull requests created by the bot include:
- A title referencing the original issue
- A detailed description of the changes made
- A reference to the issue being fixed
- Proper tags for the original assignor to review

### CLI Tool Integration

Your CLI tool should accept these arguments for implementing fixes:
```
implement --issue-file /path/to/issue.txt --owner repo-owner --repo repo-name --number 123 --branch bugfix/useful1-123
```

The issue file contains the full context of the issue, allowing the tool to understand the required changes.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the LICENSE file for details.