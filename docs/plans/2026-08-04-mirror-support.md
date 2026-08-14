# Workflowy Mirror Support Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make get, list, and search mirror-aware across API, export, and backup sources, and add matching CLI and MCP mirror-creation commands.

**Architecture:** Add an immutable resolved-tree view beside the existing source-tree helpers. It normalizes mirror metadata, selects node occurrences inside a resolved read scope, materializes only requested fetch output, and streams occurrences to a single search pipeline; reports, transforms, and write validation retain source-tree behavior. Direct API recursion uses the same metadata rules, while mirror creation remains a thin deployment-neutral client operation with source-tree access checks.

**Tech Stack:** Go 1.24, urfave/cli/v3, mcp-go, `log/slog`, testify, `httptest`, bats, just.

**Reference:** Implement against `docs/2026-08-04-mirror-support-design.md` and keep `docs/architecture.md` synchronized.

**Starting point:** Create the implementation branch from `6aa849e24ce9066d8ef3088d6964f72d7c4a3e2a` (`fix: complete Workflowy API selection behavior`). Commit the design and this plan on that branch before implementation. Do not replay the previous materialized-tree implementation commits.

---

## Chunk 1: Mirror model and resolved traversal

### Task 0: Record the quality baseline

**Files:**
- Test: all Go packages

- [ ] **Step 1: Verify the starting branch**

Run: `git branch --show-current && git status --short`

Expected: branch `feat/mirror-support-on-demand`, based on `6aa849e24ce9066d8ef3088d6964f72d7c4a3e2a`, with no uncommitted files.

- [ ] **Step 2: Record tests and coverage before implementation**

Run: `just test && go test ./... -coverprofile=/tmp/workflowy-mirror-baseline-cover.out && go tool cover -func=/tmp/workflowy-mirror-baseline-cover.out | tail -1 | tee /tmp/workflowy-mirror-baseline-coverage.txt`

Expected: PASS; `/tmp/workflowy-mirror-baseline-coverage.txt` contains the starting total coverage percentage for the final comparison.

### Task 1: Normalize mirror metadata into a rich result

**Files:**
- Create: `pkg/workflowy/mirror_reference.go`
- Create: `pkg/workflowy/mirror_reference_test.go`

- [ ] **Step 1: Write table-driven mirror classification tests**

Cover these exact cases in `TestMirrorReferenceFromItem`: beta string `origin_id`, backup string `originalId`, each recognized field set to `nil`, each recognized field with a non-string value, origin-only `mirror_ids`, origin-only `mirrorRootIds`, a non-object `mirror` value, missing `data`, and an ordinary data object. Assert the full result, not a boolean.

```go
type MirrorReferenceKind uint8

const (
	MirrorReferenceOrdinary MirrorReferenceKind = iota
	MirrorReferenceWithOrigin
	MirrorReferenceNullOrigin
	MirrorReferenceMalformed
)

type MirrorReference struct {
	Kind     MirrorReferenceKind
	OriginID string
	Field    string
	ValueType string
}
```

Assert that `mirror_ids`, `mirrorRootIds`, and a non-object `mirror` value are ordinary; only a present `origin_id` or `originalId` with an unexpected type is malformed.

- [ ] **Step 2: Run the focused test and verify the missing symbols fail**

Run: `go test ./pkg/workflowy -run TestMirrorReferenceFromItem -count=1`

Expected: FAIL because `MirrorReferenceFromItem` and its result types do not exist.

- [ ] **Step 3: Implement the metadata accessor**

Implement `MirrorReferenceFromItem(item *Item) MirrorReference` with early returns. Check `origin_id` before `originalId`, preserve the recognized field name in the result, use `fmt.Sprintf("%T", value)` for malformed type context, and add:

```go
func (reference MirrorReference) IsMirror() bool {
	return reference.Kind != MirrorReferenceOrdinary
}
```

Do not inspect `APIDeployment`, log, or traverse children in this file.

- [ ] **Step 4: Run the package tests**

Run: `go test ./pkg/workflowy -run 'TestMirrorReference' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit the metadata model**

```bash
git add pkg/workflowy/mirror_reference.go pkg/workflowy/mirror_reference_test.go
git commit -m "feat: normalize Workflowy mirror references"
```

### Task 2: Build the on-demand resolved-tree module

**Files:**
- Create: `pkg/workflowy/resolved_tree.go`
- Create: `pkg/workflowy/resolved_tree_traversal.go`
- Create: `pkg/workflowy/resolved_tree_diagnostics.go`
- Create: `pkg/workflowy/resolved_tree_fetch_test.go`
- Create: `pkg/workflowy/resolved_tree_visit_test.go`
- Create: `pkg/workflowy/resolved_tree_diagnostics_test.go`

- [ ] **Step 1: Write failing index, child-rule, and bounded-fetch tests**

Create fixtures with an origin, source children, two mirror roots, a null-origin mirror, a malformed mirror, a missing-origin mirror, and unrelated roots. Add tests covering:

- available origin children replace rather than merge with mirror source children;
- disabled resolution makes every recognized mirror a leaf;
- null, missing, and malformed origins use source children when enabled;
- root and node fetches return fresh values without mutating the source tree;
- depth zero, finite depth, and unlimited depth;
- a targeted fetch does not traverse an unrelated mirror cycle;
- a depth-zero root fetch does not inspect deeper mirror cycles;
- a source child replaced by mirror resolution is not reachable;
- a reachable original occurrence is preferred;
- the first source-priority mirror occurrence is selected when no original is reachable;
- source descendants beneath null, missing, or malformed mirror fallbacks remain mirror-path occurrences and do not outrank a truly original occurrence;
- materializing a selected descendant preserves every source ID active above it, so a descendant mirror back to an already-active origin stops immediately;
- a mirror-target fetch counts each resolution and cycle encounter once across selection and materialization, with exact `Resolved` and `Cycles` assertions.

Use this public fetch interface from the first test:

```go
type ResolveOptions struct {
	ResolveMirrors bool
	Depth          int
	Operation      string
}

type MirrorResolutionSummary struct {
	Resolved          int
	MissingOrigin     int
	MalformedMetadata int
	Cycles            int
}

type ResolvedFetch struct {
	Item       *Item
	Items      []*Item
	Occurrence NodeOccurrence
	Summary    MirrorResolutionSummary
}

