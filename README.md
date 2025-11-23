# WorkFlowy CLI

A command-line interface for interacting with WorkFlowy, including powerful analytics and reporting features.

## Features

- **Tree Operations**: List, get, and navigate your WorkFlowy tree structure
- **Markdown Export**: Convert WorkFlowy items to markdown format
- **Analytics Reports**:
  - Descendant count reports with threshold filtering
  - Rank nodes by immediate children count
  - Find oldest nodes by creation date
  - Find oldest nodes by modification date
- **Report Upload**: Upload analytics reports directly back to WorkFlowy
- **Backup Support**: Work with local backup files for faster operations

## Installation

### Via Homebrew

```bash
brew install mholzen/workflowy/workflowy
```

### From Source

```bash
git clone https://github.com/mholzen/workflowy.git
cd workflowy
go build ./cmd/workflowy
```

## Setup

### Get Your API Key

1. Visit https://workflowy.com/api-key/
2. Save your API key to `~/.workflowy/api.key`

```bash
mkdir -p ~/.workflowy
echo "your-api-key-here" > ~/.workflowy/api.key
chmod 600 ~/.workflowy/api.key
```

## Usage

### Basic Commands

#### List Items

```bash
# List all top-level items
workflowy list

# List with custom depth
workflowy list --depth 3

# Use a backup file for faster operations
workflowy list --use-backup-file
```

#### Get a Specific Item

```bash
# Get item by ID
workflowy get <item-id>

# Get with depth
workflowy get <item-id> --depth 2
```

#### Convert to Markdown

```bash
# Convert to markdown and print to stdout
workflowy markdown <item-id>

# Save to file
workflowy markdown <item-id> --output report.md
```

### Analytics Reports

#### Descendant Count Report

Analyze which nodes have the most descendants:

```bash
# Generate descendant count report
workflowy report count

# With custom threshold (show nodes with >5% of total descendants)
workflowy report count --threshold 0.05

# Upload report to WorkFlowy
workflowy report count --upload

# Upload to specific parent node
workflowy report count --upload --parent-id <node-id>
```

#### Rank by Children Count

Find nodes with the most immediate children:

```bash
# Top 20 nodes by children count (default)
workflowy report children

# Top 10 nodes
workflowy report children --top-n 10

# Upload to WorkFlowy
workflowy report children --upload
```

#### Find Oldest Nodes

Find nodes that haven't been updated in a long time:

```bash
# By creation date
workflowy report created --top-n 20

# By modification date
workflowy report modified --top-n 20

# Upload to WorkFlowy
workflowy report created --upload
workflowy report modified --upload
```

### Upload Options

All report commands support these upload flags:

- `--upload`: Upload report to WorkFlowy instead of printing
- `--parent-id <id>`: Parent node ID for uploaded report (default: root)
- `--position <top|bottom>`: Position in parent node

Example:
```bash
workflowy report count --upload --parent-id abc123 --position top
```

### Global Options

- `--format <json|md>`: Output format (default: md)
- `--depth <n>`: Recursion depth for tree operations (default: 2)
- `--api-key-file <path>`: Path to API key file (default: ~/.workflowy/api.key)
- `--log <level>`: Log level: debug, info, warn, error (default: info)
- `--include-empty-names`: Include items with empty names

### Working with Backups

For faster operations, especially with large trees, use local backup files:

```bash
# Most commands support --use-backup-file
workflowy list --use-backup-file
workflowy report count --use-backup-file

# Specify a specific backup file
workflowy list --use-backup-file=/path/to/backup.json
```

The CLI caches export data in `~/.workflowy/export-cache.json` for improved performance.

## Examples

### Generate and upload a comprehensive report

```bash
# Find nodes with many descendants and upload the report
workflowy report count --threshold 0.01 --upload
```

### Find stale content

```bash
# Find your 50 oldest unmodified nodes
workflowy report modified --top-n 50
```

### Export to markdown

```bash
# Export a specific project to markdown
workflowy markdown <project-id> --output my-project.md
```

## API Reference

This tool uses the WorkFlowy API. For more information:

- API Documentation: https://workflowy.com/api-reference/
- Community Discussion: https://community.workflowy.com/t/ruxdimentary-api-try-using-it-and-tell-me-what-you-think/185
- Get API Key: https://workflowy.com/api-key/

## Development

### Build

```bash
go build ./cmd/workflowy
```

### Run Tests

```bash
go test ./...
```

### Project Structure

```
.
├── cmd/workflowy/       # Main CLI application
├── pkg/
│   ├── workflowy/       # Core WorkFlowy API client
│   ├── reports/         # Report generation and upload
│   └── formatter/       # Output formatting
└── README.md
```

## License

MIT

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
