# useful1: a cli wrapper for Agentic Github / Gitlab / Gogs Issue Resolution

⚠️⚠️⚠️ WARNING ⚠️⚠️⚠️

This is an alpha implementation wrapping a Research Preview. It's basically a given it may break something. 
Run it at your own risk. It was also mostly written by claude code with some gudiance by https://github.com/mattbucci

`useful1` is a bot designed to assist you with managing issues in your repositories by automating fixes and pull request creation.

Once started in monitor mode, the bot automatically responds to GitHub issues assigned to it. It's recommended to run this program in a virtual machine (VM) as a non-root user with limited permissions to prevent potentially harmful operations.

Recommended operating system: Alpine Linux.

Non-standard dependencies required:
- nodejs
- npm
- git
- git-lfs
- github-cli
- ripgrep
- vim

## Usage

### Monitor Issues

Start monitoring:
```bash
./bin/useful1 monitor
```

Single-check:
```bash
./bin/useful1 monitor --once
```

Specific repository:
```bash
./bin/useful1 monitor --repo owner/repo --once
```

### GitHub Monitoring 

Monitor GitHub issues:
```bash
./bin/useful1 monitor --repo owner/repo
```

### CLI

Interactive TUI mode:
```bash
./bin/useful1 --tui
```

Configuration:
```bash
./bin/useful1 config
```

## Project Structure

```
useful1/
├── cmd/useful1/main.go            # CLI entry point
├── internal/
│   ├── anthropic/                 # AI integration
│   ├── auth/                      # Authentication
│   ├── budget/                    # API budgeting
│   ├── cli/                       # CLI execution
│   ├── common/vcs                 # VCS abstractions
│   ├── config/                    # Configuration management
│   ├── models/                    # Data models
│   ├── tui/                       # Terminal UI
│   └── workflow/                  # Workflow orchestration
├── go.mod                         # Go modules
└── Makefile                       # Build automation
```

## Supported Agentic CLI Tools (For making changes)

  - [Claude Code](https://www.npmjs.com/package/@anthropic-ai/claude-code)
  - Roocode (coming soon)
  - Cline (coming soon)

## Supported VCS Platforms (For responding to issues)

- [GitHub](https://github.com)
- GitLab (coming soon)
- Gogs (coming soon)

## Supported AI Summarization tools (For summarizing issues and creating plans)

- [Claude](https://claude.ai)
- OpenAI (coming soon)
- Mistral (coming soon)


## Contributing

Submit pull requests to contribute.

## License

MIT License.

