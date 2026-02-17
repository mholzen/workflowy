# Design: `--ancestor-depth` and `--to-ancestor` options for `get`

## Problem

The `--include-ancestors` flag is all-or-nothing: it wraps the target in every ancestor from root. Users need finer control -- include only N levels of ancestors, or ancestors up to a specific node.

## Solution

Add two new flags that control how many ancestors are included in the spine:

| Flag | Type | Default | Description |
|---|---|---|---|
| `--ancestor-depth N` | int | 0 | Include N levels of ancestors. -1 = all (same as `--include-ancestors`) |
| `--to-ancestor ID` | string | "" | Include ancestors up to and including the node with this ID |
| `--include-ancestors` | bool | false | Convenience alias for `--ancestor-depth=-1` (kept) |

## Conflict Rules

Only one ancestor option at a time. These combinations are errors:
- `--include-ancestors` + `--ancestor-depth`
- `--include-ancestors` + `--to-ancestor`
- `--ancestor-depth` + `--to-ancestor`

Error message: "cannot combine ancestor options: use only one of --include-ancestors, --ancestor-depth, or --to-ancestor"

## How ancestor-depth works

For node `d` with ancestor chain `[a, b, c]` (3 ancestors):
- `--ancestor-depth=0` (default): no ancestors, same as today
- `--ancestor-depth=1`: spine includes only `c` (immediate parent)
- `--ancestor-depth=2`: spine includes `b > c`
- `--ancestor-depth=3` or `-1`: spine includes `a > b > c` (full path)

Implementation: after `FindItemWithAncestors`, truncate the ancestors slice to keep only the last N entries.

## How --to-ancestor works

`--to-ancestor=b` with target `d` (ancestors `[a, b, c]`): spine starts at `b`, ancestors become `[b, c]`.

Implementation: find the index of the `--to-ancestor` ID in the ancestor chain, slice from that index. Error if the ID is not found among the ancestors.

## New Functions (pkg/workflowy/tree.go)

### TruncateAncestors

```go
func TruncateAncestors(ancestors []*Item, depth int) []*Item
```

Returns the last `depth` elements of the ancestors slice. If depth is -1 or >= len(ancestors), returns the full slice. If depth is 0, returns nil.

### SliceAncestorsTo

```go
func SliceAncestorsTo(ancestors []*Item, ancestorID string) ([]*Item, error)
```

Returns ancestors from the matching ID onward. Error if ancestorID not found.

## CLI Changes (cmd/workflowy/flags.go)

Add to `getFetchFlags()`:
- `--ancestor-depth` (int, default 0)
- `--to-ancestor` (string)

## CLI Changes (cmd/workflowy/fetch.go)

Resolve the three ancestor flags into a unified mode early in `fetchItems`:
1. Validate no conflicts
2. Determine if ancestors are needed (any flag set)
3. After finding ancestors, apply truncation or slicing
4. Pass result to `BuildAncestorSpine`

## MCP Changes (pkg/mcp/tools.go)

Add to `buildGetTool()`:
- `ancestor_depth` (number, default 0)
- `to_ancestor` (string)

Same conflict validation and ancestor processing in handler.

## Method Handling

Same rules as `--include-ancestors`: any ancestor option forces export (or backup), errors on explicit `--method=get`.

## Interface Parity

| CLI | MCP |
|---|---|
| `--include-ancestors` | `include_ancestors` (bool) |
| `--ancestor-depth N` | `ancestor_depth` (number) |
| `--to-ancestor ID` | `to_ancestor` (string) |
