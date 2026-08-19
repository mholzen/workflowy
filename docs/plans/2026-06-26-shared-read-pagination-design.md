# Design: shared pagination for read commands

## Problem

The existing MCP read tools can return large node trees, flat descendant lists, or recursive search results. Large Workflowy nodes can therefore exceed the useful context available to an MCP client.

## Decision

Add optional pagination to the existing `get`, `list`, and `search` surfaces instead of introducing a fourth read command with overlapping semantics.

The existing response contract stays unchanged when neither `limit` nor `offset` is provided. Supplying either argument returns one shared envelope:

```json
{
  "node": { "id": "abc", "name": "Projects" },
  "items": [],
  "total": 1000,
  "limit": 50,
  "offset": 0,
  "next_offset": 50,
  "has_more": true
}
```

`limit` defaults to 50 once pagination is active and is capped at 200. An explicit `limit` of 0 is an error rather than a request for the default, so a caller can never be handed 50 results while believing it asked for none. `offset` is zero-based.

`node` names the node the page belongs to. It is present only for `get` on a specific node, because that is the only case where the items are one node's children. `get` at root returns the top-level nodes, and `list` and `search` page a collection spanning the whole subtree; neither has a single node to name, so `node` is absent.

## Command semantics

- `get`: page the selected node's direct children. Each returned child retains the descendant depth requested by `depth`.
- `list`: flatten the existing result and page that flat collection. Where the sort is applied depends on its kind, see below.
- `search`: sort matches, groups, or tree roots and page the top-level collection.

Pagination and ancestor retrieval are mutually exclusive because an ancestor spine is not a collection of child results.

## Sorting

All three commands expose `sort`, defaulting to `priority`. A leading `+` or `-` controls direction. `get` and `list` support `priority`, `name`, `created`, and `modified`; search additionally preserves its existing `match`, `parent`, and `path` values. Search's `order_by` remains as a compatibility alias.

Sorting happens before pagination.

`priority` is Workflowy's index of a node among its own siblings, so it only orders nodes that share a parent. That makes it a structural field rather than a value carried by the node, and the two kinds are applied at different points:

- A structural sort is applied to each sibling group while the result is still a tree, before anything is flattened or collected. For `get` and the search `tree` grouping that is the natural place anyway. For `list` the tree is sorted and then flattened, so `sort=priority` reproduces Workflowy's outline order. For flat and grouped search results the outline is sorted before matches are gathered, so the depth-first collection order carries the sort.
- Every other sort is a value each node carries on its own, so it is applied to the whole flat result set, after flattening or collection. This is what makes `list --sort=modified --limit=50` a usable ranking rather than a per-parent reshuffle.

Applying a structural sort to search results after collecting them would be wrong in a subtler way: reversing a depth-first sequence for `-priority` yields reverse depth-first order, which places a child ahead of its own parent and disagrees with what `-priority` does to `list`. Sorting the outline first keeps the two commands consistent.

Applying `priority` to a flattened list instead would sort by sibling index across unrelated parents, grouping every first child together and destroying the outline — the default sort would be the one that scrambles the output.

Stable sorting preserves Workflowy's outline order when values tie, which is especially important for backups because they do not contain priority values.

## Output formats

The envelope is a JSON contract. `--format=list` and `--format=markdown` keep rendering outline content and print the window (`# 1-50 of 1000`) on stderr, so piping stdout still yields only the outline.

## Compatibility

- No new MCP tool or CLI command is added.
- Existing unpaginated response shapes remain unchanged.
- Existing `search --order-by` and MCP `order_by` callers continue to work.
