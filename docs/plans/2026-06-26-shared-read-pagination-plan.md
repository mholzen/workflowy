# Shared Read Pagination Implementation Plan

## Goal

Bound large Workflowy read responses without duplicating the semantics of existing commands.

## Tasks

1. Add shared pagination and item-sorting helpers in `pkg/workflowy`.
2. Add optional `limit`, `offset`, and `sort` inputs to CLI and MCP `get`, `list`, and `search`.
3. Preserve legacy output shapes unless pagination is explicitly requested.
4. Keep `order_by` as a search compatibility alias.
5. Add tests for pagination boundaries, sorting, compatibility, and ancestor conflicts.
6. Update MCP, CLI, README, and changelog documentation.
7. Run formatting, tests, vet, build, and diff validation.
