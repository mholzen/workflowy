# Workflowy CLI

## Table of Contents

- [Features](#features)
- [Installation](#installation)
  - [Via Homebrew](#via-homebrew)
  - [From Source](#from-source)
- [Setup](#setup)
  - [Get Your API Key](#get-your-api-key)
- [Usage](#usage)
  - [Basic Commands](#basic-commands)
  - [Usage Reports](#usage-reports)
  - [Global Options](#global-options)
  - [Access Methods and Configuration](#access-methods-and-configuration)
    - [Data Access Methods](#data-access-methods---method)
    - [Configuration Flags](#configuration-flags)
    - [Performance Comparison](#performance-comparison)
    - [Rate Limiting](#rate-limiting)


A command-line interface for interacting with Workflowy, including fetching, updating and
creating nodes, usage reports and markdown generation.

## Features

- **Node Operations**: Get, List, Post, and Update to operate on nodes.
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

- [Root](https://workflowy.com/#/root) (100.0%, 34437 descendants)
  - [projects](https://workflowy.com/#/xxx) (91.6%, 31532 descendants)
    - [project A](https://workflowy.com/#/yyy) (90.9%, 31314 descendants)
      - [phase 1](https://workflowy.com/#/zzz) (78.3%, 26951 descendants)
```

### Basic Commands

#### Get Items (Tree Structure)

The `get` command retrieves items with their hierarchical structure. It automatically chooses the most efficient API method based on depth.

```bash
# Get root items with default depth (2 levels)
workflowy get

# Get specific item with default depth
workflowy get <item-id>

# Get with custom depth (1-3 uses GET API, 4+ uses Export API)
workflowy get <item-id> --depth 5

# Get all descendants (uses Export API)
workflowy get <item-id> --all

# Force specific access method
workflowy get <item-id> --method=backup
workflowy get <item-id> --method=export
workflowy get <item-id> --method=get

# Use specific backup file
workflowy get --method=backup --backup-file=/path/to/file.backup
```

**Smart API Selection**:
- Depth 1-3: Uses GET API (efficient for shallow fetches)
- Depth 4+ or `--all`: Uses Export API (efficient for deep/complete fetches)
- Override with `--method=get`, `--method=export`, or `--method=backup`

#### List Items (Flat List)

The `list` command retrieves items as a flat list without hierarchy. Uses the same smart API selection as `get`.

```bash
# List direct children of root (depth 1)
workflowy list

# List direct children of specific item
workflowy list <item-id>

# List with custom depth
workflowy list <item-id> --depth 3

# List all descendants as flat list
workflowy list <item-id> --all

# Use backup file for fastest access
workflowy list --method=backup
```

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
- `--all`: Get/list all descendants (equivalent to `--depth=-1`)

### Access Methods and Configuration

#### Data Access Methods (`--method`)

The CLI supports three ways to access your Workflowy data. By default, it automatically chooses the most efficient method based on depth:

##### 1. GET API (`--method=get`)
**When used:**
- Default for depth 1-3
- Explicitly via `--method=get`

**Characteristics:**
- Multiple API calls (one per depth level)
- Best for specific items with shallow depth
- No caching
- Requires API key
- Subject to rate limits

**Example:**
```bash
workflowy get <item-id> --depth 2        # Smart default uses GET
workflowy get <item-id> --method=get     # Force GET API
```

##### 2. Export API (`--method=export`)
**When used:**
- Default for depth ≥4 or `--all`
- Explicitly via `--method=export`

**Characteristics:**
- Single API call fetches entire tree
- Best for deep fetches or complete tree access
- Cached locally for performance
- Requires API key
- More efficient for large trees

**Cache location:** `~/.workflowy/export-cache.json`

**Example:**
```bash
workflowy get --all                      # Smart default uses Export
workflowy get --method=export            # Force Export API
workflowy get --method=export --force-refresh  # Bypass cache
```

##### 3. Backup File (`--method=backup`)
**When used:**
- Explicitly via `--method=backup`
- Fastest option, works offline

**Characteristics:**
- Reads from local Workflowy backup file
- No API calls required
- No rate limits
- Works offline
- Data may be slightly stale (depends on backup frequency)

**Default backup location:** `~/Dropbox/Apps/Workflowy/Data/*.workflowy.backup` (most recent file)

**Example:**
```bash
# Use latest backup file
workflowy get --method=backup

# Use specific backup file
workflowy get --method=backup --backup-file=/path/to/backup.json
```

#### Configuration Flags

##### `--api-key-file`
Specifies the location of your API key file.

**Default:** `~/.workflowy/api.key`

**Setup:**
```bash
mkdir -p ~/.workflowy
echo "your-api-key-here" > ~/.workflowy/api.key
chmod 600 ~/.workflowy/api.key
```

**Usage:**
```bash
workflowy get --api-key-file=/path/to/custom-key.file
```

##### `--backup-file`
Specifies a specific backup file to use (only relevant with `--method=backup`).

**Default:** Latest file matching `~/Dropbox/Apps/Workflowy/Data/*.workflowy.backup`

**Usage:**
```bash
workflowy get --method=backup --backup-file=/custom/path/backup.json
```

##### `--force-refresh`
Bypasses the export cache and forces a fresh API call (only relevant with `--method=export`).

**Cache location:** `~/.workflowy/export-cache.json`

**Usage:**
```bash
workflowy get --all --force-refresh
```

**When to use:**
- After making changes via web/mobile app
- When you suspect stale data
- For critical operations requiring latest data

#### Performance Comparison

| Method | Speed | Freshness | Offline | Rate Limits | Best For |
|--------|-------|-----------|---------|-------------|----------|
| GET API | Medium | Real-time | No | Yes | Specific items, shallow depth |
| Export API | Fast* | Real-time | No | Yes | Full tree, deep fetches |
| Backup File | Fastest | Stale | Yes | No | Bulk operations, offline work |

\* After first fetch (cached)

#### Rate Limiting

The Workflowy API may enforce rate limits. If you encounter rate limit errors:
- Use `--method=backup` for bulk operations
- Use `--method=export` instead of multiple GET calls
- Space out API requests
- Check Workflowy's API documentation for current limits

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
