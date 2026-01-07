# Contributing to Workflowy CLI & MCP Server

Thank you for your interest in contributing!

## Getting Started

### Prerequisites

- Go 1.24.3 or later
- A Workflowy account with API access

### Development Setup

```bash
git clone https://github.com/mholzen/workflowy.git
cd workflowy
go build ./cmd/workflowy
go test ./...
```

### Running Tests

```bash
# Unit tests
go test ./...

# Integration tests (requires API key)
just test-integration
```

## How to Contribute

### Reporting Bugs

1. Check existing issues to avoid duplicates
2. Include Go version, OS, and steps to reproduce
3. Provide error messages and expected behavior

### Suggesting Features

Open an issue describing:
- The problem you're trying to solve
- Your proposed solution
- Any alternatives you've considered

### Submitting Changes

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/my-feature`)
3. Make your changes
4. Run tests (`go test ./...`)
5. Commit with a clear message
6. Push and open a Pull Request

## Code Guidelines

### Style

- Follow standard Go conventions (`go fmt`)
- Keep functions focused and small
- Prefer clear names over comments
- Run `go vet` before committing

### Commits

- Use clear, descriptive commit messages
- Keep commits focused on a single change
- Reference issues where applicable (`Fixes #123`)

### Pull Requests

- Describe what the PR does and why
- Include test coverage for new features
- Update documentation if needed
- Keep PRs focused — one feature or fix per PR

## Project Structure

```
.
├── cmd/workflowy/       # CLI application
├── pkg/
│   ├── workflowy/       # Core API client
│   ├── mcp/             # MCP server
│   ├── search/          # Search functionality
│   ├── replace/         # Search and replace
│   ├── reports/         # Report generation
│   └── formatter/       # Output formatting
└── docs/                # Documentation
```

## Questions?

Open an issue or start a discussion. We're happy to help!
