# Workflowy MCP Server

The `workflowy` CLI includes an MCP (Model Context Protocol) server that exposes Workflowy operations as MCP tools over **stdio** (so it can be used by MCP clients like Claude Desktop).

## Running the server

The MCP server is started via the `mcp` subcommand:

```bash
workflowy mcp
```

### Required authentication

The MCP server requires a Workflowy API key file (it will fail to start if the file can’t be read).

- **Default**: `~/.workflowy/api.key` (via the global `--api-key-file` flag)
- **Get an API key**: `https://workflowy.com/api-key/`

### Logging

Logging is controlled by the global flags:

- **`--log`**: `debug`, `info`, `warn`, `error`
- **`--log-file`**: write logs to a file (recommended when debugging MCP integrations)

Example:

```bash
workflowy mcp --log=debug --log-file=/tmp/workflowy-mcp.log
```

## Tool exposure (`--expose`)

By default the server only exposes **read-only** tools:

```bash
workflowy mcp --expose=read
```

You can enable additional tools using a comma-separated list:

- **Groups**: `read`, `write`, `all`
- **Individual tools**: either the full MCP tool name (e.g. `workflowy_get`) or the short alias (e.g. `get`)
- **Behavior**: case-insensitive, trimmed, deduplicated, preserved order

Examples:

```bash
workflowy mcp --expose=all
workflowy mcp --expose=read,write
workflowy mcp --expose=get,list,search
```

## Data sources, caching, and freshness

Some tools use the Workflowy Export API under the hood. Export results are cached for a short time to avoid rate limits.

- **Export cache path**: `~/.workflowy/export-cache.json`
- **Cache TTL**: 1 minute
- **Important**: the MCP server currently always calls export with `forceRefresh=false`, so there is **no MCP knob** to force-refresh exports.

### Which tools use export vs GET

- **`workflowy_get` / `workflowy_list`**:
  - Uses **GET API** when `depth` is `0..3`
  - Uses **Export API** when `depth` is `-1` or `>= 4`
- **Always uses Export API**: `workflowy_search`, all `workflowy_report_*` tools, `workflowy_replace`
- **Always uses GET API**: `workflowy_targets`, all write mutations (`create`, `update`, `delete`, `complete`, `uncomplete`)

## Tool reference

### Common conventions

- **Root**: most tools accept `"None"` as the root item ID.
- **IDs**: parameters named `item_id` / `parent_id` are passed directly to the Workflowy API. In many cases Workflowy accepts either a node ID or a target key (shortcut), depending on the endpoint.
- **Responses**: tools return JSON via MCP tool results. Errors are returned as MCP tool errors.

### Read tools (default)

#### `workflowy_get`

Get a node and optional descendants.

- **Inputs**
  - **`item_id`** (string, default `"None"`): node ID (or `"None"` for root)
  - **`depth`** (number, default `2`): recursion depth (`-1` = unlimited)
  - **`include_empty_names`** (bool, default `false`): include nodes whose `name` is empty/whitespace
- **Output**
  - For `"None"`: typically returns an object with `nodes: [...]`
  - For a concrete `item_id`: returns a single item object (with `children` populated when `depth > 0`)

#### `workflowy_list`

List descendants of a node as a **flat list**.

- **Inputs**
  - **`item_id`** (string, default `"None"`)
  - **`depth`** (number, default `2`; `-1` = unlimited)
  - **`include_empty_names`** (bool, default `false`)
- **Output**
  - `{ "items": [...] }` where each item has `children` set to `null` / omitted (flattened)

#### `workflowy_search`

Search node names by text or regular expression.

- **Inputs**
  - **`pattern`** (string, required): search text or regex
  - **`item_id`** (string, default `"None"`): limit search to this subtree
  - **`regexp`** (bool, default `false`): treat `pattern` as regex
  - **`ignore_case`** (bool, default `false`)
- **Output**
  - `{ "results": [...] }` (matches within the chosen subtree)

