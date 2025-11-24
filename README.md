# Workflowy CLI

## Table of Contents

- [Features](#features)
- [Installation](#installation)
  - [Via Homebrew](#via-homebrew)
  - [From Source](#from-source)
- [Setup](#setup)
  - [Get Your API Key](#get-your-api-key)
- [Usage](#usage)
  - [Read from Backup](#read-from-backup)
  - [Export and Report Commands](#export-and-report-commands)
  - [Advanced: Uploading Reports](#advanced-uploading-reports)
- [Backup vs Export](#backup-vs-export)
- [Caching](#caching)
- [Rate Limiting](#rate-limiting)
- [Error and Status Reporting](#error-and-status-reporting)


A command-line interface for interacting with Workflowy, including fetching, updating and
creating nodes, usage reports and markdown generation.

## Features

- **Node Operations**: Get, List, Post, Update, and Tree to operate on nodes.
- **Usage Reports**: Understand where the majority of your nodes are stored, which nodes have many children or which ones are possibly stale:
  - Descendant count reports with threshold filtering.
  - Rank nodes by immediate children count
  - Find oldest nodes by creation date
  - Find oldest nodes by modification date
- **Report Upload**: Upload usage reports using the API or paste
  the markdown output into Workflowy
- **Markdown Export**: Convert a tree of nodes to a Markdown document
- **Backup File Support**: Operates on a local backup files for faster operations
- **Local Caching**: Caches


## Installation

### Via Homebrew

```bash
brew install mholzen/workflowy/workflowy-cli
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

### Quick Start

Generate the descedant count report, copy it to your clipboard

```bash
workflowy report count | pbcopy
```

Then simply paste to in Workflowy.  It will produce a report like this:

```
# Descendant Count Report

Root: Root
Threshold: 1.00%
Total descendants: 34437

- [Root](https://workflowy.com/#/root) (descendants: 34437, children: 8, ratio: 100.00%)
  - [projects](https://workflowy.com/#/xxx) (descendants: 31532, children: 3, ratio: 91.56%)
    - [project A](https://workflowy.com/#/yyy) (descendants: 31314, children: 4, ratio: 90.93%)
      - [phase 1](https://workflowy.com/#/zzz) (descendants: 26951, children: 3, ratio: 78.26%)
```

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

#### Get Entire Tree

```bash
# Get everything (single efficient API call)
workflowy tree

# Use backup file for fastest access
workflowy tree --use-backup-file
```

**Note**: `tree` uses the export API to fetch everything in one call. `get` with high depth makes multiple API calls and is better for specific subtrees.

#### Update a Node

```bash
# Update node content
workflowy update <item-id> "new content"

# Update note only
workflowy update <item-id> --note "my note"

# Update multiple fields
workflowy update <item-id> --name "new title" --note "new note"
```

#### Convert to Markdown

```bash
# Convert to markdown and print to stdout
workflowy markdown <item-id>

# Save to file
workflowy markdown <item-id> --output report.md
```

### Usage Reports

#### Descendant Count Report

Rank nodes by the number of total descendant nodes:

```bash
# Generate descendant count report
workflowy report count

# With custom threshold (show nodes with >5% of total descendants)
workflowy report count --threshold 0.05
```

#### Rank by Children Count

Find nodes with the most immediate children:

```bash
# Top 20 nodes by children count (default)
workflowy report children

# Top 10 nodes
workflowy report children --top-n 10
```

#### Find Oldest Nodes

Find nodes that haven't been updated in a long time:

```bash
# By creation date
workflowy report created --top-n 20

# By modification date
workflowy report modified --top-n 20
```

### Upload Options

All report commands support these upload flags:

- `--upload`: Upload report to Workflowy instead of printing
- `--parent-id <id>`: Parent node ID for uploaded report (default: root)
- `--position <top|bottom>`: Position in parent node (default: top)

Example:
```bash
workflowy report count --upload --parent-id xxx-yyy-zzz --position top
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

The default location for the backup file is the most recent file
`~/Dropbox/Apps/Workflowy/Data` that follows the `*.workflowy.backup` pattern.

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

This tool uses the Workflowy API. For more information:

- API Documentation: https://workflowy.com/api-reference/
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
│   ├── workflowy/       # Core Workflowy API client
│   ├── reports/         # Report generation and upload
│   └── formatter/       # Output formatting
└── README.md
```

## License

MIT

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
