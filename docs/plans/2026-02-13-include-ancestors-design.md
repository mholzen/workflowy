# Design: `--include-ancestors` option for `get`

## Problem

When fetching a node with `get`, the response contains only the node and its descendants. There is no context about where the node lives in the tree. Users need the full path from root to understand the node's position.

## Solution

Add an `--include-ancestors` boolean flag to the `get` command (CLI) and `include_ancestors` parameter (MCP). When set, the response wraps the target node in its ancestor chain from root, producing a "spine" tree.

## Approach

Build the ancestor spine from the full export tree (Approach A):

1. Fetch the full tree via export or backup (already cached for 60s)
2. Find the target node by walking the tree, tracking the ancestor chain
3. Reconstruct a spine tree: each ancestor has one child (the next in the path), the target node gets its normal children (respecting depth)
4. Return the spine as the top-level result

## Data Flow

When `--include-ancestors` is set and the user requests node X:

1. Load the full tree (via export or backup -- never `get`)
2. Walk the tree to find node X, collecting the ancestor chain
3. Build a spine tree: clone each ancestor with only the path-child in its Children, until the target node which keeps its normal children (respecting `--depth`)
4. Return the spine root as the result (an `*Item`)

## New Functions

### `FindItemWithAncestors` (pkg/workflowy/tree.go)

```go
func FindItemWithAncestors(items []*Item, targetID string) (*Item, []*Item)
```

Returns `(targetItem, ancestors)` where `ancestors[0]` is the top-level parent and `ancestors[len-1]` is the immediate parent. Returns `(nil, nil)` if not found.

### `BuildAncestorSpine` (pkg/workflowy/tree.go)

```go
func BuildAncestorSpine(target *Item, ancestors []*Item, maxDepth int) *Item
```

- Creates shallow copies of each ancestor (same metadata, but Children replaced)
- Each ancestor copy has exactly one child: the next node in the chain
- The target node keeps its children (depth-limited by maxDepth)
- Returns the root of the spine
- When target is root itself (ancestors is empty), returns the target as-is

## CLI Changes

### flags.go

Add `--include-ancestors` boolean flag to `getFetchFlags()`.

### fetch.go

- Method auto-selection switches to `export` when `--include-ancestors` is set
- The `export` and `backup` branches gain ancestor-wrapping logic after finding the item
- The `get` method branch returns an error if `--include-ancestors` is set

## MCP Changes

### tools.go -- `buildGetTool()`

- Add `include_ancestors` boolean parameter (default false)
- Same method validation logic
- After fetching, wrap result in ancestor spine

## Method Handling

| `--method` value | `--include-ancestors` | Behavior |
|---|---|---|
| (unspecified) | true | Force `export` |
| `export` | true | Proceed normally |
| `backup` | true | Proceed normally |
| `get` | true | Error: "Cannot use --include-ancestors with --method=get (requires full tree)" |

## Output Example

For `workflowy get <planning-id> --include-ancestors --depth=1`:

```json
{
  "id": "root-child-id",
  "name": "Projects",
  "children": [{
    "id": "q1-id",
    "name": "Q1",
    "children": [{
      "id": "planning-id",
      "name": "Planning",
      "children": [
        {"id": "task1-id", "name": "Task 1"},
        {"id": "task2-id", "name": "Task 2"}
      ]
    }]
  }]
}
```

Each ancestor is a full Item (with all metadata), but with children filtered to only the path child.

## Interface Parity

The feature is exposed across all interfaces:
- **CLI**: `--include-ancestors` flag on `get` command
- **MCP**: `include_ancestors` boolean parameter on `workflowy_get` tool

## Testing

- Unit tests for `FindItemWithAncestors` and `BuildAncestorSpine` in `pkg/workflowy/tree_test.go`
- Integration tests for CLI `--include-ancestors` flag
- Integration tests for MCP `include_ancestors` parameter
