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
  - [workflowy_move](#workflowy_move)
  - [workflowy_delete](#workflowy_delete)
  - [workflowy_complete](#workflowy_complete)
  - [workflowy_uncomplete](#workflowy_uncomplete)
  - [workflowy_replace](#workflowy_replace)
  - [workflowy_transform](#workflowy_transform)
  - [workflowy_report_count](#workflowy_report_count)
  - [workflowy_report_children](#workflowy_report_children)
  - [workflowy_report_created](#workflowy_report_created)
  - [workflowy_report_modified](#workflowy_report_modified)
  - [workflowy_report_mirrors](#workflowy_report_mirrors)
- [Exposure Modes](#exposure-modes)
- [Sandboxed Access](#sandboxed-access)
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
| `include_ancestors` | boolean | Wrap result in ancestor path from root to target node (requires export or backup method) | `false` |
| `ancestor_depth` | number | Include N levels of ancestors (-1 for all, 0 for none; requires export or backup method) | `0` |
| `to_ancestor` | string | Include ancestors up to and including this node ID (requires export or backup method) | - |
| `method` | string | Access method: get, export, or backup | auto |

Only one ancestor parameter may be used at a time. When used, `include_ancestors` is equivalent to `ancestor_depth=-1`.

**Example prompts:**
- "Show me the contents of my Projects folder"
- "Get the Meeting Notes node and show me where it lives in my hierarchy"
- "Show me the Tasks node with 2 levels of parent context"

---

#### workflowy_list

List descendants as a flat list.

**Parameters:**
| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `item_id` | string | Node ID or target name | root |
| `depth` | number | Recursion depth (-1 for all) | `2` |
| `include_empty_names` | boolean | Include empty-named items | `false` |
| `method` | string | Access method: get, export, or backup | auto |

**Example prompt:** "List all items in my inbox"

---

#### workflowy_search

Search nodes by text or regex pattern. Completed nodes (and their descendants) are excluded by default.

**Parameters:**
| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `pattern` | string | Search text or regex | required |
| `id` | string | Limit to subtree | root |
| `regexp` | boolean | Treat as regex | `false` |
| `ignore_case` | boolean | Case-insensitive | `false` |
| `include_completed` | boolean | Include completed nodes in results | `false` |
| `group_by` | string | Group results: `parent`, `path`, `tree`, `modified.<unit>`, `created.<unit>` | - |
| `path_max_length` | number | Max chars per path segment when using `group_by=path` | `20` |
| `order_by` | string | Sort by: `match`, `parent`, `path`, `modified`, `created` (prefix `+`/`-` for asc/desc) | - |
| `method` | string | Access method: get, export, or backup | export |

**Group-by units:** `year`, `month`, `day`, or a Go time format string.

**Example prompts:**
- "Search for all items containing 'meeting'"
- "Find items matching the pattern TODO or FIXME"
- "Search for dates in my notes, grouped by parent"
- "Find all TODOs grouped as a tree"

---

#### workflowy_targets

List available shortcuts and system targets. Also returns write restriction info if `--write-root-id` is set.

**Parameters:**
| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `method` | string | Access method for resolving root names | auto |

**Returns:**
- `targets`: Array of available shortcuts and system targets
- `write_root`: (optional) Object with `id` and `name` of the write-restricted area

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
| `method` | string | Access method: get, export, or backup | export |

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
| `method` | string | Access method: get, export, or backup | export |

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
| `method` | string | Access method: get, export, or backup | export |

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
| `method` | string | Access method: get, export, or backup | export |

**Example prompt:** "Find notes I haven't touched in a while"

---

#### workflowy_report_mirrors

Rank nodes by mirror count (most mirrored first). Uses backup file as mirror data is only available there.

**Parameters:**
| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `top_n` | number | Number of results | `20` |
| `preserve_tags` | boolean | Keep HTML tags | `false` |

**Example prompt:** "Which nodes are mirrored the most?"

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
| `method` | string | Access method for validation (writes use API) | auto |

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
| `method` | string | Access method for validation (writes use API) | auto |

**Example prompt:** "Update the note on that item to include today's date"

---

#### workflowy_move

Move a node to a new parent.

**Parameters:**
| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `id` | string | Node ID to move | required |
| `parent_id` | string | Destination parent ID or target | required |
| `position` | string | `top` or `bottom` | `top` |
| `method` | string | Access method for validation (writes use API) | auto |

**Example prompts:**
- "Move that task to my inbox"
- "Move this item to the top of my Projects folder"

---

#### workflowy_delete

Delete a node and its children.

**Parameters:**
| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `id` | string | Node ID to delete | required |
| `method` | string | Access method for validation (writes use API) | auto |

**Example prompt:** "Delete that completed task"

---

#### workflowy_complete

Mark a node as complete.

**Parameters:**
| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `item_id` | string | Node ID to complete | required |
| `method` | string | Access method for validation (writes use API) | auto |

**Example prompt:** "Mark that task as done"

---

#### workflowy_uncomplete

Mark a node as incomplete.

**Parameters:**
| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `item_id` | string | Node ID to uncomplete | required |
| `method` | string | Access method for validation (writes use API) | auto |

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
| `method` | string | Access method for reading (writes use API) | export |

**Example prompts:**
- "Replace all occurrences of 'v1' with 'v2' in my project notes"
- "Rename TASK-xxx to ISSUE-xxx across my outline"

---

#### workflowy_transform

Transform node names and/or notes using built-in or shell transformations.

**Parameters:**
| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `id` | string | Node ID to transform | required |
| `transform_name` | string | Built-in: lowercase, uppercase, capitalize, title, trim, no-punctuation, no-whitespace, split, group | - |
| `exec` | string | Shell command (use `{}` for input) | - |
| `separator` | string | Separator for split transform | `,` |
| `regex` | boolean | Treat separator as regex pattern | `false` |
| `list` | boolean | Split by markdown list markers (-, *, +, 1., etc.) | `false` |
| `group_by` | string | For group: modified, created, modified.<unit>, created.<unit> | `created.day` |
| `order` | string | For group: +modified, -modified, +created, -created | `-modified` |
| `depth` | number | Traversal depth (-1 unlimited) | `-1` |
| `name` | boolean | Transform node names | `true` |
| `note` | boolean | Transform node notes | `false` |
| `dry_run` | boolean | Preview without applying | `true` |
| `as_child` | boolean | Insert result as child | `false` |
| `method` | string | Access method for reading (writes use API) | export |

**Example prompts:**
- "Convert this node to uppercase"
- "Split this comma-separated list into child nodes"
- "Split this markdown list into separate child nodes"
- "Group these items by the day they were modified"
- "Organize my inbox by creation date, oldest first"
- "Trim whitespace from all items in this folder"

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

## Access Method

The MCP server uses a **hybrid approach** for access methods:

1. **Server-wide default** (`--method` flag): Sets the default for all tools
2. **Per-tool override** (`method` parameter): Each tool call can override the default

This provides flexibility - set a sensible default at server startup, then override for specific operations as needed.

### Available Methods

| Method | Description | Speed | Freshness |
|--------|-------------|-------|-----------|
| `get` | Direct API calls | Slowest | Always fresh |
| `export` | Cached export API | Fast | Up to 1 minute stale |
| `backup` | Local backup file | Fastest | Depends on backup age |
| (auto) | `get` for depth 1-3, `export` for depth 4+ | Varies | Varies |

### Server-Wide Default

Set the default method when starting the MCP server:

```bash
# Auto-select based on depth (default)
workflowy mcp

# Always use fresh API data
workflowy mcp --method=get

# Always use cached export
workflowy mcp --method=export

# Work entirely offline from backup
workflowy mcp --method=backup
```

### Per-Tool Override

Every tool accepts a `method` parameter that overrides the server default:

```json
// Search using export (fresh data) even if server default is backup
{"pattern": "TODO", "method": "export"}

// Get a report from backup (fast) even if server default is get
{"id": "abc123", "method": "backup"}
```

### Practical Example

A common workflow is to start with backup for speed, then use export when fresh data is needed:

```bash
# Start server with backup as default (fast, offline)
workflowy mcp --method=backup
```

Then in your AI conversation:
1. **Search backup** - fast, but won't find recently created nodes
2. **Create a node** - always uses API
3. **Search with export** - finds the newly created node

```json
// Step 1: Search backup (won't find new nodes)
{"pattern": "foobar", "method": "backup"}  // → no results

// Step 2: Create node (always uses API)
{"name": "foobar"}  // → created

// Step 3: Search export (finds new node)
{"pattern": "foobar", "method": "export"}  // → found!
```

### When to Use Each Method

**`backup`:**
- Fastest access (no API calls)
- Works offline
- Required for mirror data
- Best for: bulk reports, offline work, mirror analysis

**`export`:**
- Fast cached access
- Consistent data across operations
- Rate-limited to 1 refresh/minute
- Best for: most read operations, searching

**`get`:**
- Always fresh data
- Slower (multiple API calls for deep trees)
- Best for: recently modified nodes, shallow reads

### Backup File

Use `--backup-file` to specify a custom backup file path:

```bash
workflowy mcp --method=backup --backup-file=/path/to/backup.json
```

By default, the latest backup from `~/Dropbox/Apps/Workflowy/Data` is used.

---

## Sandboxed Access

Use `--read-root-id` and/or `--write-root-id` to restrict operations to specific subtrees. This is ideal for giving AI assistants access to only a portion of your Workflowy.

### Read Restrictions (`--read-root-id`)

Restricts **all operations** (read and write) to a node and its descendants:

```json
{
  "mcpServers": {
    "workflowy": {
      "command": "workflowy",
      "args": ["mcp", "--expose=all", "--read-root-id=<project-id>"]
    }
  }
}
```

When `--read-root-id` is set:

1. **All tool descriptions** include the restriction note
2. **Read operations** (get, list, search, reports) are scoped to the subtree
3. **Write operations** are also restricted to the subtree
4. **Default scope**: bare requests (no ID specified) return the read-root subtree
5. **workflowy_targets** returns `read_root` info with the ID and name

### Write Restrictions (`--write-root-id`)

Restricts only **write operations** to a node and its descendants:

```json
{
  "mcpServers": {
    "workflowy": {
      "command": "workflowy",
      "args": ["mcp", "--expose=all", "--write-root-id=inbox"]
    }
  }
}
```

When `--write-root-id` is set:

1. **Write tool descriptions** include the restriction
2. **Create operations** default to the write-root as parent
3. **All write operations** (update, delete, move, complete, etc.) are restricted to descendants
4. **workflowy_targets** returns `write_root` info

### Combining Both

When both flags are set, write operations must satisfy both constraints:

```json
{
  "mcpServers": {
    "workflowy": {
      "command": "workflowy",
      "args": ["mcp", "--expose=all", "--read-root-id=<project-id>", "--write-root-id=<project-inbox-id>"]
    }
  }
}
```

### Example

```bash
# Start MCP server with full sandbox
workflowy mcp --expose=all --read-root-id=<project-id>
```

The AI assistant will see tool descriptions like:
> "Get node and descendants (restricted to abc-123 and descendants)"

And `workflowy_targets` will return:
```json
{
  "targets": [...],
  "read_root": {
    "id": "abc-123-full-uuid",
    "name": "My Project"
  }
}
```

### Implementation Note

These restrictions are enforced by the MCP server, not by the Workflowy API itself. When required by the operation, the full data export is retrieved from Workflowy, then operations are validated and scoped locally before returning results. This means that for operations like search, the full tree is read from the API even though only results within the restricted subtree are provided to the agent.

### Create Default Parent

When `--read-root-id` is set (even without `--write-root-id`), create operations default to the read-root as the parent node. Priority for the default parent is:

1. Explicitly specified `parent_id`
2. `--write-root-id` (if set)
3. `--read-root-id` (if set)
4. Root ("None")

### Cache and Rate Limiting

The MCP server uses Workflowy's Export API with a local cache to validate access restrictions. The Export API is rate-limited to one request per minute, which affects how quickly newly created nodes become visible for validation.

**What happens with newly created nodes:**

When a node is created, it exists in Workflowy immediately but may not appear in the local export cache. If a subsequent operation (e.g., `workflowy_complete`) targets that node, the server:

1. Checks the cached tree — node not found
2. Forces a cache refresh from the Export API
3. If the refresh succeeds — retries validation with fresh data
4. If the refresh is rate-limited — returns stale cache data, and the node is still not found

When the cache refresh cannot find the node, the error message indicates this clearly:

```
complete denied: target <node-id> not found in cache; if newly created, retry in ~60s
```

**Workaround:** Wait approximately 60 seconds between creating a node and performing operations on it that require cache validation (complete, delete, update, move). Operations that don't require cache validation (like reading via `workflowy_get`) work immediately.

### Use Cases

- **Full Sandbox**: Use `--read-root-id` to completely isolate an AI to one subtree
- **Read All, Write Some**: Use `--write-root-id` alone to allow reading everything but writing only to a specific area
- **Project Isolation**: Restrict a script to only see and modify one project
- **Safe Experimentation**: Test operations without risking other data

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

At debug level, every Workflowy API request logs its HTTP method and path:

```text
DEBUG: http request (method='GET', path='/nodes/example')
```

HTTP debug logs never include the API key, authorization header, or request body.

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