tree := NewResolvedTree(sourceRoots, "test export")
result, err := tree.Fetch("None", mirrorID, ResolveOptions{
	ResolveMirrors: true,
	Depth:          -1,
	Operation:      "get",
})
```

`targetID == "None"` populates `Items`; a node target populates `Item` and `Occurrence`. For immutability, snapshot source JSON, mutate the returned fetch through `FlattenTree` and `FilterEmptyItem`, and assert the source JSON is unchanged.

- [ ] **Step 2: Run the bounded-fetch tests and verify failure**

Run: `go test ./pkg/workflowy -run 'TestResolvedTreeFetch' -count=1`

Expected: FAIL because `ResolvedTree` and its fetch interface do not exist.

- [ ] **Step 3: Implement one source index and target-directed fetch**

In `resolved_tree.go`, index every source item once as a private node containing the item and its source parent. Use that single index for origin lookup, source occurrence reconstruction, and restricted scope lookup. Do not maintain a second item map or recursively search a tree already represented by the index.

Implement `Fetch` as operation-specific traversal:

1. Resolve the requested read-scope source occurrence.
2. For a root request, materialize each scope root directly at the requested depth.
3. For a node request, reconstruct a possible original source path from the index and validate every parent-child edge with the resolved child rule.
4. If that original occurrence is not reachable, traverse only until the first reachable mirror occurrence is found.
5. Before selection traversal unwinds, return a private selection containing snapshots of the public occurrence's ancestors and the active source-ID path at that occurrence; neither may alias reusable traversal stacks.
6. Materialize only the selected occurrence and requested descendant depth, continuing with that active path so cycle detection sees every source ID activated during selection.

Use one operation-scoped summary and diagnostics tracker across selection and materialization. Selection accounts for mirror resolutions needed to reach the target; materialization accounts for the selected node and returned descendants. The selected occurrence itself must not be observed or counted twice merely because fetch has two phases. Do not build an occurrence slice, traverse unrelated roots after selection, or perform an unlimited selection pass before applying a finite requested depth. Return contextual errors such as:

```go
return nil, fmt.Errorf(
	"Cannot find Workflowy node %q within resolved read scope %q from %s",
	targetID,
	readScopeID,
	tree.sourceLabel,
)
```

Place mirror child selection, target selection, materialization, and active-path helpers in `resolved_tree_traversal.go`. Keep source-tree mutation helpers in `tree.go` unchanged.

- [ ] **Step 4: Run bounded-fetch tests and verify they pass**

Run: `go test ./pkg/workflowy -run 'TestResolvedTreeFetch' -count=1`

Expected: PASS.

- [ ] **Step 5: Write failing streaming-visit and cycle tests**

Add tests covering:

- occurrence ancestor paths and occurrence identity;
- equality ID uses an origin ID for a mirror and a node ID otherwise;
- every descendant reached below a mirror has `ViaMirror=true`;
- target scoping uses the same original-first, first-reachable-mirror selection as fetch;
- self-mirror and nested-mirror cycles stop before repeating an active origin;
- independent occurrences of the same origin remain traversable;
- a cycle-truncated occurrence does not suppress a later path with a different cycle boundary;
- every occurrence path is visited without returning or retaining a complete occurrence slice;
- visitor errors stop traversal and retain their context.

Use this streaming interface:

```go
type NodeOccurrence struct {
	Item      *Item
	Ancestors []*Item
	ViaMirror bool
}

func (occurrence NodeOccurrence) Identity() string
func (occurrence NodeOccurrence) EqualityID() string
func (occurrence NodeOccurrence) Snapshot() NodeOccurrence

type OccurrenceVisitResult struct {
	RetainedOccurrences int
}

type OccurrenceVisitor func(NodeOccurrence) (OccurrenceVisitResult, error)

func (tree *ResolvedTree) Visit(
	readScopeID, targetID string,
	options ResolveOptions,
	visitor OccurrenceVisitor,
) (MirrorResolutionSummary, error)
```

The occurrence passed to a visitor is valid only for that callback. `Snapshot` copies its ancestor path for a consumer that chooses to retain it. This lets non-matching search occurrences reuse traversal path storage without allocating copied ancestors and identity strings. `Identity` and `EqualityID` are computed only when a consumer needs them.

- [ ] **Step 6: Run visit tests and verify failure**

Run: `go test ./pkg/workflowy -run 'TestResolvedTreeVisit|TestNodeOccurrence' -count=1`

Expected: FAIL because streaming traversal and occurrence helpers are absent.

- [ ] **Step 7: Implement branch-local traversal without a collecting interface**

Implement depth-first `Visit` in `resolved_tree_traversal.go`. Maintain reusable ancestor and active-source-ID stacks with push/pop ownership inside the traversal. Before following a mirror origin, stop when that origin is already active on the current path. Do not use a global visited set: completion filtering and cycle boundaries depend on occurrence ancestry, and independent paths remain valid.

Expose no `Walk` method and no method that returns all occurrences. Fetch and search are the only required resolved-tree consumers. Search will retain snapshots only for selected matches in Task 6.

- [ ] **Step 8: Run fetch and visit tests**

Run: `go test ./pkg/workflowy -run 'TestResolvedTree|TestNodeOccurrence' -count=1`

Expected: PASS.

- [ ] **Step 9: Write failing observability tests**

Add tests for `MirrorResolutionSummary.Attempts`, `Failures`, and `ThresholdWarning`; contextual missing-origin and malformed warnings; consolidated cycle warnings; and one info summary per public traversal. At debug level, assert:

- indexing records source roots, source nodes, and mirrors;
- traversal start/completion records operation, source, scope, target, requested/effective depth, visited occurrences, retained occurrences, current/maximum path depth, elapsed time, heap bytes, heap objects, total allocated bytes, and GC count;
- progress occurs at 1,000, 2,000, 4,000, and subsequent doubling thresholds;
- memory statistics are not read when debug logging is disabled.

Give each tree an instance-owned memory-stat reader so tests replace it without package-global mutable state. Fetch retained occurrences equal materialized output nodes; Visit uses the most recent `OccurrenceVisitResult.RetainedOccurrences`. Add an exact completion-log regression for the mirror-target fixture from Step 1, proving selection and materialization do not double-count `visited_occurrences` or `retained_occurrences`.

- [ ] **Step 10: Run diagnostics tests and verify failure**

Run: `go test ./pkg/workflowy -run 'TestMirrorResolutionSummary|TestResolvedTree(Logs|DoesNotReadMemoryStats)' -count=1`

Expected: FAIL because diagnostics are not implemented.

- [ ] **Step 11: Implement traversal diagnostics from the outset**

Implement diagnostics in `resolved_tree_diagnostics.go`. Read Go memory statistics only when debug logging is enabled. Log no node names, notes, or credentials. Consolidate repeated cycle warnings by mirror/origin pair while preserving the total cycle count. Emit the aggregate warning at a failure ratio of 50% or greater. Null origins are neither attempts nor failures.

- [ ] **Step 12: Run workflowy tests and race detection**

Run: `go test ./pkg/workflowy -count=1 && go test -race ./pkg/workflowy -count=1`

Expected: PASS with no races.

- [ ] **Step 13: Commit the on-demand resolved-tree module**

```bash
git add pkg/workflowy/resolved_tree.go pkg/workflowy/resolved_tree_traversal.go pkg/workflowy/resolved_tree_diagnostics.go pkg/workflowy/resolved_tree_fetch_test.go pkg/workflowy/resolved_tree_visit_test.go pkg/workflowy/resolved_tree_diagnostics_test.go
git commit -m "feat: add on-demand Workflowy mirror traversal"
```

### Task 3: Make direct API recursion mirror-aware

**Files:**
- Create: `pkg/workflowy/recursive_fetch.go`
- Create: `pkg/workflowy/recursive_fetch_test.go`
- Modify: `pkg/workflowy/workflowy.go`
- Modify: `pkg/workflowy/interfaces.go`
- Modify: `pkg/workflowy/workflowy_test.go`
- Modify: `cmd/workflowy/backup_api_test.go`
- Modify: `cmd/workflowy/client_test.go`
- Modify: `cmd/workflowy/count_report_test.go`
- Modify: `pkg/mcp/api_selection_test.go`
- Modify: `pkg/mcp/tools_test.go`

- [ ] **Step 1: Write HTTP request-count tests**

Using `httptest.Server`, add:

- `TestRecursiveFetchDisabledDoesNotListMirrorChildren`
- `TestRecursiveFetchNilRootRetrievesMirrorBeforeDisabledTraversal`
- `TestRecursiveFetchRejectsMismatchedRootItem`
- `TestRecursiveFetchDisabledStillListsOrdinaryChildren`
- `TestRecursiveFetchEnabledUsesServerMirrorChildren`
- `TestRecursiveFetchStopsMirrorCycle`
- `TestRecursiveFetchInternalRootSeedsCyclePath`
- `TestRecursiveFetchUsesMetadataForBothDeployments`

Record every request path. For a retrieved mirror with resolution disabled, assert the node is returned unchanged and `/nodes?parent_id=<mirror>` is never requested. Repeat with `RootItem=nil`: assert one `GET /nodes/<mirror>` retrieves metadata before the child request is suppressed. For an ordinary root containing a mirror, assert recursion stops below that child only. Assert a supplied root whose ID differs from `itemID` makes no HTTP requests and returns the specified contextual `Cannot ...` error. Assert an internally retrieved root seeds the same active cycle path as a supplied root.

- [ ] **Step 2: Run the focused tests and verify failure**

Run: `go test ./pkg/workflowy -run 'TestRecursiveFetch' -count=1`

Expected: FAIL because the option-bearing fetch API does not exist.

- [ ] **Step 3: Add the explicit recursive fetch contract**

Move recursive traversal out of `workflowy.go` into `recursive_fetch.go` and add:

```go
type RecursiveFetchOptions struct {
	Depth          int
	ResolveMirrors bool
	Operation      string
	RootItem       *Item
}

