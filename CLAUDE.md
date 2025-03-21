# Useful1 Development Guide

## Build & Test Commands
- Build: `make build`
- Run: `make run` 
- Run: `./bin/useful1 [command]`
- Run TUI mode: `./bin/useful1 --tui [command]`
- Test all: `make test`
- Test single: `go test -v ./path/to/package -run TestName`
- Coverage: `make test-coverage`
- Lint: `make lint`
- Format: `make fmt`

## File Operations
- Convert line endings to LF: `find /root/Development/useful1 -type f -name "*.go" -exec sed -i 's/\r$//' {} \;`
- Search for patterns with ripgrep: `rg -l "pattern" --type go`
- Get exact context for editing: `rg -A 3 -B 3 "pattern" file.go`

## Command Usage
- Configuration: `./bin/useful1 config`
- Execute Claude CLI directly: `./bin/useful1 execute [arguments...]`

### GitHub Monitoring Commands
- Monitor with CLI (default): `./bin/useful1 monitor --repo [owner/repo] --interval [seconds]` (default: 60 seconds)
- Run once: `./bin/useful1 monitor --repo [owner/repo] --once`
- Monitor with TUI: `./bin/useful1 --tui monitor --repo [owner/repo] --interval [seconds]` (default: 60 seconds)
  - Press 'q' to exit the TUI
- Use TUI main menu: `./bin/useful1 --tui`
- Run with debug logs: `./bin/useful1 --log-level debug monitor --repo [owner/repo]`
- Enable auto-respond: `./bin/useful1 monitor --repo [owner/repo] --auto-respond`

## Code Style
- **Variables**: Use clear variable names which explain the purpose of the variable
- **Logic**: If logic for branching becomes complicated, extract boolean comparisons to variables and then use these variables in the if statments
- **Comments**: If a comment is needed to explain a block of code, extract it to a function either inline as a closure or above
- **OOP**: Prefer composition over inheritance
- **Imports**: Standard library first, then external packages
- **Formatting**: Always run `gofmt` and any tests before committing
- **Types**: Use strong typing; minimize interface{}
- **Naming**: CamelCase for exported, camelCase for private
- **Errors**: Explicit error checking, wrap with context. Always prefer guard clauses and early returns for invalid input to functions. Always assign errors to variables and bubble up errors to the calling function
- **Structure**: Follow Go standard project layout
  - `cmd/`: Entry points
  - `internal/`: Private application code
  - Packages organized by domain

## Dependencies
- Go 1.23.6
- Key packages: go-github/v45, cobra, viper

If you do not know how to use a dependency use go doc to see the interface documentation

## File Edit Best Practices
- Get exact context before editing: Use ripgrep to find the exact string context including whitespace
- Use multi-line edit blocks: Include at least 3-5 lines before and after the change point
- For UI components: Check for instruction text that needs updating when modifying behaviors
- Test all edits: After making changes, run `make build` `make test` and `make lint-all` to verify correctness
- Never add comments like "this would be replaced in real implementation" - implement the actual functionality
- Never create stub/dummy implementations - write complete, production-ready code for all features
- Always ensure the code works correctly with actual, parsed values rather than hardcoded defaults