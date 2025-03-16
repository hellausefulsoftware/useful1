# Useful1 Development Guide

## Build & Test Commands
- Build: `make build`
- Run: `make run`
- Test all: `make test`
- Test single: `go test -v ./path/to/package -run TestName`
- Coverage: `make test-coverage`
- Lint: `make lint`
- Format: `make fmt`

## Code Style
- **Variables**: Use clear variable names which explain the purpose of the variable
- **Logic**: If logic for branching becomes complicated, extract boolean comparisons to variables and then use these variables in the if statments
- **Comments**: If a comment is needed to explain a block of code, extract it to a function either inline as a closure or above
- **OOP**: Prefer composition over inheritance
- **Imports**: Standard library first, then external packages
- **Formatting**: Always run `gofmt` and any tests before committing
- **Types**: Use strong typing; minimize interface{}
- **Naming**: CamelCase for exported, camelCase for private
- **Errors**: Explicit error checking, wrap with context. Always prefer guard clauses and early returns for invalid input to functions 
- **Structure**: Follow Go standard project layout
  - `cmd/`: Entry points
  - `internal/`: Private application code
  - Packages organized by domain

## Dependencies
- Go 1.23.6
- Key packages: go-github/v45, cobra, viper