type RecursiveFetchResult struct {
	Response *ListChildrenResponse
	Summary  MirrorResolutionSummary
}

ListChildrenRecursiveWithOptions(ctx context.Context, itemID string, options RecursiveFetchOptions) (*RecursiveFetchResult, error)
```

Keep `ListChildrenRecursive` and `ListChildrenRecursiveWithDepth` as compatibility wrappers with `ResolveMirrors: true`. Add the option-bearing method to `workflowy.Client`. For a non-root `itemID`, `ListChildrenRecursiveWithOptions` retrieves the root node internally when `RootItem` is nil; when `RootItem` is non-nil but its ID differs from `itemID`, return `Cannot recursively fetch Workflowy node <itemID>: supplied root item has ID <rootID>`. CLI and MCP handlers pass their already-retrieved item to avoid the extra request.

- [ ] **Step 4: Write recursive observability tests before traversal implementation**

Add `TestRecursiveFetchReturnsResolutionSummary`, `TestRecursiveFetchLogsOneTraversalSummary`, `TestRecursiveFetchLogsContextualMalformedFallback`, `TestRecursiveFetchLogsContextualCycle`, and `TestRecursiveFetchWarnsWhenAtLeastHalfOfAttemptsFail`. Capture `slog` records and assert the same count attributes and equations as `ResolvedTree`. Assert exactly one info summary for a public recursive call, individual malformed/cycle warnings contain item/origin/path context, and the threshold warning fires at 50% but not below it.

- [ ] **Step 5: Run recursive observability tests and verify failure**

Run: `go test ./pkg/workflowy -run 'TestRecursiveFetch(Returns|Logs|Warns)' -count=1`

Expected: FAIL because option-bearing traversal and its instrumentation are not implemented.

- [ ] **Step 6: Implement metadata checks and active-path state**

Start the path with `RootItem` when the caller already retrieved a requested node. When non-root `RootItem` is nil, assign the internally retrieved node to the local root and seed the path exactly as if the caller supplied it. Root-list calls begin one path per returned top-level item. When resolution is disabled, do not list a recognized mirror's children. When enabled and a mirror has an available origin, reject it if the origin is already active; otherwise add that origin ID to a branch-local copy of the active path before listing and recursing beneath the mirror. Let the selected API's list response supply children, and discard the copied path when that branch returns. Return and log the same summary fields as local traversal.

The compatibility wrappers may perform one additional `GetItem` for non-root IDs because they do not receive a `RootItem`; record this in their Go documentation.

- [ ] **Step 7: Update test clients for the expanded interface**

Add forwarding or recording implementations of `ListChildrenRecursiveWithOptions` to the fake clients in the files listed above. Do not weaken `workflowy.Client` with type assertions or optional runtime capability checks.

- [ ] **Step 8: Run all unit tests**

Run: `just test`

Expected: PASS.

- [ ] **Step 9: Commit direct API traversal**

```bash
git add pkg/workflowy/recursive_fetch.go pkg/workflowy/recursive_fetch_test.go pkg/workflowy/workflowy.go pkg/workflowy/interfaces.go pkg/workflowy/workflowy_test.go cmd/workflowy/backup_api_test.go cmd/workflowy/client_test.go cmd/workflowy/count_report_test.go pkg/mcp/api_selection_test.go pkg/mcp/tools_test.go
git commit -m "feat: control mirror resolution for API reads"
```

## Chunk 2: Get, list, and search interface parity

### Task 4: Wire mirror resolution into CLI get and list

**Files:**
- Modify: `cmd/workflowy/flags.go`
- Modify: `cmd/workflowy/flags_test.go`
- Modify: `cmd/workflowy/invocation.go`
- Modify: `cmd/workflowy/fetch.go`
- Modify: `cmd/workflowy/fetch_test.go`
- Modify: `cmd/workflowy/read_guard.go`
- Create: `cmd/workflowy/mirror_fetch_test.go`
- Modify: `cmd/workflowy/transform_test.go`

- [ ] **Step 1: Write flag and default tests**

Assert `getFetchFlags()` contains a `resolve-mirrors` boolean flag with value `true`, and urfave accepts both `--resolve-mirrors=false` and `--resolve-mirrors=true`. Extend `FetchParameters` with `resolveMirrors bool` and assert parsing.

- [ ] **Step 2: Run the flag tests and verify failure**

Run: `go test ./cmd/workflowy -run 'Test.*ResolveMirrors.*Flag|TestGetAndValidateFetchParams' -count=1`

Expected: FAIL because the flag is absent.

- [ ] **Step 3: Add the shared CLI fetch flag**

Add this command-local flag through `getFetchFlags` and reuse the same constructor from search:

```go
func getResolveMirrorsFlag() cli.Flag {
	return &cli.BoolFlag{
		Name:  "resolve-mirrors",
		Value: true,
		Usage: "Resolve mirror descendants",
	}
}
```

- [ ] **Step 4: Write export, backup, access, and API handler tests**

Add CLI-level tests covering get and list defaults, explicit false, export mirror children, backup `originalId`, direct API request suppression, a descendant reachable only through a mirror beneath `read-root-id`, preferred original occurrence, first reachable mirror fallback, and fresh output across two invocations. Add `TestCommandInvocationRestrictedGetDoesNotTraverseUnrelatedMirrorBranches` with a restricted target followed by an unrelated mirror cycle inside the same read scope; assert validation succeeds without observing that cycle. For restricted direct GET, assert read-root validation and target selection share exactly one selected-deployment export snapshot. Add ancestor assertions proving a mirror-only reachable child receives its mirror path while an in-scope original wins when available. Add `TestTransformDoesNotTraverseResolvedMirrorChildren` before changing handlers: a dry-run transform at a mirror must omit origin descendants outside its source subtree. Add `TestGetMirrorThresholdWarningWritesStderr` and `TestGetMirrorBelowThresholdDoesNotWriteWarning`, capturing the command's log writer at exactly 50% and below it. Assert errors start with `Cannot` and contain target ID, read scope, and source.

Add a counting export/backup provider and assert each restricted export or backup get/list invocation loads its source tree exactly once for ID resolution, read-scope validation, occurrence selection, and result materialization.

- [ ] **Step 5: Run the new command tests and verify failure**

Run: `go test ./cmd/workflowy -run 'Test((Get|List).*Mirror|CommandInvocationRestrictedGetDoesNotTraverseUnrelatedMirrorBranches|TransformDoesNotTraverseResolvedMirrorChildren)' -count=1`

Expected: FAIL because fetch still reads and depth-limits the source tree.

- [ ] **Step 6: Route fetches through the correct traversal**

Add a distinct `commandInvocation.fetchResolvedItems`; do not change the source-tree `fetchItems` path used by transforms. Return a focused value:

```go
type mirrorFetchResult struct {
	Value       any
	SourceLabel string
	Summary     workflowy.MirrorResolutionSummary
}
```

Cache an invocation-scoped `sourceItems` and `sourceLabel` in `commandInvocation`. ID resolution, `ReadGuard`, and resolved fetching must use that same snapshot. For export and backup, construct `ResolvedTree` and call `Fetch`; do not call `FindItemInTree` or `LimitItemsDepth`. For GET, load the selected deployment export only when a read restriction requires reachability validation, validate the requested target through `ResolvedTree.Fetch` at depth zero, retrieve a non-root node once, then call `ListChildrenRecursiveWithOptions` using that item as `RootItem` and the CLI flag. Access-only validation must never enumerate the complete read scope.

Make `ReadGuard` expose the resolved scope ID without independently rejecting a target by source-tree ancestry. Keep its existing source-tree validation methods for reports, transforms, and writes. For export/backup ancestor options, build the spine from the selected occurrence's ancestors and fresh materialized target, applying `ancestor-depth` and `to-ancestor` there. Keep `runTransform` on `invocation.fetchItems`; only get/list call `fetchResolvedItems`.

- [ ] **Step 7: Run CLI tests**

Run: `go test ./cmd/workflowy -count=1`

Expected: PASS.

- [ ] **Step 8: Commit CLI mirror-aware fetches**

```bash
git add cmd/workflowy/flags.go cmd/workflowy/flags_test.go cmd/workflowy/invocation.go cmd/workflowy/fetch.go cmd/workflowy/fetch_test.go cmd/workflowy/read_guard.go cmd/workflowy/mirror_fetch_test.go cmd/workflowy/transform_test.go
git commit -m "feat: resolve mirrors in CLI fetch commands"
```

### Task 5: Wire mirror resolution and warnings into MCP get and list

**Files:**
- Create: `pkg/mcp/mirror_results.go`
- Create: `pkg/mcp/mirror_results_test.go`
- Create: `pkg/mcp/mirror_fetch.go`
- Create: `pkg/mcp/mirror_fetch_test.go`
- Modify: `pkg/mcp/tools.go`
- Modify: `pkg/mcp/tools_test.go`

- [ ] **Step 1: Write MCP schema and warning-result tests**

Assert `workflowy_get` and `workflowy_list` expose boolean `resolve_mirrors` with default true. Test a helper that returns the existing single JSON content block below threshold and prepends one text warning block at or above threshold.

```go
func newMirrorAwareToolResultJSON(value any, summary workflowy.MirrorResolutionSummary, operation, source string) (*mcptypes.CallToolResult, error)
```

The warning text must equal `summary.ThresholdWarning(operation, source)` and the JSON block must be byte-for-byte equivalent to `mcptypes.NewToolResultJSON(value)`.

- [ ] **Step 2: Run schema/result tests and verify failure**

Run: `go test ./pkg/mcp -run 'Test.*ResolveMirrors|TestMirrorAwareToolResult' -count=1`

Expected: FAIL because the schemas and helper are absent.

- [ ] **Step 3: Add MCP properties and warning-result delivery**

Add the `resolve_mirrors` schema property with default true to get and list and read it in their handlers. Implement only `newMirrorAwareToolResultJSON` in this step so warning content is testable before resolved-fetch handler integration. Do not yet replace handler traversal.

- [ ] **Step 4: Write MCP behavior and concurrency tests**

In `mirror_fetch_test.go`, cover export, backup, and GET; true and false; mirror-only read access; malformed/missing/cycle threshold warnings; and two parallel MCP calls selecting different APIs and resolution values. Add `TestToolBuilderRestrictedGetDoesNotTraverseUnrelatedMirrorBranches` with the same bounded access-validation assertion as CLI. For restricted direct GET, assert read-root validation and target selection share exactly one selected-deployment export snapshot. For ancestor-enabled get, assert a target reachable only through a mirror is allowed and its returned spine uses the chosen mirror occurrence; assert original preference when both paths are in scope. Use counting clients/providers to assert restricted export and backup get/list load one source snapshot. Assert no invocation tree, summary, or option leaks between calls.

- [ ] **Step 5: Run MCP behavior tests and verify failure**

Run: `go test ./pkg/mcp -run 'Test(.*Mirror|ToolBuilderRestrictedGetDoesNotTraverseUnrelatedMirrorBranches)' -count=1`

Expected: FAIL because resolved fetching, bounded direct-GET validation, snapshot reuse, and ancestor routing are not implemented.

- [ ] **Step 6: Implement resolved fetching in the focused file**

In `mirror_fetch.go`, add `ToolBuilder.fetchResolvedItems`, returning value, source label, and summary while leaving the existing source-tree `fetchItems` for transform/write tools. Pass `resolve_mirrors` through local `ResolvedTree` and direct recursive fetch. Make `prepareInvocation` cache one invocation-scoped export or backup source snapshot when get/list needs local resolution or read-scope validation. `validateReadTarget`, occurrence selection, materialization, and `fetchItemWithAncestors` must reuse that snapshot rather than call `loadTree` independently. Route `fetchItemWithAncestors` through the occurrence selected by `ResolvedTree` so it returns that occurrence's ancestor spine. Use `ResolvedTree.Fetch` at depth zero for restricted direct-GET validation and `newMirrorAwareToolResultJSON` for get/list responses.

- [ ] **Step 7: Run MCP tests with races**

Run: `go test -race ./pkg/mcp -count=1`

Expected: PASS with no races.

- [ ] **Step 8: Commit MCP fetch parity**

```bash
git add pkg/mcp/mirror_results.go pkg/mcp/mirror_results_test.go pkg/mcp/mirror_fetch.go pkg/mcp/mirror_fetch_test.go pkg/mcp/tools.go pkg/mcp/tools_test.go
git commit -m "feat: resolve mirrors in MCP fetch tools"
```

### Task 6: Search resolved occurrences once or at every mirror

**Files:**
- Create: `pkg/search/occurrences.go`
- Create: `pkg/search/occurrences_test.go`
- Modify: `pkg/search/search.go`
- Modify: `pkg/search/search_test.go`
- Modify: `pkg/search/tree.go`
- Modify: `pkg/search/tree_test.go`
- Modify: `cmd/workflowy/flags.go`
- Modify: `cmd/workflowy/flags_test.go`
- Modify: `cmd/workflowy/commands.go`
- Modify: `cmd/workflowy/search.go`
- Create: `cmd/workflowy/mirror_search_test.go`
- Modify: `pkg/mcp/tools.go`
- Modify: `pkg/mcp/tools_test.go`

- [ ] **Step 1: Write failing incremental-matcher tests**

Define an incremental matcher that receives one occurrence at a time. Test:

- non-matches are discarded without taking an occurrence snapshot;
- default deduplication uses `occurrence.EqualityID()`;
- a later original occurrence replaces an earlier mirror occurrence;
- the first mirror remains when no original is in scope;
- `SearchMirrors=true` retains every matching occurrence;
- completed ancestry filters only that occurrence path;
- a completed occurrence cannot suppress a later open occurrence;
- source-priority order remains stable;
- flat, parent/path/date grouped, and tree views contain the same selected matches;
- `SearchMirrors=true` keeps repeated Workflowy node IDs on distinct tree branches while serialized IDs remain unchanged.

Use:

```go
type OccurrenceSearchOptions struct {
	UseRegexp        bool
	IgnoreCase       bool
	IncludeCompleted bool
	SearchMirrors    bool
}

