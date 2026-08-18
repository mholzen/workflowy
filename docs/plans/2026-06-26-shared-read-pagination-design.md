# Design: shared pagination for read commands

## Problem

The existing MCP read tools can return large node trees, flat descendant lists, or recursive search results. Large Workflowy nodes can therefore exceed the useful context available to an MCP client.

## Decision

Add optional pagination to the existing `get`, `list`, and `search` surfaces instead of introducing a fourth read command with overlapping semantics.

The existing response contract stays unchanged when neither `limit` nor `offset` is provided. Supplying either argument returns one shared envelope:

```json
{
  "items": [],
  "total": 1000,
  "limit": 50,
  "offset": 0,
  "next_offset": 50,
  "has_more": true
}
```

`limit` defaults to 50 once pagination is active and is capped at 200. `offset` is zero-based.

## Command semantics

- `get`: page the selected node's direct children. Each returned child retains the descendant depth requested by `depth`.
- `list`: flatten the existing result, sort it, then page that flat collection.
- `search`: sort matches, groups, or tree roots and page the top-level collection.

Pagination and ancestor retrieval are mutually exclusive because an ancestor spine is not a collection of child results.

## Sorting

All three commands expose `sort`, defaulting to `priority`. A leading `+` or `-` controls direction. `get` and `list` support `priority`, `name`, `created`, and `modified`; search additionally preserves its existing `match`, `parent`, and `path` values. Search's `order_by` remains as a compatibility alias.

Sorting happens before pagination. Stable sorting preserves Workflowy's outline order when values tie, which is especially important for backups because they do not contain priority values.

## Compatibility

- No new MCP tool or CLI command is added.
- Existing unpaginated response shapes remain unchanged.
- Existing `search --order-by` and MCP `order_by` callers continue to work.
