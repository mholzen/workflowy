# Workflowy Mirror Support Design

## Summary

Add consistent mirror-aware descendant fetching across Workflowy's `get`, `export`, and `backup` access methods; make search traverse mirror contents with explicit occurrence controls; and add CLI and MCP commands for creating mirrors. Mirror behavior is enabled by default, can be disabled per invocation, and depends on mirror metadata rather than a hardcoded production/beta capability check.

This design uses the source-tree and resolved-tree language defined in `docs/architecture.md`. Detailed contract research is recorded in `docs/research/2026-08-03-workflowy-mirror-children.md`.

## Motivation

Workflowy's beta list endpoint returns an origin node's children when the requested parent is a mirror. The beta export remains a flat source tree: mirror rows expose `data.mirror.origin_id`, but the origin's child rows retain the origin's `parent_id`. Backups carry the equivalent relationship as `metadata.mirror.originalId` and likewise usually omit children beneath mirror rows.

Without local resolution, `method=export` and `method=backup` show mirror nodes as empty or incomplete even though `method=get` exposes their descendants. Users should receive the same descendant behavior from all access methods, while retaining a way to treat mirrors as leaves.

Search should see content reachable through mirrors without returning the same underlying match repeatedly by default. Reports and transforms should retain source-tree semantics so mirrors do not inflate counts or cause repeated writes.

Workflowy's beta API also supports creating mirrors. The CLI and MCP server should expose that operation through the existing deployment-selection and access-control conventions without preventing requests to production.

## Goals

- Return mirror descendants consistently from `get`, `export`, and `backup` reads.
- Enable mirror resolution by default and allow callers to make mirrors leaves.
- Search content reached through mirrors once by default or once per occurrence on request.
- Keep reports, transforms, and write validation on source trees.
- Create mirrors through matching CLI and MCP interfaces.
- Let the selected API deployment determine whether the mirror endpoint is supported.
- Preserve troubleshooting paths through contextual warnings, errors, and tests.

## Non-Goals

- Do not add a separate mirror-removal command; existing node deletion removes a mirror.
- Do not make reports count resolved mirror descendants.
- Do not make transforms apply the same write through multiple mirror occurrences.
- Do not normalize all beta and production response differences.
- Do not maintain a deployment capability table or preflight endpoint support.
- Do not merge a mirror node's source children with its origin's children.

## Domain Model

### Source tree

A source tree contains the parent-child relationships encoded directly by an export or backup. It remains the input for write validation, reports, transforms, and mirror analysis. Mirror-aware fetch and search read validation uses the resolved tree.

### Resolved tree

A resolved tree is an immutable view derived from a source tree. It is not a second, fully materialized tree. Fetch and search traverse the source tree through the resolved child rule only as their operation requires:

| Node state | `resolve_mirrors=true` | `resolve_mirrors=false` |
| --- | --- | --- |
| Ordinary node | Source children | Source children |
| Mirror with available origin | Origin's source children | No children |
| Mirror with null origin | Mirror's source children | No children |
| Mirror with referenced origin absent from source tree | Mirror's source children and warning | No children |

For a resolved mirror, origin children replace mirror source children; the two sets are never combined. Nested mirrors follow the same rule recursively.

### Mirror reference

One Workflowy module owns mirror detection and source-field normalization. It recognizes:

- API/export: `Item.Data["mirror"]["origin_id"]`
- Backup: `Item.Data["mirror"]["originalId"]`

The result distinguishes an ordinary node, a mirror with an origin ID, a mirror with a null origin, and malformed mirror metadata. This distinction is required because `origin_id: null` still identifies a mirror and must become a leaf when resolution is disabled.

Workflowy origin nodes legitimately carry `mirror_ids` or `mirrorRootIds` without an origin field. A mirror object with neither `origin_id` nor `originalId` is therefore ordinary for child resolution. Malformed metadata means one of the recognized origin fields is present but its value is neither a string nor null. It still identifies a mirror. With resolution enabled, traversal uses the mirror's source children and emits a contextual warning; with resolution disabled, the mirror is a leaf. A non-object value under the `mirror` key has no recognized origin field and remains ordinary for child resolution.