type OccurrenceMatcher struct {
	// private matcher and retained-match state
}

func NewOccurrenceMatcher(pattern string, options OccurrenceSearchOptions) *OccurrenceMatcher
func (matcher *OccurrenceMatcher) Visit(workflowy.NodeOccurrence) (workflowy.OccurrenceVisitResult, error)
func (matcher *OccurrenceMatcher) Matches() OccurrenceMatches

type OccurrenceMatches struct {
	matches []occurrenceMatch
}

func (matches OccurrenceMatches) Results() []Result
func (matches OccurrenceMatches) Grouped(strategy GroupingStrategy) []GroupedResult
func (matches OccurrenceMatches) Tree() []*TreeNode
```

`OccurrenceMatcher.Visit` snapshots an occurrence only when retaining it as a match. Do not add a public function that accepts a complete occurrence slice; no such compatibility interface exists at the restart point.

- [ ] **Step 2: Run matcher tests and verify failure**

Run: `go test ./pkg/search -run 'Test(OccurrenceMatcher|OccurrenceMatches)' -count=1`

Expected: FAIL because the incremental occurrence matcher does not exist.

- [ ] **Step 3: Implement matching, deduplication, and view adapters once**

Implement pattern evaluation and deduplication in `occurrences.go`. Keep `occurrenceMatch` private. When a match is selected, retain `occurrence.Snapshot()` so later traversal stack reuse cannot alter its ancestors. For tree output, key internal branches by `occurrence.Identity()`; never key repeated paths only by Workflowy node ID.

Keep existing source-tree `SearchItems`, `SearchItemsGroupedBy`, and `SearchItemsTree` behavior unchanged for non-mirror callers. Do not route source-tree compatibility functions through a new collecting occurrence layer.

- [ ] **Step 4: Run all search tests**

Run: `go test ./pkg/search -count=1`

Expected: PASS.

- [ ] **Step 5: Write CLI and MCP streaming-search tests**

Assert CLI search has `--resolve-mirrors=true|false` and `--search-mirrors`; MCP search has `resolve_mirrors` default true and `search_mirrors` default false. Both interfaces must reject false/true with exactly:

```text
Cannot search mirror occurrences when mirror resolution is disabled
```

Add end-to-end handler tests for mirror-only content, default original preference, first-mirror fallback, all matching occurrences, every grouping mode, and resolved read-root scoping. Add `TestSearchMirrorThresholdWarningWritesStderr`, `TestSearchMirrorBelowThresholdDoesNotWriteWarning`, `TestSearchCommandStreamsResolvedOccurrences`, and `TestSearchToolStreamsResolvedOccurrences`. Capture debug completion logs and assert many visited non-matches produce only the selected retained-match count. Use counting providers to assert each search loads one source snapshot.

- [ ] **Step 6: Run handler tests and verify failure**

Run: `go test ./cmd/workflowy ./pkg/mcp -run 'Test(.*Search.*Mirror|SearchCommandStreamsResolvedOccurrences|SearchToolStreamsResolvedOccurrences)' -count=1`

Expected: FAIL until handlers stream `ResolvedTree` occurrences into the matcher.

- [ ] **Step 7: Route both interfaces directly through `ResolvedTree.Visit`**

Load export or backup source once into the invocation snapshot, construct one matcher, and pass `matcher.Visit` directly to `ResolvedTree.Visit`. Call exactly one match view after traversal. `search_mirrors` changes retained matches, not traversal: every occurrence path remains eligible because completed ancestry and cycle boundaries are path-local. Reject method `get` as today. Deliver threshold warnings through stderr for CLI and `newMirrorAwareToolResultJSON` for MCP. Remove independent source-tree target validation; resolved reachability is part of the same visit.

- [ ] **Step 8: Run affected packages with races**

Run: `go test -race ./pkg/workflowy ./pkg/search ./cmd/workflowy ./pkg/mcp -count=1`

Expected: PASS.

- [ ] **Step 9: Commit search occurrence support**

```bash
git add pkg/search/occurrences.go pkg/search/occurrences_test.go pkg/search/search.go pkg/search/search_test.go pkg/search/tree.go pkg/search/tree_test.go cmd/workflowy/flags.go cmd/workflowy/flags_test.go cmd/workflowy/commands.go cmd/workflowy/search.go cmd/workflowy/mirror_search_test.go pkg/mcp/tools.go pkg/mcp/tools_test.go
git commit -m "feat: search Workflowy mirror occurrences"
```

## Chunk 3: Mirror creation

### Task 7: Add the deployment-neutral mirror client operation

**Files:**
- Create: `pkg/workflowy/mirror_node.go`
- Create: `pkg/workflowy/mirror_node_test.go`
- Modify: `pkg/workflowy/interfaces.go`
- Modify: `cmd/workflowy/backup_api_test.go`
- Modify: `cmd/workflowy/client_test.go`
- Modify: `cmd/workflowy/count_report_test.go`
- Modify: `pkg/mcp/api_selection_test.go`
- Modify: `pkg/mcp/tools_test.go`

- [ ] **Step 1: Write request validation and HTTP contract tests**

Test `SetPosition` for empty, top, bottom, and invalid values. With `httptest.Server`, assert `POST /nodes/<escaped-id>/mirror`, JSON `parent_id`, omitted/default position, explicit position, response decoding, and preservation of actual non-2xx status/body errors.

- [ ] **Step 2: Run mirror client tests and verify failure**

Run: `go test ./pkg/workflowy -run 'TestMirrorNode' -count=1`

Expected: FAIL because mirror request/response and method are undefined.

- [ ] **Step 3: Implement the request, response, and endpoint**

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

Implement `SetPosition` via `ValidatePosition` and `WorkflowyClient.MirrorNode` via `wc.Do`. Do not inspect deployment or replace an API error with a locally invented 404.

- [ ] **Step 4: Extend the client interface and update test clients**

Add `MirrorNode` to `workflowy.Client`. Add explicit fake implementations to every listed test client; recording clients should retain the requested source ID and body for later CLI/MCP assertions.

- [ ] **Step 5: Run all unit tests**

Run: `just test`

Expected: PASS.

- [ ] **Step 6: Commit the client operation**

```bash
git add pkg/workflowy/mirror_node.go pkg/workflowy/mirror_node_test.go pkg/workflowy/interfaces.go cmd/workflowy/backup_api_test.go cmd/workflowy/client_test.go cmd/workflowy/count_report_test.go pkg/mcp/api_selection_test.go pkg/mcp/tools_test.go
git commit -m "feat: add Workflowy mirror client operation"
```

### Task 8: Add the CLI mirror command and effective-destination validation

**Files:**
- Create: `cmd/workflowy/mirror.go`
- Create: `cmd/workflowy/mirror_test.go`
- Create: `pkg/workflowy/mirror_mutation.go`
- Create: `pkg/workflowy/mirror_mutation_test.go`
- Modify: `cmd/workflowy/commands.go`
- Modify: `cmd/workflowy/flags.go`
- Modify: `cmd/workflowy/invocation.go`
- Modify: `cmd/workflowy/read_guard.go`
- Modify: `cmd/workflowy/write_guard.go`

- [ ] **Step 1: Write CLI contract tests**

Assert command name, two required positional arguments, `position`, `method`, `backup-file`, `api`, and credential flags. Cover missing source/parent, `None` destination, full ID, unique short ID, Workflowy URL, target key with API validation, top/bottom, invalid position, text output containing both IDs, and unchanged JSON response fields.

- [ ] **Step 2: Run command tests and verify failure**

Run: `go test ./cmd/workflowy -run 'TestMirrorCommand' -count=1`

Expected: FAIL because `getMirrorCommand` does not exist.

- [ ] **Step 3: Write failing mirror-mutation resolver tests**

In `pkg/workflowy/mirror_mutation_test.go`, write table-driven tests for ordinary nodes, source mirror, parent mirror, missing source, missing parent, missing referenced origin, null/malformed origin, restricted unresolved destination, proven conflict, and unprovable conflict. Assert the complete rich result or a contextual `Cannot ...` error, never a bare boolean.

- [ ] **Step 4: Run resolver tests and verify failure**

Run: `go test ./pkg/workflowy -run TestResolveMirrorMutation -count=1`

Expected: FAIL because the resolver and result type do not exist.

- [ ] **Step 5: Implement the resolver and validation-source helper**

Add one invocation helper that loads the selected validation tree once: backup for `method=backup`, selected deployment export for `get`, `export`, or auto. Resolve full/short/URL inputs against that tree. Use the selected API resolver for target keys only with API-backed validation; reject target keys contextually when backup is the selected validation source. Keep this source tree separate from `ResolvedTree`.

Add a pure shared domain resolver in `pkg/workflowy/mirror_mutation.go`:

```go
type MirrorMutationResolution struct {
	RequestedSourceID      string
	RequestedParentID      string
	EffectiveSourceID      string
	EffectiveDestinationID string
	EffectiveSourceKnown      bool
	EffectiveDestinationKnown bool
}