#### `workflowy_targets`

List available Workflowy targets (shortcuts and system targets).

- **Inputs**: none
- **Output**
  - `{ "targets": [...] }`

#### `workflowy_report_count`

Generate a descendant count report.

- **Inputs**
  - **`item_id`** (string, default `"None"`)
  - **`threshold`** (number, default `0.01`): minimum ratio (0.0–1.0)
  - **`preserve_tags`** (bool, default `false`): accepted but currently **not used**
- **Output**
  - A Workflowy-style item tree (`name`, `children`, …) representing the report

#### `workflowy_report_children`

Rank nodes by immediate children count.

- **Inputs**
  - **`item_id`** (string, default `"None"`)
  - **`top_n`** (number, default `20`; `0` = all)
  - **`preserve_tags`** (bool, default `false`): accepted but currently **not used**
- **Output**
  - A JSON structure containing the ranking output (including `top_n`)

#### `workflowy_report_created`

Rank nodes by creation date (oldest first).

- **Inputs**
  - **`item_id`** (string, default `"None"`)
  - **`top_n`** (number, default `20`; `0` = all)
  - **`preserve_tags`** (bool, default `false`): accepted but currently **not used**
- **Output**
  - A JSON structure containing the ranking output (including `top_n`)

#### `workflowy_report_modified`

Rank nodes by modification date (oldest first).

- **Inputs**
  - **`item_id`** (string, default `"None"`)
  - **`top_n`** (number, default `20`; `0` = all)
  - **`preserve_tags`** (bool, default `false`): accepted but currently **not used**
- **Output**
  - A JSON structure containing the ranking output (including `top_n`)

### Write tools (must be explicitly enabled)

#### `workflowy_create`

Create a new node.

- **Inputs**
  - **`name`** (string, required)
  - **`parent_id`** (string, default `"None"`)
  - **`position`** (string, optional): `"top"` or `"bottom"`
  - **`layout_mode`** (string, optional): `bullets`, `todo`, `h1`, `h2`, `h3`
  - **`note`** (string, optional)
- **Output**
  - Workflowy API response (includes `item_id`)

#### `workflowy_update`

Update an existing node.

- **Inputs**
  - **`item_id`** (string, required)
  - **`name`** (string, optional)
  - **`note`** (string, optional)
  - **`layout_mode`** (string, optional)
- **Constraints**
  - Must specify at least one of `name`, `note`, or `layout_mode`.
- **Output**
  - Workflowy API response (status)

#### `workflowy_delete`

Delete a node.

- **Inputs**
  - **`item_id`** (string, required)
- **Output**
  - Workflowy API response (status)

#### `workflowy_complete`

Mark a node as complete.

- **Inputs**
  - **`item_id`** (string, required)
- **Output**
  - Workflowy API response (status)

#### `workflowy_uncomplete`

Mark a node as uncomplete.

- **Inputs**
  - **`item_id`** (string, required)
- **Output**
  - Workflowy API response (status)

#### `workflowy_replace`

Search and replace text in node names using regex.

- **Inputs**
  - **`pattern`** (string, required): regex to match
  - **`substitution`** (string, required): replacement (supports capture groups)
  - **`parent_id`** (string, default `"None"`): limit to a subtree
  - **`depth`** (number, default `-1`): max traversal depth (`-1` = unlimited)
  - **`ignore_case`** (bool, default `false`)
  - **`dry_run`** (bool, default `true`)
- **Behavior**
  - If `dry_run=false`, updates are applied **without interactive confirmation**.
- **Output**
  - `{ "results": [...] }` with per-match status (`applied`, `skipped`, etc.)

## Claude Desktop configuration example

Add a server entry to Claude Desktop config:

```json
{
  "mcpServers": {
    "workflowy": {
      "command": "workflowy",
      "args": ["mcp", "--expose=read", "--log-file=/tmp/workflowy-mcp.log"]
    }
  }
}
```

To enable write tools (be careful):

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


