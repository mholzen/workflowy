# Workflowy MCP Server Guide

Connect AI assistants like Claude, ChatGPT, and other MCP-compatible clients to your Workflowy data.

## Table of Contents

- [What is MCP?](#what-is-mcp)
- [Quick Setup](#quick-setup)
- [Client Configuration](#client-configuration)
  - [Claude Desktop](#claude-desktop)
- [Workflowy MCP Tools](#workflowy-mcp-tools)
  - [workflowy_get](#workflowy_get)
  - [workflowy_list](#workflowy_list)
  - [workflowy_search](#workflowy_search)
  - [workflowy_targets](#workflowy_targets)
  - [workflowy_create](#workflowy_create)
  - [workflowy_update](#workflowy_update)
  - [workflowy_delete](#workflowy_delete)
  - [workflowy_complete](#workflowy_complete)
  - [workflowy_uncomplete](#workflowy_uncomplete)
  - [workflowy_replace](#workflowy_replace)
  - [workflowy_report_count](#workflowy_report_count)
  - [workflowy_report_children](#workflowy_report_children)
  - [workflowy_report_created](#workflowy_report_created)
  - [workflowy_report_modified](#workflowy_report_modified)
- [Exposure Modes](#exposure-modes)
- [Example Conversations](#example-conversations)
  - [Finding Information](#finding-information)
  - [Creating Content](#creating-content)
  - [Analysis](#analysis)
  - [Bulk Operations](#bulk-operations)
  - [Finding Stale Content](#finding-stale-content)
- [Logging and Debugging](#logging-and-debugging)
- [Security Considerations](#security-considerations)
- [Troubleshooting](#troubleshooting)



## What is MCP?

The [Model Context Protocol (MCP)](https://modelcontextprotocol.io) is an open standard that allows AI assistants to interact with external tools and data sources. This Workflowy MCP server exposes your Workflowy outlines to AI assistants, enabling them to search, read, create, and modify your notes.

## Quick Setup

### 1. Install the CLI

```bash
brew install mholzen/workflowy/workflowy-cli
```

Or [build from source](CLI.md#from-source).

### 2. Configure Your API Key

```bash
mkdir -p ~/.workflowy
echo "your-api-key-here" > ~/.workflowy/api.key
chmod 600 ~/.workflowy/api.key
```

Get your API key at https://workflowy.com/api-key/

### 3. Configure Your AI Client

See [Client Configuration](#client-configuration) below.

---

## Client Configuration

### Claude Desktop

Add to your Claude Desktop configuration file:

**macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
**Windows**: `%APPDATA%\Claude\claude_desktop_config.json`

#### Full Access (Read + Write)

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

#### Read-Only Access (Safer)

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

After updating the configuration, **restart Claude Desktop**.

### Other MCP Clients

The server uses stdio transport. Start it with:

```bash
workflowy mcp --expose=all
```

Configure your MCP client to run this command and communicate via stdin/stdout.

---

## Available Tools

### Read Tools

These tools are enabled by default (`--expose=read`).

#### workflowy_get

Get a node and its descendants as a tree structure.

**Parameters:**
| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `item_id` | string | Node ID or target name | root |
| `depth` | number | Recursion depth (-1 for all) | `2` |
| `include_empty_names` | boolean | Include empty-named items | `false` |

**Example prompt:** "Show me the contents of my Projects folder"

---

#### workflowy_list

List descendants as a flat list.

**Parameters:**
| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `item_id` | string | Node ID or target name | root |
| `depth` | number | Recursion depth (-1 for all) | `2` |
| `include_empty_names` | boolean | Include empty-named items | `false` |

**Example prompt:** "List all items in my inbox"

---

#### workflowy_search

Search nodes by text or regex pattern.

**Parameters:**
| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `pattern` | string | Search text or regex | required |
| `item_id` | string | Limit to subtree | root |
| `regexp` | boolean | Treat as regex | `false` |
| `ignore_case` | boolean | Case-insensitive | `false` |

**Example prompts:**
- "Search for all items containing 'meeting'"
- "Find items matching the pattern TODO or FIXME"
- "Search for dates in my notes"

---

#### workflowy_targets

List available shortcuts and system targets.

**Parameters:** None

**Example prompt:** "What shortcuts do I have in Workflowy?"

---

#### workflowy_report_count

Generate a descendant count report showing where most content lives.

**Parameters:**
| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `item_id` | string | Root node for report | root |
| `threshold` | number | Minimum ratio (0.0-1.0) | `0.01` |
| `preserve_tags` | boolean | Keep HTML tags | `false` |

**Example prompt:** "Where is most of my content in Workflowy?"

---

#### workflowy_report_children

Rank nodes by immediate children count.

**Parameters:**
| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `item_id` | string | Root node for report | root |
| `top_n` | number | Number of results | `20` |
| `preserve_tags` | boolean | Keep HTML tags | `false` |

**Example prompt:** "Which nodes have the most children?"

---

#### workflowy_report_created

Rank nodes by creation date (oldest first).

**Parameters:**
| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `item_id` | string | Root node for report | root |
| `top_n` | number | Number of results | `20` |
| `preserve_tags` | boolean | Keep HTML tags | `false` |

**Example prompt:** "What are my oldest notes?"

---

#### workflowy_report_modified

Rank nodes by modification date (oldest first).

**Parameters:**
| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `item_id` | string | Root node for report | root |
| `top_n` | number | Number of results | `20` |
| `preserve_tags` | boolean | Keep HTML tags | `false` |

**Example prompt:** "Find notes I haven't touched in a while"

---

### Write Tools

These tools require `--expose=write` or `--expose=all`.

#### workflowy_create

Create a new node.

**Parameters:**
| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `name` | string | Node content | required |
| `parent_id` | string | Parent node ID or target | root |
| `note` | string | Note content | - |
| `position` | string | `top` or `bottom` | `bottom` |
| `layout_mode` | string | bullets, todo, h1, h2, h3 | `bullets` |

**Example prompts:**
- "Create a new item called 'Buy groceries' in my inbox"
- "Add a TODO item under my Projects folder"

---

#### workflowy_update

Update an existing node.

**Parameters:**
| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `item_id` | string | Node ID to update | required |
| `name` | string | New name | - |
| `note` | string | New note | - |
| `layout_mode` | string | bullets, todo, h1, h2, h3 | - |

**Example prompt:** "Update the note on that item to include today's date"

---

#### workflowy_delete

Delete a node and its children.

**Parameters:**
| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `item_id` | string | Node ID to delete | required |

**Example prompt:** "Delete that completed task"

---

#### workflowy_complete

Mark a node as complete.

**Parameters:**
| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `item_id` | string | Node ID to complete | required |

**Example prompt:** "Mark that task as done"

---

#### workflowy_uncomplete

Mark a node as incomplete.

**Parameters:**
| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `item_id` | string | Node ID to uncomplete | required |

**Example prompt:** "Uncheck that task"

---

#### workflowy_replace

Bulk find-and-replace text in node names.

**Parameters:**
| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `pattern` | string | Regex pattern to match | required |
| `substitution` | string | Replacement (supports $1, $2) | required |
| `parent_id` | string | Limit to subtree | root |
| `depth` | number | Traversal depth (-1 unlimited) | `-1` |
| `ignore_case` | boolean | Case-insensitive | `false` |
| `dry_run` | boolean | Preview without applying | `true` |

**Example prompts:**
- "Replace all occurrences of 'v1' with 'v2' in my project notes"
- "Rename TASK-xxx to ISSUE-xxx across my outline"

---

## Exposure Modes

Control which tools are available:

| Mode | Description | Command |
|------|-------------|---------|
| `read` | Read-only tools (default) | `workflowy mcp` |
| `write` | Write tools only | `workflowy mcp --expose=write` |
| `all` | All tools | `workflowy mcp --expose=all` |
| Custom | Specific tools | `workflowy mcp --expose=get,list,search` |

**Examples:**

```bash
# Safe read-only access
workflowy mcp

# Full access
workflowy mcp --expose=all

# Specific tools only
workflowy mcp --expose=get,list,search,create
```

---

## Example Conversations

### Finding Information

> **You:** Search my Workflowy for anything about "quarterly review"

> **Claude:** I found 5 items containing "quarterly review":
> 1. Q3 Quarterly Review - Notes (in Projects/Work)
> 2. Quarterly Review Template (in Templates)
> ...

### Creating Content

> **You:** Add a new task "Prepare presentation" to my inbox

> **Claude:** I've created "Prepare presentation" in your inbox.

### Analysis

> **You:** Which parts of my Workflowy have the most content?

> **Claude:** Here's your content distribution:
> - Projects (45% - 234 items)
>   - Work (28% - 145 items)
>   - Personal (17% - 89 items)
> - Archive (32% - 166 items)
> ...

### Bulk Operations

> **You:** Preview what would happen if I replaced "2024" with "2025" in my Goals folder

> **Claude:** Dry run complete. 12 items would be updated:
> - "2024 Goals" → "2025 Goals"
> - "Q1 2024 Objectives" → "Q1 2025 Objectives"
> ...
>
> Should I apply these changes?

### Finding Stale Content

> **You:** Show me notes I haven't modified in over 6 months

> **Claude:** Here are your 20 oldest unmodified items:
> 1. "Old project ideas" - last modified Jan 2024
> 2. "Meeting notes archive" - last modified Feb 2024
> ...

---

## Logging and Debugging

### Enable Logging

```bash
workflowy mcp --log-file=/tmp/workflowy-mcp.log
```

### View Logs

```bash
tail -f /tmp/workflowy-mcp.log
```

### Log Levels

```bash
workflowy mcp --log=debug --log-file=/tmp/workflowy-mcp.log
```

---

## Security Considerations

### API Key Protection

- Store your API key with restricted permissions (`chmod 600`)
- Never commit your API key to version control
- The MCP server reads the key from `~/.workflowy/api.key`

### Read-Only Mode

For safety, the default mode is read-only. Only enable write access when needed:

```json
{
  "mcpServers": {
    "workflowy": {
      "command": "workflowy",
      "args": ["mcp"]
    }
  }
}
```

### Limiting Scope

Use `--expose` to limit available tools:

```bash
# Only search and read
workflowy mcp --expose=get,list,search

# Only reports
workflowy mcp --expose=report_count,report_modified
```

---

## Troubleshooting

### "Tool not found" Error

Ensure you've enabled the required tools:

```bash
# For write operations
workflowy mcp --expose=all
```

### Connection Issues

1. Check the log file for errors
2. Verify your API key is valid
3. Restart Claude Desktop after config changes

### Stale Data

The MCP server uses the Export API with caching. To force fresh data:

```bash
workflowy mcp --force-refresh
```

### Rate Limiting

If you see rate limit errors:
- Space out requests
- Use backup method for bulk operations:

```bash
workflowy mcp --method=backup
```

---

## Best Practices

1. **Start read-only**: Use default settings until you're comfortable
2. **Use dry-run**: Preview replace operations before applying
3. **Enable logging**: Always use `--log-file` for debugging
4. **Be specific**: Give Claude specific node IDs or target names when possible
5. **Review changes**: Ask Claude to show changes before confirming bulk operations