Mirror detection is based only on item metadata. It never checks `APIDeployment`, so production begins using the same behavior when its responses expose supported metadata.

### Node occurrence

A node occurrence is one path to a node in a resolved tree. Origin descendants can have an original occurrence and multiple occurrences beneath mirrors. Occurrences share node IDs but carry different ancestor paths.

An occurrence's identity is the ordered ancestor-ID path plus its node ID. This identity is used wherever separate occurrences must survive, including `search_mirrors=true` tree output. Node IDs and origin IDs remain data attributes and default-deduplication keys; they are not occurrence identities.

For search deduplication, a mirror node uses its origin node ID as the equality key when the origin ID is available; an ordinary node uses its own ID. This prevents multiple mirror roots for the same origin from producing repeated default results. The returned result still uses the ID and path of the chosen occurrence.

## ResolvedTree Module

Create a focused Workflowy module named `ResolvedTree`. It accepts source-tree roots, builds a private ID index once, and exposes two behaviors:

1. Locate a requested occurrence and return fresh, depth-limited resolved roots or a resolved subtree for fetch operations.
2. Visit node occurrences with their ancestor paths for search without retaining non-matching occurrences.

The module does not mutate source `Item` values or their `Children` slices. Fetch results contain fresh item structures so depth limiting, flattening, filtering, and formatting cannot alter the source tree or another occurrence. A targeted fetch does not enumerate unrelated roots. A finite-depth root fetch does not traverse below that depth. Search streams every occurrence path to its matcher and discards non-matching occurrences immediately. The matcher retains one selected match per equality key by default, replacing a mirrored match with a later original occurrence when available; `search_mirrors=true` retains every matching occurrence instead.

Mirror expansion tracks the source node IDs currently being expanded on the current path, including ordinary nodes and the initially requested source subtree. Before following a mirror origin, it checks that origin ID against this expansion path. If the origin is already present, the affected mirror occurrence becomes a leaf and a warning identifies the mirror, origin, and path. This stops an origin containing a mirror of itself before its subtree is expanded a second time. The expansion path is local to each traversal branch, not global, because the same origin may validly appear at multiple independent mirror locations. Recursive direct API fetching applies the same rule using node IDs and mirror metadata returned by the API.

Path-local cycle detection bounds each branch but does not bound the number of distinct branches. Layered or densely connected mirrors can produce exponentially many non-cyclic paths. Search must visit those paths because completion state and cycle boundaries depend on occurrence ancestry, but it retains only active traversal state and selected matches rather than a complete occurrence tree. Unlimited root output can remain large because the complete output was explicitly requested, and `search_mirrors=true` can retain many matches; diagnostics make traversal and retention growth visible.

When a non-empty origin ID is absent from the source-tree index, the module uses the mirror's source children and logs the mirror ID, missing origin ID, and source label. A documented null origin uses source children without failing the invocation. When resolution is disabled, all recognized mirrors are leaves and no origin lookup occurs.

## User Interface

### Mirror resolution

The CLI `get`, `list`, and `search` commands accept a command-local boolean option:

```text
--resolve-mirrors=<true|false>
    Resolve mirror descendants (default: true)
```

The matching MCP tools `workflowy_get`, `workflowy_list`, and `workflowy_search` accept:

```json
{
  "resolve_mirrors": false
}
```

The MCP property is boolean and defaults to `true`. The option controls descendants only. A requested mirror node is still returned with the content and metadata supplied by the selected source.

The option applies to every access method:

- `get`: inspect each retrieved node's mirror metadata before requesting its children. When resolution is disabled, a mirror is returned without descendants.
- `export`: derive the requested result from `ResolvedTree`.
- `backup`: derive the requested result from `ResolvedTree`.

