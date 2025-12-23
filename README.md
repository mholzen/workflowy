# Workflowy CLI and MCP Server

<a href="https://glama.ai/mcp/servers/@mholzen/workflowy">
  <img width="380" height=“200" src="https://glama.ai/mcp/servers/@mholzen/workflowy/badge" />
</a>

## Quick Start

1. [Get an API key](#get-your-api-key) or [enable backup to Dropbox](#3-backup-file---methodbackup)

2. Install
```bash
brew install mholzen/workflowy/workflowy-cli
```

3. Run the descendant count report, send the output to the clipboard and paste directly to Workflowy:
```bash
workflowy report count | pbcopy   # or clip, wl-copy or xclip
```
Use `pbcopy` on macOS, `clip` on Windows, `wl-copy` on Linux, or `xclip` for X11 systems.

It will produce a report such as:

```
# Descendant Count Report

Root: Root
Threshold: 1.00%
Total descendants: 43

- [Root](https://workflowy.com/#/...) (100.0%, 43 descendants)
  - [Projects](https://workflowy.com/#/...) (72.1%, 31 descendants)
    - [Project C](https://workflowy.com/#/...) (34.9%, 15 descendants)
      - [issues](https://workflowy.com/#/...) (23.3%, 10 descendants)
    - [Project B](https://workflowy.com/#/...) (20.9%, 9 descendants)
      - [issues](https://workflowy.com/#/...) (9.3%, 4 descendants)
    - [Project A](https://workflowy.com/#/...) (14.0%, 6 descendants)
  - [Inbox](https://workflowy.com/#/...) (14.0%, 6 descendants)
  - [People](https://workflowy.com/#/...) (11.6%, 5 descendants)
  ```

to understand where the majority of your nodes are located.


## Table of Contents

- [Features](#features)
- [Installation](#installation)
  - [Via Homebrew](#via-homebrew)
  - [From Source](#from-source)
- [Setup](#setup)
  - [Get Your API Key](#get-your-api-key)
- [Usage](#usage)
  - [Basic Commands](#basic-commands)
    - [List Targets](#list-targets)
    - [Search For Items](#search-for-items)
    - [Search and Replace](#search-and-replace)
  - [Usage Reports](#usage-reports)
  - [Global Options](#global-options)
  - [Command-Specific Options](#command-specific-options)
  - [Access Methods and Configuration](#access-methods-and-configuration)
    - [Data Access Methods](#data-access-methods---method)
    - [Configuration Flags](#configuration-flags)
    - [Performance Comparison](#performance-comparison)
    - [Rate Limiting](#rate-limiting)
- [MCP Server](#mcp-server)
  - [Configuring with Claude Desktop](#configuring-with-claude-desktop)
  - [Available Tools](#available-tools)
- [Examples](#examples)


A command-line interface for interacting with Workflowy, including fetching, creating, and
updating nodes, searching through content, and generating usage reports.

## Features

- **Node Operations**: Get, List, Create, Update, Complete, Uncomplete, and Delete to operate on nodes.
- **Targets**: List all available shortcuts and system targets (like "inbox") for use in commands.
- **Search**: Search through all nodes with text or regex patterns, with case-sensitive/insensitive options and highlighted results.
- **Search and Replace**: Bulk find-and-replace across node names using regex with capture group support, interactive mode, and dry-run preview.
- **Usage Reports**: Understand where the majority of your nodes are stored, which nodes have many children or which ones are possibly stale:
  - Rank nodes by the count of descendants, with a configurable threshold to the total number of nodes
  - Rank nodes by count of immediate children
  - Rank nodes by oldest created or modified dates
- **Format**: produces Markdown lists or JSON
- **Report Upload**: Upload usage reports using the API or paste
  the markdown output into Workflowy
- **Backup File Support**: Operates on local backup files for faster operations
- **Local Caching**: Caches API responses for improved performance


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
2. Configure your API key using one of these methods:

**Option A: Environment variable (recommended for CI/scripts)**
```bash
export WORKFLOWY_API_KEY="your-api-key-here"
```

**Option B: Key file (recommended for personal use)**
```bash
mkdir -p ~/.workflowy
echo "your-api-key-here" > ~/.workflowy/api.key
chmod 600 ~/.workflowy/api.key
```

**Precedence order:**
1. `--api-key-file` flag (if explicitly provided)
2. `WORKFLOWY_API_KEY` environment variable
3. Default file (`~/.workflowy/api.key`)

## Usage

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

# List all descendants a list of JSON nodes
workflowy list --all --format=json

# Use backup file for fastest access
workflowy list --method=backup
```

#### Update A Node

```bash
# Update node content
workflowy update <item-id> "new content"

# Update note only
workflowy update <item-id> --note "my note"

# Update multiple fields
workflowy update <item-id> --name "new title" --note "new note"
```

#### Complete And Uncomplete Nodes

```bash
# Mark a node as complete
workflowy complete <item-id>

# Mark a node as uncomplete
workflowy uncomplete <item-id>

# JSON output
workflowy complete <item-id> --format json
```

#### Delete A Node

```bash
# Permanently delete a node
workflowy delete <item-id>

# JSON output
workflowy delete <item-id> --format json
```

#### List Targets

List all available targets (shortcuts and system targets like "inbox") that can be used as IDs in other commands:

```bash
# List all targets
workflowy targets

# JSON output with full details
workflowy targets --format json

# Markdown output
workflowy targets --format markdown
```

**What are targets?**
- **System targets**: Built-in locations like `inbox`
- **Shortcuts**: User-defined shortcuts you create in Workflowy

**Use targets as IDs**: You can use target keys (like `inbox`) instead of UUIDs in commands that accept parent IDs or item IDs.

**Example:**
```bash
# Create a node in your inbox using the "inbox" target
workflowy create --parent-id=inbox "New task"

# Get contents of a shortcut using its key
workflowy get home
```

#### Search For Items

Search through your Workflowy items by name with text or regex patterns:

```bash
# Basic search (case-sensitive)
workflowy search "project"

# Case-insensitive search
workflowy search -i "PROJECT"

# Regex search
workflowy search -E "test.*ing"

# Regex with case-insensitive
workflowy search -iE "bug.*fix"

# Search within specific subtree
workflowy search "todo" --item-id abc-123-def

# Output as JSON with match positions
workflowy search "meeting" --format json
```

**Search Features:**
- Text or regex pattern matching
- Case-sensitive by default, use `-i` for case-insensitive
- Highlights all matches with **bold** in markdown output
- Returns markdown links to matching items
- Search entire tree or specific subtree with `--item-id`
- JSON output includes match positions for programmatic use

**Output Formats:**
- `--format list` (default): Markdown list with clickable links and **highlighted** matches
- `--format json`: JSON array with match positions and metadata
- `—-format markdown`: **Experimental** formatter that attempts to turn a tree of
  nodes without any `layoutMode` information into a Markdown document,
  translating parent nodes in header nodes, joining paragraphs, capitalizing and
  joining paragraphs, and detecting list vs paragraph items using a heuristic.

#### Search and Replace

Bulk find-and-replace text in node names using regular expressions:

```bash
# Simple text replacement
workflowy replace "old-text" "new-text"

# Case-insensitive replacement
workflowy replace -i "todo" "DONE"

# Using capture groups (use ${N} when followed by alphanumerics)
workflowy replace "task-([0-9]+)" 'issue_$1'        # task-123 → issue_123
workflowy replace "(\w+) (\w+)" '$2 $1'             # "hello world" → "world hello"

# Preview changes without applying (dry-run)
workflowy replace --dry-run "pattern" "replacement"

# Interactive mode - confirm each replacement
workflowy replace --interactive "pattern" "replacement"

# Limit to a specific subtree
workflowy replace --parent-id abc-123-def "pattern" "replacement"

# Limit depth of traversal
workflowy replace --parent-id abc-123-def --depth 3 "pattern" "replacement"

# JSON output
workflowy replace --format json "pattern" "replacement"
```

**Replace Features:**
- Regular expression patterns with capture group support
- Substitution syntax: `$1`, `$2` (or `${1}`, `${2}` when followed by alphanumerics)
- `--dry-run`: Preview all changes without modifying any nodes
- `--interactive`: Prompt for confirmation before each replacement (y/N/q to quit)
- `--parent-id`: Limit replacements to a specific subtree
- `--depth`: Control how deep to traverse (-1 for unlimited, default)
- `-i`: Case-insensitive pattern matching

**Output Formats:**
- `--format list` (default): Shows before/after for each match with node IDs
- `--format json`: JSON array with old_name, new_name, applied status, and URLs

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

- `--format <list|json|markdown>`: Output format: list, json, or markdown (default: "list")
- `--log <level>`: Log level: debug, info, warn, error (default: info)
- `--log-file <path>`: Write logs to a file instead of stderr (useful for MCP server mode)

### Command-Specific Options

#### Get and List Commands

- `--depth <n>`: Recursion depth for operations (default: 2)
- `--all`: Get/list all descendants (equivalent to `--depth=-1`)
- `--include-empty-names`: Include items with empty names

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

The CLI can read from a Workflowy backup file, stored to Dropbox and sync'ed to
your filesystem locally.  Enable "Auto-Backup my Workflowy to Dropbox" in
[Settings](https://workflowy.com/learn/account/) and ensure your Dropbox folder
`/Apps/Workflowy/Data/` is synced locally to `~/Dropbox/Apps/Workflowy/Data/`.

**When used:**
- Explicitly via `--method=backup`
- As a fallback, if no API key is found

**Characteristics:**
- Reads from local Workflowy backup file
- Fastest option
- Works offline
- No API calls required
- No rate limits
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

##### API Key Configuration

The CLI supports multiple ways to provide your API key:

| Method | Precedence | Best For |
|--------|------------|----------|
| `--api-key-file` flag | 1 (highest) | One-off commands with different key |
| `WORKFLOWY_API_KEY` env var | 2 | CI/CD, scripts, containers |
| Default file | 3 (lowest) | Personal workstation use |

**Environment variable:**
```bash
export WORKFLOWY_API_KEY="your-api-key-here"
workflowy list
```

**Default file location:** `~/.workflowy/api.key`

```bash
mkdir -p ~/.workflowy
echo "your-api-key-here" > ~/.workflowy/api.key
chmod 600 ~/.workflowy/api.key
```

**Explicit file path:**
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

## MCP Server

The Workflowy CLI includes an MCP (Model Context Protocol) server that enables AI assistants like Claude to interact with your Workflowy data.

### Configuring with Claude Desktop

Add the following to your Claude Desktop configuration file:

**macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
**Windows**: `%APPDATA%\Claude\claude_desktop_config.json`

```json
{
  "mcpServers": {
    "workflowy": {
      "command": "workflowy",
      "args": ["mcp", "--expose=all", "--log-file=/tmp/workflowy-mcp.log"]
    }
  }
}
```

For read-only access (safer):
```json
{
  "mcpServers": {
    "workflowy": {
      "command": "workflowy",
      "args": ["mcp", "--log-file=/tmp/workflowy-mcp.log"]
    }
  }
}
```

**Using environment variable for API key:**
```json
{
  "mcpServers": {
    "workflowy": {
      "command": "workflowy",
      "args": ["mcp", "--expose=all", "--log-file=/tmp/workflowy-mcp.log"],
      "env": {
        "WORKFLOWY_API_KEY": "your-api-key-here"
      }
    }
  }
}
```

After updating the configuration, restart Claude Desktop.

### Available Tools

The MCP server exposes Workflowy operations as tools that Claude can use:

**Read Tools** (default, `--expose=read`):
- `workflowy_get` - Get a node and its descendants
- `workflowy_list` - List descendants as a flat list
- `workflowy_search` - Search nodes by text or regex
- `workflowy_targets` - List available shortcuts and system targets
- `workflowy_report_count` - Generate descendant count report
- `workflowy_report_children` - Rank nodes by children count
- `workflowy_report_created` - Rank nodes by creation date
- `workflowy_report_modified` - Rank nodes by modification date

**Write Tools** (`--expose=write` or `--expose=all`):
- `workflowy_create` - Create a new node
- `workflowy_update` - Update an existing node
- `workflowy_delete` - Delete a node
- `workflowy_complete` - Mark a node as complete
- `workflowy_uncomplete` - Mark a node as uncomplete
- `workflowy_replace` - Search and replace text in node names

**Examples:**
```bash
# Start with read-only tools (safe default)
workflowy mcp

# Enable all tools including write operations
workflowy mcp --expose=all

# Enable specific tool groups
workflowy mcp --expose=read,write

# Enable only specific tools
workflowy mcp --expose=get,list,search

# With logging to file (recommended for debugging)
workflowy mcp --expose=all --log-file=/tmp/workflowy-mcp.log
```

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

### Search for specific content

```bash
# Find all TODO items (case-insensitive)
workflowy search -i "todo"

# Find items with dates matching a pattern
workflowy search -E "\d{4}-\d{2}-\d{2}"

# Search for bugs in a specific project subtree
workflowy search -i "bug" --item-id project-xyz-123
```

### Bulk search and replace

```bash
# Preview changes before applying
workflowy replace --dry-run "TODO" "DONE"

# Replace with confirmation for each match
workflowy replace --interactive "TODO" "DONE"

# Rename task prefixes using capture groups
workflowy replace "TASK-([0-9]+)" 'ISSUE-$1'

# Bulk rename within a specific project
workflowy replace --parent-id project-xyz-123 "v1" "v2"
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
│   ├── workflowy/       # Core Workflowy API client and tree utilities
│   ├── mcp/             # MCP server implementation
│   ├── search/          # Search functionality
│   ├── replace/         # Search and replace functionality
│   ├── reports/         # Report generation and upload
│   └── formatter/       # Output formatting
└── README.md
```

## License

MIT

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
