# Design: paginated direct children for MCP

## Problem

The existing MCP read tools return a node subtree, flattened descendants, or recursive search results. Large nodes can produce hundreds of kilobytes or more of JSON, which makes it impossible for an AI assistant to enumerate and triage direct children in bounded batches.

## Decision

Add a dedicated `workflowy_children` MCP tool instead of changing `workflowy_list`.

This is the lowest-risk option because existing tool schemas and default output shapes remain unchanged. The new tool has one focused contract: list only direct children of a parent, sort them into stable outline order, then apply filtering and pagination.

## Behavior

- Default access method is the live direct-children API: `GET /api/v1/nodes?parent_id=<id>`.
- `method=export` and `method=backup` are available as explicit fallbacks.
- Results are sorted by `priority` ascending, then `id` ascending before pagination.
- `name_filter` is a regular expression applied to direct child names before pagination.
- `limit` defaults to 50 and is capped at 200 to keep tool responses bounded.
- `offset` is zero-based.
- The response includes `total`, `limit`, `offset`, `has_more`, and `next_offset` when another page exists.

## Compact output

The default compact projection returns:

- `id`
- `name`
- `layoutMode` when present
- `completed`
- `has_children` when the source data contains child information

The live direct-children API does not expose child counts, so `has_children` is omitted for live API results instead of returning a misleading `false`. Backup/export sources include this field when children were present in the loaded tree.

## Response example

```json
{
  "items": [
    {
      "id": "3495d784-5db2-408f-8c4a-7ae1be810d4f",
      "name": "Triage this",
      "layoutMode": "todo",
      "completed": false
    }
  ],
  "total": 1000,
  "limit": 50,
  "offset": 0,
  "next_offset": 50,
  "has_more": true
}
```

## CLI parity

Add `workflowy children [<id>]` with the same pagination and filter behavior. The CLI is useful for manual verification and mirrors the MCP capability without changing `get` or `list`.