For recursive `get`, ordinary children are fetched normally. A child identified as a mirror is not used as the parent of another list request when resolution is disabled. When resolution is enabled, Workflowy's server-provided child behavior remains authoritative.

### Search mirror occurrences

The CLI `search` command accepts:

```text
--search-mirrors
    Return matches from each mirror occurrence
```

The MCP `workflowy_search` tool accepts boolean `search_mirrors`, defaulting to `false`.

Search always uses the resolved tree when `resolve_mirrors=true`:

- Default: return one result per equality key. Prefer an original occurrence when it is inside the search scope. Otherwise retain the first mirror occurrence in source-tree priority order.
- `search_mirrors=true`: return every matching occurrence, including the original occurrence when it is inside the search scope. Each result retains that occurrence's parent and path.

`resolve_mirrors=false` with `search_mirrors=true` is invalid and returns:

```text
Cannot search mirror occurrences when mirror resolution is disabled
```

All existing flat, grouped, and tree search output modes use the same occurrence policy before grouping and sorting.

Tree output keys its internal branches by occurrence identity rather than node ID. Therefore `search_mirrors=true` can retain two branches containing the same node IDs beneath different mirror paths. The serialized nodes continue to expose Workflowy node IDs; the occurrence identity is traversal state, not a new public identifier.

### Mirror creation command

Add a CLI command:

```text
workflowy mirror <id> <parent-id> [--position=top|bottom] [--method=get|export|backup] [--api=production|beta]
```

`api` is the existing shared deployment selector, not a mirror-specific requirement. The command does not enforce beta. It resolves and validates through the selected deployment, sends the mirror request there, and wraps whatever API error that deployment returns. `method` chooses the read source used to resolve IDs and enforce access restrictions; the write always uses the selected API deployment.

The command accepts the repository's normal full UUID, unique short ID, and Workflowy URL inputs for `id` and `parent-id`. Target keys may be resolved to full UUIDs when the selected validation source supports them. Root (`None`) is rejected as a destination. The request sent to Workflowy always contains full IDs.

`position` accepts `top` and `bottom`; omission uses Workflowy's default. Successful text output identifies both the new mirror ID and the origin ID. JSON output returns the response object unchanged.

Add an MCP tool:

```text
workflowy_mirror
```

with parameters:

| Parameter | Type | Required | Meaning |
| --- | --- | --- | --- |
| `id` | string | yes | Node to mirror |
| `parent_id` | string | yes | Destination parent |
| `position` | string enum | no | `top` or `bottom` |
| `method` | string enum | no | Validation source: `get`, `export`, or `backup` |
| `api` | string enum | no | Selected Workflowy deployment |

The tool joins the MCP `write` and `all` groups and receives the same centralized per-invocation API selection wrapper as other network tools. The `mirror` expose alias maps to it.

Existing `workflowy delete <mirror-id>` and `workflowy_delete` remain the supported removal interfaces because Workflowy's ordinary delete endpoint has the same effect as its mirror-specific delete endpoint.

## Workflowy Client Interface

Add request and response types that reflect the operation rather than reusing move types:

```go
type MirrorNodeRequest struct {
    ParentID string  `json:"parent_id"`
    Position *string `json:"position,omitempty"`
}

type MirrorNodeResponse struct {
    ItemID   string `json:"item_id"`
    OriginID string `json:"origin_id"`
}
```

`MirrorNodeRequest.SetPosition` uses the same position validation as create and move. Extend `workflowy.Client` with:

```go
MirrorNode(ctx context.Context, itemID string, request *MirrorNodeRequest) (*MirrorNodeResponse, error)
```

The concrete client sends `POST /nodes/:id/mirror`. It does not branch on deployment or interpret unsupported-endpoint responses.

Recursive child fetching receives an explicit mirror-resolution choice. It returns mirror nodes unchanged and skips their descendant request when resolution is disabled. It tracks every source node ID on each active expansion path, beginning with the initially requested ordinary or mirror node, so recursive requests stop safely at mirror cycles. Authentication and HTTP debug logging remain unchanged.