func ResolveMirrorMutation(
	items []*Item,
	sourceID, parentID, sourceLabel string,
	requireEffectiveDestination bool,
) (MirrorMutationResolution, error)
```

The resolver indexes the supplied validation tree. Missing requested source or parent always fails with `Cannot ...` plus the missing ID and source label. For an ordinary node, the effective ID is its requested ID and the corresponding `Effective...Known` field is true. For a mirror whose non-empty origin exists in the index, the effective ID is that origin and is known. For null, malformed, or referenced-but-absent origins, the effective ID remains the requested ID and the corresponding known field is false; this fails contextually only when `requireEffectiveDestination` is true.

If both effective IDs are known and equal, fail before the API call with source, requested parent, effective ID, and source label. If either side is unknown, do not invent a conflict; let the selected API decide. The request path always uses `RequestedSourceID` and the body always uses `RequestedParentID`, even when validation uses `EffectiveDestinationID`.

- [ ] **Step 6: Write access, load-count, and deployment-routing tests**

Cover missing source, missing requested parent, source outside `read-root-id`, ordinary destination outside `write-root-id`, a parent mirror inside the write root whose origin is outside, a parent mirror outside whose origin is inside, referenced effective origin absent with and without restriction, null/malformed destination origin, source-origin destination conflict when provable, unprovable conflict delegated to the API, `--api=production`, `--api=beta`, and actual API errors from either deployment. In parent-mirror cases, assert `MirrorNode` receives requested source and parent IDs, never effective validation IDs. Use a counting provider to assert each command loads exactly one validation snapshot even without access restrictions: backup for `method=backup`, and one selected-deployment export for get/export/auto. Assert every local error starts with `Cannot` and includes relevant IDs/source label; assert no local beta gate and no hardcoded status.

- [ ] **Step 7: Run access tests and verify failure**

Run: `go test ./cmd/workflowy -run 'TestMirrorCommand.*(Access|Destination|API|Error)' -count=1`

Expected: FAIL until validation and routing are implemented.

- [ ] **Step 8: Implement `getMirrorCommand`**

Register the command in `getCommands`. Resolve source and parent to full IDs, reject root, call `workflowy.ResolveMirrorMutation` on the invocation's single validation snapshot, validate requested source against the source-tree read scope and known effective destination against the source-tree write scope, then call `MirrorNode`. Wrap position validation with the supplied position and operation. Always wrap API failures with source ID, requested parent, effective destination, and selected deployment:

```go
return fmt.Errorf(
	"Cannot mirror Workflowy node %q into requested parent %q with effective destination %q using Workflowy API %q: %w",
	resolution.RequestedSourceID,
	resolution.RequestedParentID,
	resolution.EffectiveDestinationID,
	deployment,
	err,
)
```

Use early returns and keep the command in `mirror.go`, not the already-large `commands.go`.

- [ ] **Step 9: Run CLI tests**

Run: `go test ./cmd/workflowy -count=1`

Expected: PASS.

- [ ] **Step 10: Commit the CLI command**

```bash
git add cmd/workflowy/mirror.go cmd/workflowy/mirror_test.go pkg/workflowy/mirror_mutation.go pkg/workflowy/mirror_mutation_test.go cmd/workflowy/commands.go cmd/workflowy/flags.go cmd/workflowy/invocation.go cmd/workflowy/read_guard.go cmd/workflowy/write_guard.go
git commit -m "feat: create Workflowy mirrors from CLI"
```

### Task 9: Add the MCP mirror tool with matching options

**Files:**
- Create: `pkg/mcp/mirror_tool.go`
- Create: `pkg/mcp/mirror_tool_test.go`
- Modify: `pkg/mcp/tools.go`
- Modify: `pkg/mcp/tools_test.go`
- Modify: `pkg/mcp/server.go`
- Modify: `pkg/mcp/server_test.go`
- Modify: `pkg/mcp/api_selection_test.go`

- [ ] **Step 1: Write exact schema and expose-list tests**

Assert `ToolMirror == "workflowy_mirror"`; required string `id` and `parent_id`; optional enum `position` top/bottom; optional enum `method` get/export/backup; centralized enum `api` production/beta. Assert `mirror`, `workflowy_mirror`, `write`, and `all` expose it exactly once, while `read` does not.

- [ ] **Step 2: Run schema tests and verify failure**

Run: `go test ./pkg/mcp -run 'Test.*Mirror.*(Schema|Expose)' -count=1`

Expected: FAIL because the tool is not registered.

- [ ] **Step 3: Register the schema through existing dispatch**

Add the constant/factory/network-tool entries in `tools.go` and group/alias entries in `server.go`. Implement `func (b ToolBuilder) buildMirrorTool() mcpserver.ServerTool` in `mirror_tool.go` with the exact schema and a temporary handler that returns a contextual not-implemented tool error. Let `selectAPIHandler` add and process `api`; do not add a second selector inside the handler. Do not implement validation, loading, or mutation behavior in this step.

- [ ] **Step 4: Write handler, access, error, and concurrency tests**

Mirror the CLI cases for ID resolution, missing source, missing requested parent, missing referenced effective origin under restriction, source read restriction, effective destination write restriction, position, root rejection, backup validation, mirror-of-mirror response, proven and unprovable conflict, production/beta dispatch, actual API error preservation, and simultaneous calls using different APIs/methods. In parent-mirror cases, assert the client receives requested IDs rather than effective validation IDs. Assert local errors start with `Cannot` and contain source, requested-parent, effective-destination, deployment, and source-label context. Assert the JSON response contains `item_id` and the true `origin_id` returned by Workflowy.

Use counting clients/providers to prove one selected validation snapshot per call: `method=backup` reuses the prepared backup tree; `get`, `export`, and auto each use one export from the selected deployment for ID resolution, access checks, `ResolveMirrorMutation`, and conflict detection.

- [ ] **Step 5: Run behavior tests and verify failure**

Run: `go test ./pkg/mcp -run 'TestMirrorTool' -count=1`

Expected: FAIL until the handler uses the source-tree validation path and client operation.

- [ ] **Step 6: Implement the handler and mutation-specific snapshot preparation**

Make `prepareInvocation` load a validation snapshot for `workflowy_mirror` on every call, even when no read/write restriction is active: backup for `method=backup`, and one selected-deployment export for get/export/auto. Use `workflowy.ResolveMirrorMutation` with that snapshot and reuse source-tree access validators; do not copy metadata parsing into MCP. Validate the requested source ID and the known effective destination, but send the requested source/parent IDs to `MirrorNode`. Wrap position errors with operation and supplied value. Return `mcptypes.NewToolResultJSON(response)` on success. Wrap API failures with the same exact source, requested-parent, effective-destination, and deployment form used by CLI before converting them to a tool error.

- [ ] **Step 7: Run MCP tests with races**

Run: `go test -race ./pkg/mcp -count=1`

Expected: PASS.

- [ ] **Step 8: Commit MCP mirror creation**

```bash
git add pkg/mcp/mirror_tool.go pkg/mcp/mirror_tool_test.go pkg/mcp/tools.go pkg/mcp/tools_test.go pkg/mcp/server.go pkg/mcp/server_test.go pkg/mcp/api_selection_test.go
git commit -m "feat: create Workflowy mirrors from MCP"
```

## Chunk 4: Integration, documentation, and final validation

### Task 10: Add read-only and destructive mirror integration coverage

**Files:**
- Modify: `test/api_read.bats`
- Modify: `test/api_write.bats`
- Modify: `test/test_helper.bash`
- Modify: `justfile`

- [ ] **Step 1: Add fixture guards before test bodies**

Add helpers that skip read tests unless `TEST_MIRROR_ID` is set, and skip write tests unless both `TEST_MIRROR_SOURCE_ID` and `TEST_MIRROR_PARENT_ID` are set. Accept optional `TEST_MIRROR_EXPECTED_ORIGIN_ID` for a source fixture that is itself a mirror. Validate configured IDs are neither empty nor `None`; never infer a writable node from account data. Require `TEST_MIRROR_SOURCE_ID` and, when supplied, `TEST_MIRROR_EXPECTED_ORIGIN_ID` to be full UUIDs because the test compares them directly with the API's full `origin_id` response.

- [ ] **Step 2: Align just targets with read/write separation**

Change `test-integration` to run both `test/reports.bats` and `test/api_read.bats`. Retain `test-integration-write` exclusively for `test/api_write.bats`; do not add write tests to `test-all`. Extend the destructive-target warning to name `TEST_MIRROR_SOURCE_ID` and `TEST_MIRROR_PARENT_ID` alongside `TEST_PARENT_ID`.

- [ ] **Step 3: Add beta read tests**

In `api_read.bats`, compare descendant ID sets from `get --api=beta --method=get` and `get --api=beta --method=export` for `TEST_MIRROR_ID`. Add a third assertion that `--resolve-mirrors=false --depth=2 --format=json` returns the requested mirror with no `children`.

- [ ] **Step 4: Run the read-only target**

Run: `just test-integration`

Expected: PASS, with mirror tests skipped when `TEST_MIRROR_ID` is absent.

- [ ] **Step 5: Add the create-and-clean-up write test**

In `api_write.bats`, call `workflowy mirror "$TEST_MIRROR_SOURCE_ID" "$TEST_MIRROR_PARENT_ID" --api=beta --format=json`, capture `item_id` and `origin_id`, and immediately register the new ID in a beta-specific cleanup list before later assertions can fail. Teardown calls ordinary `workflowy delete "$item_id" --api=beta`. Assert both IDs are non-empty. For an ordinary source, assert `origin_id` equals the source ID; for a mirror source, require `TEST_MIRROR_EXPECTED_ORIGIN_ID` and assert the returned value equals it.

- [ ] **Step 6: Run Bats syntax and unit tests**

Run: `bash -n test/test_helper.bash && bats --count test/api_write.bats && bats --formatter tap test/api_read.bats`

Expected: helper syntax passes, the destructive file's tests are discovered without executing them, and read live-fixture tests either pass or report skips. Then run `just test` and expect PASS.

- [ ] **Step 7: Commit integration coverage**

```bash
git add test/api_read.bats test/api_write.bats test/test_helper.bash justfile
git commit -m "test: cover Workflowy mirror integration"
```

### Task 11: Document all interfaces and release behavior

**Files:**
- Modify: `README.md`
- Modify: `docs/CLI.md`
- Modify: `docs/MCP.md`
- Modify: `docs/architecture.md`
- Modify: `CHANGELOG.md`
- Create: `docs/blog/2026-08-04-workflowy-mirror-support.md`

- [ ] **Step 1: Update CLI and README examples**

Document `--resolve-mirrors=true|false` on get/list/search, `--search-mirrors`, the conflict, occurrence preference, `workflowy mirror <id> <parent-id>`, `--method`, `--position`, and shared `--api`. Include one beta example and one production example that explains the selected API's actual error is authoritative.

- [ ] **Step 2: Update MCP reference and inventory**

Document `resolve_mirrors` on get/list/search, `search_mirrors` on search, `workflowy_mirror` schema, `mirror` expose alias, write/all membership, warning content blocks, and ordinary delete as mirror removal. Ensure all schema names use snake_case.

- [ ] **Step 3: Synchronize architecture, observability, and changelog**

Verify `docs/architecture.md` describes the resolved tree as an on-demand view, target-directed fetch, streaming search, path-local cycles, and the inherent time cost of dense mirror graphs. Document debug index, traversal, retention, path-depth, elapsed-time, and Go heap/GC attributes in CLI and MCP troubleshooting sections. If implementation diverges materially, fix the implementation; if that is not possible, stop and request design approval instead of redefining the architecture in documentation. Add the new topmost `## [0.10.0] - Mirror Support` CHANGELOG entry covering mirror reads, search policy, mirror creation, defaults, observability, and no production gate.

