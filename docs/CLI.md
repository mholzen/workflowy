# Workflowy CLI Reference

Complete command-line reference for the Workflowy CLI tool.

## Table of Contents

- [Workflowy CLI Reference](#workflowy-cli-reference)
- [Installation](#installation)
  - [Via Homebrew (Recommended)](#via-homebrew-recommended)
  - [From Source](#from-source)
- [Setup](#setup)
  - [Get Your API Key](#get-your-api-key)
- [Global Options](#global-options)
- [Full and Short IDs](#full-and-short-ids)
- [Available Commands](#available-commands)
  - [search](#search)
  - [replace](#replace)
  - [report](#report)
    - [report count](#report-count)
    - [report children](#report-children)
    - [report created](#report-created)
    - [report modified](#report-modified)
  - [list](#list)
  - [get](#get)
  - [create](#create)
  - [update](#update)
  - [delete](#delete)
  - [complete](#complete)
  - [uncomplete](#uncomplete)
  - [targets](#targets)
  - [mcp](#mcp)
- [Data Access Methods](#data-access-methods)
- [Example Usage](#example-usage)
  - [Search Examples](#search-examples)
  - [Bulk Search and Replace Examples](#bulk-search-and-replace-examples)
  - [Usage Report Examples](#usage-report-examples)



## Installation

### Via Homebrew (Recommended)

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
2. Save your API key:

```bash
mkdir -p ~/.workflowy
echo "your-api-key-here" > ~/.workflowy/api.key
chmod 600 ~/.workflowy/api.key
```

## Global Options

These options apply to all commands:

| Option | Description | Default |
|--------|-------------|---------|
| `--format <list\|json\|markdown>` | Output format | `list` |
| `--log <level>` | Log level: debug, info, warn, error | `info` |
| `--log-file <path>` | Write logs to file instead of stderr | - |
| `--method <get\|export\|backup>` | Data access method | auto |
| `--api-key-file <path>` | API key file location | `~/.workflowy/api.key` |
| `--backup-file <path>` | Backup file path (for `--method=backup`) | auto-detected |
| `--force-refresh` | Bypass cache (for `--method=export`) | `false` |


## Full and Short IDs

Workflowy nodes are identified with a full length UUID which can be seen, for
example, using the JSON output format, or in the results of the search command:

```bash
$ workflowy get --format=json
{
  "nodes": [
    {
      "id": "3495d784-5db2-408f-8c4a-7ae1be810d4f",
      "name": "Plan A",
      ...

$ workflowy search Plan
- [**Plan** A](https://workflowy.com/#/3495d784-5db2-408f-8c4a-7ae1be810d4f)
```

The full UUIDs are difficult to obtain directly from within the Workflowy application.
You can find them by using this [bookmarklet](../bookmarklet/) or by
inspecting the DOM and looking for the `projectid` attribute as is described
[in this post](https://community.workflowy.com/t/rudimentary-api-try-using-it-and-tell-me-what-you-think/185).

However, Workflowy does provide an easy way to get the last 12 digits of the full UUID
via the "Copy Internal Link" command (meta+shift+L), which puts in the copy
buffer an URL of the form:

  `https://workflowy.com/#/7ae1be810d4f`

So, for convenience, you can use the last 12 characters of a node ID instead of
the full UUID:

```bash
# Full UUID
workflowy get 3495d784-5db2-408f-8c4a-7ae1be810d4f

# Short ID (last 12 characters)
workflowy get 7ae1be810d4f

# The Internal Link
workflowy get https://workflowy.com/#/7ae1be810d4f
```

Short IDs work with all commands and flags that accept node IDs (`--parent-id`,
`--item-id`, positional arguments), as well as MCP tools.

**Note:** This method is a bit slower because it requires searching through a
full export of your nodes.  Also, if multiple nodes share the same last 12
characters (extremely rare), an error will be returned listing all matches.

**ID Resolution Order:**
1. Target keys (e.g., `inbox`) are recognized first
2. 12-character hex strings are treated as short IDs
3. Everything else is treated as a full UUID


## Commands

### workflowy get

Retrieve items with their hierarchical tree structure.

```bash
# Get root items (default depth: 2)
workflowy get

# Get specific item
workflowy get <item-id>

# Get with custom depth
workflowy get <item-id> --depth 5

# Get all descendants
workflowy get <item-id> --all

# Force specific access method
workflowy get <item-id> --method=backup
```

**Options:**

| Option | Description | Default |
|--------|-------------|---------|
| `--depth <n>` | Recursion depth | `2` |
| `--all` | Get all descendants (`--depth=-1`) | `false` |
| `--include-empty-names` | Include items with empty names | `false` |

**Smart API Selection:**
- Depth 1-3: Uses GET API (efficient for shallow fetches)
- Depth 4+ or `--all`: Uses Export API (efficient for deep fetches)

---

### workflowy list

Retrieve items as a flat list without hierarchy.

```bash
# List direct children of root
workflowy list

# List children of specific item
workflowy list <item-id>

# List with custom depth
workflowy list <item-id> --depth 3

# List all descendants as JSON
workflowy list --all --format=json
```

**Options:** Same as `workflowy get`

---

### workflowy create

Create a new node.

```bash
# Create at root
workflowy create "New item"

# Create under specific parent
workflowy create --parent-id <parent-id> "New item"

# Create with note
workflowy create --parent-id <parent-id> --note "My note" "New item"

# Create at specific position
workflowy create --parent-id <parent-id> --position top "New item"
```

**Options:**

| Option | Description | Default |
|--------|-------------|---------|
| `--parent-id <id>` | Parent node ID or target name | root |
| `--note <text>` | Note content | - |
| `--position <top\|bottom>` | Position in parent | `bottom` |
| `--layout-mode <mode>` | Layout: bullets, todo, h1, h2, h3 | `bullets` |

---

### workflowy update

Update an existing node.

```bash
# Update node name
workflowy update <item-id> "new content"

# Update note only
workflowy update <item-id> --note "my note"

# Update multiple fields
workflowy update <item-id> --name "new title" --note "new note"
```

**Options:**

| Option | Description |
|--------|-------------|
| `--name <text>` | New node name |
| `--note <text>` | New note content |
| `--layout-mode <mode>` | Layout: bullets, todo, h1, h2, h3 |

---

### workflowy delete

Permanently delete a node and its children.

```bash
workflowy delete <item-id>

# JSON output
workflowy delete <item-id> --format json
```

---

### workflowy complete

Mark a node as complete.

```bash
workflowy complete <item-id>
```

---

### workflowy uncomplete

Mark a node as incomplete.

```bash
workflowy uncomplete <item-id>
```

---

### workflowy targets

List available shortcuts and system targets (like "inbox").

```bash
# List all targets
workflowy targets

# JSON output
workflowy targets --format json
```

**What are targets?**
- **System targets**: Built-in locations like `inbox`
- **Shortcuts**: User-defined shortcuts you create in Workflowy

Use target keys instead of UUIDs in commands:

```bash
# Create in inbox
workflowy create --parent-id=inbox "New task"

# Get contents of a shortcut
workflowy get home
```

---

### workflowy search

Search through nodes by name with text or regex patterns.

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

# JSON output with match positions
workflowy search "meeting" --format json
```

**Options:**

| Option | Description | Default |
|--------|-------------|---------|
| `-i` | Case-insensitive | `false` |
| `-E` | Treat pattern as regex | `false` |
| `--item-id <id>` | Limit search to subtree | root |

**Output:**
- `--format list`: Markdown with clickable links and **highlighted** matches
- `--format json`: JSON with match positions and metadata

---

### workflowy replace

Bulk find-and-replace text in node names using regex.

```bash
# Simple replacement
workflowy replace "old-text" "new-text"

# Case-insensitive
workflowy replace -i "todo" "DONE"

# Using capture groups
workflowy replace "task-([0-9]+)" 'issue_$1'
workflowy replace "(\w+) (\w+)" '$2 $1'

# Preview changes (dry-run)
workflowy replace --dry-run "pattern" "replacement"

# Interactive mode
workflowy replace --interactive "pattern" "replacement"

# Limit to subtree
workflowy replace --parent-id abc-123-def "pattern" "replacement"

# Limit depth
workflowy replace --parent-id abc-123-def --depth 3 "pattern" "replacement"
```

**Options:**

| Option | Description | Default |
|--------|-------------|---------|
| `-i` | Case-insensitive | `false` |
| `--dry-run` | Preview without applying | `false` |
| `--interactive` | Confirm each replacement | `false` |
| `--parent-id <id>` | Limit to subtree | root |
| `--depth <n>` | Traversal depth (-1 unlimited) | `-1` |

**Substitution syntax:** `$1`, `$2` or `${1}`, `${2}` for capture groups.

---

## Report Commands

All report commands support these upload options:

| Option | Description | Default |
|--------|-------------|---------|
| `--upload` | Upload to Workflowy instead of printing | `false` |
| `--parent-id <id>` | Parent for uploaded report | root |
| `--position <top\|bottom>` | Position in parent | `top` |

### workflowy report count

Rank nodes by total descendant count.

```bash
# Default report
workflowy report count

# Custom threshold (show nodes with >5% of total)
workflowy report count --threshold 0.05

# Upload to Workflowy
workflowy report count --upload --parent-id xxx-yyy-zzz
```

**Options:**

| Option | Description | Default |
|--------|-------------|---------|
| `--threshold <ratio>` | Minimum ratio to display (0.0-1.0) | `0.01` |

**Example output:**

```
# Descendant Count Report

Root: Root
Threshold: 1.00%
Total descendants: 43

- [Root](https://workflowy.com/#/...) (100.0%, 43 descendants)
  - [Projects](https://workflowy.com/#/...) (72.1%, 31 descendants)
    - [Project C](https://workflowy.com/#/...) (34.9%, 15 descendants)
```

---

### workflowy report children

Rank nodes by immediate children count.

```bash
# Top 20 (default)
workflowy report children

# Top 10
workflowy report children --top-n 10
```

**Options:**

| Option | Description | Default |
|--------|-------------|---------|
| `--top-n <n>` | Number of results | `20` |

---

### workflowy report created

Find oldest nodes by creation date.

```bash
workflowy report created --top-n 20
```

---

### workflowy report modified

Find stale nodes by modification date.

```bash
workflowy report modified --top-n 20
```

---

## Data Access Methods

### GET API (`--method=get`)

- **When used**: Default for depth 1-3
- **Characteristics**: Multiple API calls, real-time data, subject to rate limits
- **Best for**: Specific items with shallow depth

### Export API (`--method=export`)

- **When used**: Default for depth ≥4 or `--all`
- **Characteristics**: Single API call, cached locally
- **Cache location**: `~/.workflowy/export-cache.json`
- **Best for**: Full tree access, deep fetches

```bash
# Bypass cache
workflowy get --method=export --force-refresh
```

### Backup File (`--method=backup`)

- **When used**: Explicitly, or as fallback if no API key
- **Characteristics**: Reads local backup, fastest, works offline
- **Requirements**: Enable "Auto-Backup to Dropbox" in Workflowy settings
- **Default location**: `~/Dropbox/Apps/Workflowy/Data/*.workflowy.backup`

```bash
# Use latest backup
workflowy get --method=backup

# Use specific file
workflowy get --method=backup --backup-file=/path/to/backup.json
```

### Performance Comparison

| Method | Speed | Freshness | Offline | Rate Limits |
|--------|-------|-----------|---------|-------------|
| GET API | Medium | Real-time | No | Yes |
| Export API | Fast* | Real-time | No | Yes |
| Backup File | Fastest | Stale | Yes | No |

*After first fetch (cached)

---

## Examples

### Morning Review

```bash
# Find stale items
workflowy report modified --top-n 20

# Find items needing attention
workflowy search -i "todo"
```

### Weekly Cleanup

```bash
# Find oversized nodes
workflowy report count --threshold 0.05

# Find nodes with too many children
workflowy report children --top-n 10
```

### Project Migrations

```bash
# Preview rename
workflowy replace --dry-run "v1" "v2"

# Apply with confirmation
workflowy replace --interactive "v1" "v2"

# Bulk rename in specific folder
workflowy replace --parent-id project-id "OLD-" "NEW-"
```

### Backup Workflows

```bash
# Fast offline search
workflowy search -i "meeting" --method=backup

# Generate report from backup
workflowy report count --method=backup
```

---

## Troubleshooting

### Rate Limiting

If you encounter rate limit errors:
- Use `--method=backup` for bulk operations
- Use `--method=export` instead of multiple GET calls
- Space out API requests

### Stale Data

If data seems outdated:
- Use `--force-refresh` with export method
- Check Dropbox sync status for backup method

### API Key Issues

```bash
# Verify key file exists and has correct permissions
ls -la ~/.workflowy/api.key

# Should show: -rw------- (600 permissions)
```
