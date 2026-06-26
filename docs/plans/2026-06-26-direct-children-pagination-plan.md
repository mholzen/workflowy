# Direct Children Pagination Implementation Plan

## Goal

Expose a bounded direct-children listing surface for large Workflowy nodes.

## Tasks

1. Add shared child pagination/projection helpers in `pkg/workflowy`.
2. Add `workflowy_children` MCP tool with `id`, `limit`, `offset`, `compact`, `name_filter`, `ignore_case`, and `method`.
3. Register the tool in `--expose=read`, `--expose=all`, and `--expose=children`.
4. Add `workflowy children` CLI parity.
5. Add focused unit tests for stable ordering, pagination boundaries, compact projection, full projection, and name filtering.
6. Update MCP, CLI, README, and changelog documentation.
7. Run Go tests when Go tooling is available.