- [ ] **Step 4: Write the feature release post**

Create `docs/blog/2026-08-04-workflowy-mirror-support.md` with motivation, source vs resolved trees, examples for all interfaces, safety/access behavior, observability warnings, beta availability today, and the metadata-driven production path. Do not claim undocumented HTTP statuses.

- [ ] **Step 5: Check terminology and links**

Run: `rg -n -i 'source tree|resolved tree' README.md docs CHANGELOG.md`

Expected: user-facing design documentation consistently uses the approved source-tree and resolved-tree terms. Then run this negative check for the three rejected terms:

```bash
for rejected_term in "canon""ical" "logi""cal" "projec""ted"; do
  if rg -n -i "$rejected_term" README.md docs CHANGELOG.md; then
    exit 1
  fi
done
git diff --check
```

Expected: no matches and no diff errors.

- [ ] **Step 6: Commit documentation**

```bash
git add README.md docs/CLI.md docs/MCP.md docs/architecture.md CHANGELOG.md docs/blog/2026-08-04-workflowy-mirror-support.md
git commit -m "docs: explain Workflowy mirror support"
```

### Task 12: Simplify, review, and verify the complete feature

**Files:**
- Modify: only files identified by the simplification and review findings

- [ ] **Step 1: Format and run the full unit suite**