## Access Restrictions

Resolved fetch and search operations evaluate read restrictions against the same resolved tree used for their results. A mirror under the read root therefore makes its resolved descendants readable through that occurrence. An ID-only access check passes when at least one occurrence with the target ID is reachable below the resolved read root.

When a requested ID has multiple reachable occurrences, fetch selects the original occurrence if it is inside the resolved read scope; otherwise it selects the first reachable occurrence in source-tree priority order. The same rule chooses an ancestor path when ancestor output is requested. A fetch that does not expose ancestors still returns the chosen occurrence's fresh node structure and resolved descendants. Reports, transforms, and write operations retain source-tree validation.

Mirror creation validates:

1. The requested source node against `read_root_id`.
2. The effective destination against `write_root_id`.

Workflowy resolves a mirror destination parent to that parent's origin before creating the new mirror. When a write restriction is active and the validation source identifies the requested parent as a mirror with an available origin, validation uses that origin ID. This prevents a parent mirror inside the write root from redirecting the actual write outside the permitted source-tree scope.

If the validation source identifies a mirror parent but cannot provide its origin, restricted mirror creation fails contextually rather than guessing the effective destination. With no write restriction, the selected API remains responsible for destination resolution and validation.

The API also rejects a destination that resolves to the same origin being mirrored. The client does not duplicate this server rule unless the already-loaded validation data proves the conflict; the API's actual response remains authoritative.

## Data Flow

### Export or backup fetch

1. Load the source tree once from the selected method.
2. Resolve the requested ID and active read root against that source.
3. Construct `ResolvedTree` with the source label.
4. If an original occurrence is directly reachable inside the resolved read scope, select it without walking unrelated branches. Reachability must validate every ancestor edge with the resolved child rule because available mirror origins replace their mirror's source children. Otherwise traverse until the first reachable mirror occurrence is found.
5. Materialize only that occurrence and its requested descendant depth, carrying its ancestor expansion-path state so cycle decisions match the path through which access was granted.
6. Format or flatten only the fresh resolved result.

For a root request, materialize each resolved scope root directly at the requested depth. Do not perform a separate unlimited selection traversal first.

### Direct API fetch

1. Retrieve the requested node when the request is not for the root.
2. Return immediately without children when depth is zero.
3. If resolution is disabled and the node is a mirror, return it as a leaf.
4. Otherwise fetch children and recurse.
5. Before recursing into each returned child, apply the same metadata-driven mirror check.

### Search

1. Load the export or backup source tree once.
2. Construct `ResolvedTree` and select the requested search-root occurrence using the same original-first, first-reachable-mirror fallback as fetch. Carry the selected occurrence's ancestor and expansion path into its subtree traversal.
3. Visit occurrences one at a time and evaluate the search pattern immediately.
4. Visit every occurrence path because completion filtering and mirror-cycle boundaries depend on its ancestry. Discard non-matches immediately. Deduplicate retained matches by origin ID for mirrors and node ID otherwise unless `search_mirrors=true`.
5. Prefer an original in-scope occurrence; otherwise retain the first mirror occurrence by source-tree priority.
6. Group, sort, and render the selected occurrences with their chosen paths.

### Mirror creation

1. Select the deployment through the existing CLI or MCP API-selection path.
2. Resolve source and parent references to full IDs using the selected validation source.
3. Validate source read access.
4. Determine and validate the effective destination when write restrictions apply.
5. Validate position.
6. Send `POST /nodes/:id/mirror` to the selected deployment.
7. Return `item_id` and `origin_id`, or wrap the deployment's actual error with source, parent, and deployment context.

## Error Handling and Observability

New errors follow the repository's `Cannot ...` convention and contain the relevant IDs, source, and deployment. Required cases include:

