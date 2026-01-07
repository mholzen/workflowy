# Workflowy MCP Server & CLI

A feature-rich **Model Context Protocol (MCP) server** and **Command Line Interface (CLI)** for [Workflowy](https://workflowy.com) written in Go. Connect your AI assistant (Claude, ChatGPT, etc.) to your Workflowy data or run commands from a terminal emulator or script, including **search**, **bulk replace**, **usage reports**, and **offline access** capabilities.

[![Go](https://img.shields.io/badge/Go-1.24.3+-00ADD8?logo=go&logoColor=white)](https://go.dev/) [![Homebrew](https://img.shields.io/badge/Homebrew-Available-FBB040?logo=homebrew&logoColor=white)](https://github.com/mholzen/workflowy) [![MCP Compatible](https://img.shields.io/badge/MCP-Compatible-8A2BE2)](https://modelcontextprotocol.io) [![MIT License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)


<a href="https://glama.ai/mcp/servers/@mholzen/workflowy">
  <img width="380" height=“200" src="https://glama.ai/mcp/servers/@mholzen/workflowy/badge" />
</a>

## Why This Workflowy MCP Server?

 - Full-text search with regex
 - Bulk search & replace
 - Usage reports (stale nodes, size analysis)
 - Offline mode via backup files
 - CLI + MCP server in one tool
 - Caching for performance
 - Homebrew installation
 - Basic CRUD operations
 - Using short IDs (Copy Internal Link)

## Quick Start

### Install via Homebrew

```bash
brew install mholzen/workflowy/workflowy-cli
```

### Configure Your API Key

```bash
mkdir -p ~/.workflowy
echo "your-api-key-here" > ~/.workflowy/api.key
chmod 600 ~/.workflowy/api.key
```

Get your API key at https://workflowy.com/api-key/

### Run Your First Command

```bash
# Generate a report showing where most of your nodes are
workflowy report count | pbcopy   # paste directly into Workflowy!
```

Use `pbcopy` on macOS, `clip` on Windows, `wl-copy` on Linux, or `xclip` for X11 systems.


## Use with Claude Desktop (MCP Server)

Add to your Claude Desktop configuration:

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

Switch `--expose=all` to `--expose=read` if you want to allow read tools only.

Restart Claude Desktop and start asking Claude to work with your Workflowy!

## MCP Tools for AI Assistants

### Read Tools (Safe)
| Tool | Description |
|------|-------------|
| `workflowy_get` | Get a node and its descendants as a tree |
| `workflowy_list` | List descendants as a flat list |
| `workflowy_search` | Search nodes by text or regex |
| `workflowy_targets` | List shortcuts and system targets (inbox, etc.) |
| `workflowy_report_count` | Find where most of your content lives |
| `workflowy_report_children` | Find nodes with many children |
| `workflowy_report_created` | Find oldest nodes |
| `workflowy_report_modified` | Find stale, unmodified nodes |

### Write Tools
| Tool | Description |
|------|-------------|
| `workflowy_create` | Create new nodes |
| `workflowy_update` | Update node content |
| `workflowy_delete` | Delete nodes |
| `workflowy_complete` | Mark nodes complete |
| `workflowy_uncomplete` | Mark nodes incomplete |
| `workflowy_replace` | Bulk find-and-replace with regex |

## CLI Features

### Search Your Entire Outline

```bash
# Find all TODOs (case-insensitive)
workflowy search -i "foobar"

# Regex search for dates
workflowy search -E "<time.*>"

# Search within a specific subtree (using Internal Link)
workflowy search "bug" --item-id https://workflowy.com/#/1bdae4aecf00
```

### Bulk Search and Replace

```bash
# Preview changes first (dry run)
workflowy replace --dry-run "foo" "bar"

# Interactive confirmation
workflowy replace --interactive "foo" "bar"

# Use regex capture groups
workflowy replace "TASK-([0-9]+)" 'ISSUE-$1'
```

### Some Common CRUD Operations

```bash
# Add a task to your inbox
workflowy create "Buy groceries" --parent-id=inbox

# Change the name of an item
workflowy update xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx --name "Project Plan v2"

# Mark a node as complete, using a short ID
workflowy complete https://workflowy.com/#/xxxxxxxxxxxx
```


### Usage Reports

```bash
# Where is most of my content?
workflowy report count --threshold 0.01

# Which nodes have the most children?
workflowy report children --top-n 20

# Find stale content (oldest modified)
workflowy report modified --top-n 50
```

## Data Access Methods

Choose the best method for your use case:

| Method | Speed | Freshness | Offline | Best For |
|--------|-------|-----------|---------|----------|
| `--method=get` | Medium | Real-time | No | Specific items |
| `--method=export` | Fast* | 1 min worst case (due to rate limiting) | No | Full tree access |
| `--method=backup` | Fastest | Stale | **Yes** | Bulk operations |

*Cached after first fetch

### Offline Mode with Dropbox Backup

Enable Workflowy's Dropbox backup and access your data offline:

```bash
workflowy get --method=backup
workflowy search -i "project" --method=backup
```

## Installation Options

### Homebrew (Recommended)

```bash
brew install mholzen/workflowy/workflowy-cli
```

### From Source

```bash
git clone https://github.com/mholzen/workflowy.git
cd workflowy
go build ./cmd/workflowy
```

## Documentation

- [Full CLI Reference](docs/CLI.md)
- [MCP Server Guide](docs/MCP.md)
- [API Reference](https://workflowy.com/api-reference/)
- [Changelog](CHANGELOG.md)

## Examples

### AI Assistant Workflows

Ask Claude:
- "Search my Workflowy for all items containing 'meeting notes'"
- "Show me nodes I haven't touched in 6 months"
- "Replace all 'v1' with 'v2' in my Project A folder"
- "What's taking up the most space in my outline?"

### CLI Workflows

```bash
# Morning review: find stale items
workflowy report modified --top-n 20

# Weekly cleanup: find oversized nodes
workflowy report count --threshold 0.05

# Bulk rename: update project prefix
workflowy replace "OLD-" "NEW-" --parent-id projects-folder-id
```

## Contributing

Contributions welcome! See the [Contributing Guide](CONTRIBUTING.md).

```bash
# Development setup
git clone https://github.com/mholzen/workflowy.git
cd workflowy
go test ./...
```

## License

MIT — see [LICENSE](LICENSE)