Run: `git diff --name-only --diff-filter=ACMR 6aa849e -- '*.go' | xargs gofmt -w && just test`

Expected: PASS.

- [ ] **Step 2: Run the full race suite**

Run: `go test -race ./... -count=1`

Expected: PASS with no races.

- [ ] **Step 3: Run read-only integration tests**

Run: `just test-integration`

Expected: PASS; explicitly configured live mirror cases may run, otherwise skip. Do not run `just test-integration-write` without the dedicated user-provided fixtures.

- [ ] **Step 4: Run @code-simplifier on the feature diff**

Apply the code-simplifier skill to the complete feature working-tree diff from `6aa849e`, preserving behavior. Focus on duplicated mirror option plumbing, metadata parsing, occurrence selection, validation-tree loading, and warning-result construction. Re-run `just test` and `go test -race ./... -count=1` after any edit.

- [ ] **Step 5: Run @requesting-code-review against the design spec**

Review `git diff 6aa849e` against `docs/2026-08-04-mirror-support-design.md` and `docs/architecture.md`; this form includes committed feature work and current simplifier edits. Resolve all blocking Standards and Spec findings, then re-run focused tests for every touched package.

- [ ] **Step 6: Verify repository state and coverage-sensitive paths**

Run:

```bash
go test -race ./... -count=1
go test ./... -coverprofile=/tmp/workflowy-mirror-cover.out
go tool cover -func=/tmp/workflowy-mirror-cover.out | tail -1 | tee /tmp/workflowy-mirror-final-coverage.txt
baseline_coverage=$(awk '/^total:/{gsub("%", "", $3); print $3}' /tmp/workflowy-mirror-baseline-coverage.txt)
final_coverage=$(awk '/^total:/{gsub("%", "", $3); print $3}' /tmp/workflowy-mirror-final-coverage.txt)
test -n "$baseline_coverage"
test -n "$final_coverage"
awk -v baseline="$baseline_coverage" -v final="$final_coverage" 'BEGIN { if (final + 0 < baseline + 0) exit 1 }'
git diff --check
git status --short
```

Expected: race and coverage tests pass; the command exits nonzero if total coverage decreases; diff check is clean; only intentional feature files appear before the final cleanup commit.

- [ ] **Step 7: Commit every remaining tracked feature edit**

If formatting, simplification, or review left tracked working-tree changes, stage exactly the paths reported by the reviewed diff and commit them. Do not create new files in this final cleanup step.

```bash
if ! git diff --quiet; then
  git diff --name-only --diff-filter=ACMRD -z | xargs -0 git add --
  git commit -m "refactor: simplify Workflowy mirror support"
fi
```

If no tracked files changed, record that no final fixup commit was necessary in the execution handoff.

- [ ] **Step 8: Run final range and repository checks**

Run: `git diff --check 6aa849e..HEAD && git status --short`

Expected: the committed feature range has no whitespace errors and the working tree is clean.