- Conflicting search flags.
- Restricted mirror creation with an unresolved effective destination.
- Source or destination not found in the selected validation source.
- Invalid root destination or position.
- API errors from mirror creation, preserving the actual status and body.

Warnings are emitted when:

- A non-empty origin reference is absent from the source tree and source children are used.
- Malformed mirror metadata falls back to source children.
- A mirror cycle stops expansion.

Warning attributes include the mirror ID, origin ID when available, source label, and operation. Missing-origin and malformed-metadata warnings remain contextual per encounter. Repeated cycles are consolidated by mirror/origin pair per traversal; the warning includes the first path and total `occurrences`, while the summary's cycle count includes every encounter. No warning is emitted merely because a deployment or ordinary node lacks mirror metadata.

Each resolved-tree or recursive-API traversal emits one info-level summary with counts for successful mirror resolutions, missing-origin fallbacks, malformed-metadata fallbacks, and cycles. A resolution attempt is a mirror with a non-null origin or malformed metadata; null origins are not failures. The failed-resolution count is the sum of missing-origin, malformed-metadata, and cycle counts. When at least one resolution was attempted and failures are 50% or more of attempts, the traversal also emits a warning containing the counts, failure ratio, source label, and operation. Contextual fallback warnings and consolidated cycle warnings remain in addition to this aggregate instrumentation.

For CLI invocations, the threshold warning is written to stderr through `slog.Warn`. For MCP get, list, and search results, the same warning is also prepended as a text content block before the normal JSON content block, leaving the normal JSON payload unchanged. Server logging alone does not satisfy MCP user visibility. Unit tests assert both stderr logging and the additional MCP content block; below-threshold MCP results retain their existing single JSON content block.

Debug logs include the mirror-resolution and search-occurrence choices at invocation start. Resolved-tree indexing logs source-root, node, and mirror counts. Traversal logs its scope, target, requested depth, effective depth, and operation; progress is reported at exponentially increasing occurrence thresholds so accelerating growth is visible without logging every node. Traversal progress includes visited occurrences, current and maximum path depth, elapsed time, heap bytes, heap objects, total allocated bytes, and garbage-collection count. Fetch completion defines retained occurrences as freshly materialized output nodes. Each search visitor result reports its current retained-match count after deduplication and any mirror-to-original replacement, allowing traversal completion to log the final value. Each resolved-tree instance owns its memory-stat reader; the production constructor uses Go runtime statistics, while tests inject an instance-specific reader without mutable package state. Memory statistics are read only when debug logging is enabled. Existing HTTP logs identify the selected deployment and request path, including `/nodes/:id/mirror`.

## Testing

### Mirror metadata and resolved trees

Unit tests cover:

- beta `origin_id`, backup `originalId`, null origin, origin-only `mirror_ids`/`mirrorRootIds`, ordinary nodes, and malformed recognized origin fields;
- ordinary source children;
- resolved mirror children replacing, not merging with, source children;
- disabled resolution making every recognized mirror a leaf;
- missing origin fallback and contextual warning;
- nested mirrors;
- path-local cycle detection and warning;
- aggregate resolution counts, CLI stderr delivery, MCP result delivery, and the 50% failed-resolution warning threshold;
- repeated independent occurrences of one origin;
- depth zero, finite depth, and unlimited depth;
- source-tree items and child slices remaining unchanged after fetch, flatten, and search.
- targeted fetch avoiding unrelated roots and respecting requested depth during its first traversal;
- streaming search retaining matches rather than every visited occurrence;
- a completed occurrence not suppressing a later open mirror occurrence;
- a cycle-truncated occurrence not suppressing a later path with a different cycle boundary;
- debug indexing, traversal-progress, completion, and Go-heap attributes.

### Direct API fetching

Local HTTP-server tests cover:

- direct mirror retrieval with resolution disabled makes no child request;
- recursive root fetching skips child requests beneath mirror results when disabled;
- ordinary nodes recurse when mirror metadata is absent;
- enabled behavior retains server-resolved mirror children;
- production and beta clients use identical metadata-driven logic.

### Search

CLI and MCP tests cover:

- `resolve_mirrors` defaults true and is exposed on get, list, and search;
- `search_mirrors` defaults false and is search-only;
- conflicting options fail before traversal;
- mirror-only content is searched;
- default results deduplicate multiple mirror roots and descendant occurrences;
- an in-scope original occurrence wins over mirrors;
- the first source-priority mirror wins when the original is outside scope;
- `search_mirrors=true` returns all in-scope occurrences and paths;
- flat, parent/path/date grouped, and tree outputs follow the same occurrence policy;
- tree output retains distinct branches with repeated node IDs by keying traversal state on occurrence identity.

### Mirror mutation

Client, CLI, and MCP tests cover:

- exact request method, path, body, and response decoding;
- full/short/URL input resolution and root rejection;
- `top`, `bottom`, and invalid position;
- selected deployment routing with no beta gate;
- production or beta API failures preserved and wrapped contextually;
- source read-root validation;
- effective destination write-root validation when the requested parent is a mirror;
- mirror-of-mirror response where `origin_id` differs from requested `id`;
- exact MCP schema, centralized API dispatch, expose groups, and aliases;
- existing delete remains the documented mirror-removal path.

Race tests verify that concurrent resolved-tree reads, searches with different occurrence policies, and MCP calls with different deployments/options do not share mutable invocation state.

### Integration tests

Read-only integration tests use explicitly configured mirror IDs to compare beta `get` and `export` descendants and to verify `resolve_mirrors=false` returns a mirror leaf. They remain under `just test-integration` and skip when mirror fixtures are not configured.

An optional destructive integration test creates a mirror under an explicitly configured test parent, verifies `item_id` and `origin_id`, and deletes the created mirror during cleanup. It belongs under `just test-integration-write`, skips without dedicated source/parent IDs, and must never target root or user data inferred by the test.

## Documentation

The implementation updates:

- `docs/architecture.md` with stable source-tree and resolved-tree decisions;
- README examples and MCP tool inventory;
- `docs/CLI.md` for `resolve-mirrors`, `search-mirrors`, and `workflowy mirror`;
- `docs/MCP.md` for matching properties, defaults, occurrence behavior, and `workflowy_mirror`;
- command/tool help and schemas;
- `CHANGELOG.md`;
- a feature release post under the repository's existing documentation convention, or `docs/blog/` if no convention exists.

Documentation states that mirror behavior is metadata-driven, beta currently supplies the documented metadata and mutation endpoint, production is not locally blocked, and the selected deployment's actual response is authoritative.

## Alternatives Rejected

### Eager source-tree mutation

Attaching origin children directly to mirror nodes would affect reports, transforms, validation, and later callers. Shared child pointers would also make depth limiting and flattening order-dependent.

### Separate fetch and search resolution

Duplicating mirror traversal in handlers would repeat nested-mirror, missing-origin, cycle, and depth behavior across CLI and MCP.

### Full resolved-scope occurrence materialization

Building a slice containing every occurrence before selecting a fetch result or evaluating a search pattern retains copied ancestor paths, expansion paths, and identity strings for unrelated nodes. Path-local cycle detection cannot prevent combinatorial growth across distinct non-cyclic paths. The resolved-tree module instead performs operation-specific traversal over the source tree and retains only requested output.

### Deployment-gated mirror support

Hardcoding beta would require an application change when Workflowy promotes mirror behavior to production and would duplicate capability knowledge already present in response metadata or the selected API's response.

### Transport switching when resolution is disabled

Forcing export when `get` is selected would make a descendant option unexpectedly change transport and rate-limit behavior. Mirror metadata on retrieved nodes is sufficient to stop child requests locally.

### Mirror-specific removal command

Workflowy's ordinary delete endpoint removes a mirror without deleting its origin, and the repository already exposes delete consistently across CLI and MCP.
