# Workflowy API Selection Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let CLI and MCP users select Workflowy's production or beta deployment while preserving production defaults, offline backup behavior, and deployment-isolated export caches.

**Architecture:** Introduce a validated `APIDeployment` domain value in `pkg/workflowy`, construct Workflowy clients from that value, and bind each client to an explicit export-cache path. CLI commands construct one selected client; MCP retains both clients and selects an immutable `ToolBuilder` copy per invocation before resolving restriction roots against the effective validation source.

**Tech Stack:** Go 1.24, urfave/cli v3, mcp-go v0.43, `log/slog`, testify, Bats, Markdown, `just`

---

## Chunk 1: Deployment domain, cache isolation, and CLI selection

### Task 1: Define and validate the API deployment domain

**Files:**
- Create: `pkg/workflowy/api_deployment.go`
- Create: `pkg/workflowy/api_deployment_test.go`

- [ ] **Step 1: Write failing deployment tests**

Cover these cases in `TestParseAPIDeployment`:

```go
tests := []struct {
	raw      string
	want     APIDeployment
	wantErr  string
}{
	{raw: "", want: ProductionAPI},
	{raw: "production", want: ProductionAPI},
	{raw: "beta", want: BetaAPI},
	{raw: "prod", wantErr: `Cannot select Workflowy API "prod": expected "production" or "beta"`},
}
```

Also assert that production and beta map to `https://workflowy.com/api/v1` and `https://beta.workflowy.com/api/v1`, and to distinct production-compatible cache filenames.

- [ ] **Step 2: Run RED**

Run: `GOCACHE=/private/tmp/workflowy-api-selection-cache go test ./pkg/workflowy -run 'TestParseAPIDeployment|TestAPIDeployment' -count=1`

Expected: FAIL because `APIDeployment`, its constants, parser, and mappings do not exist.

- [ ] **Step 3: Implement the closed deployment value**

Create:

```go
type APIDeployment string

const (
	ProductionAPI APIDeployment = "production"
	BetaAPI       APIDeployment = "beta"
)

func ParseAPIDeployment(raw string) (APIDeployment, error)
func (deployment APIDeployment) BaseURL() (string, error)
func (deployment APIDeployment) exportCacheFile() (string, error)
```

Parsing defaults an empty string to production. All invalid paths return the contextual `Cannot select Workflowy API ...` error. The production cache filename stays `.workflowy/export-cache.json`; beta uses `.workflowy/export-cache-beta.json`.

- [ ] **Step 4: Run GREEN**

Run the Step 2 command again.

Expected: PASS.

### Task 2: Make export-cache paths explicit and deployment-bound

**Files:**
- Create: `pkg/cache/export_test.go`
- Modify: `pkg/cache/export.go`
- Modify: `pkg/workflowy/workflowy.go`
- Modify: `pkg/workflowy/workflowy_test.go`
- Modify: `cmd/workflowy/commands.go`
- Modify: `pkg/mcp/server.go`

- [ ] **Step 1: Write failing cache isolation tests**

In `pkg/cache/export_test.go`, create two temporary paths, write different payloads through `WriteExportCache(path, payload)`, read them through `ReadExportCache(path)`, and assert neither path returns the other's data.

In `pkg/workflowy/workflowy_test.go`, set `HOME` to `t.TempDir()`, construct production and beta clients, and assert their internal `exportCachePath` fields are:

```text
<HOME>/.workflowy/export-cache.json
<HOME>/.workflowy/export-cache-beta.json
```

- [ ] **Step 2: Run RED**

Run: `GOCACHE=/private/tmp/workflowy-api-selection-cache go test ./pkg/cache ./pkg/workflowy -run 'TestExportCacheIsolation|TestNewWorkflowyClientForAPI' -count=1`

Expected: FAIL because cache functions do not accept paths and the API-aware constructor does not exist.

- [ ] **Step 3: Implement explicit cache paths**

Change the cache interface to:

```go
func GetCachePath(relativePath string) (string, error)
func ReadExportCache(cachePath string) (*ExportCache, error)
func WriteExportCache(cachePath string, data any) error
```

Add `exportCachePath string` to `WorkflowyClient` and deliberately replace the current no-error constructor with:

```go
func NewWorkflowyClient(deployment APIDeployment, opts ...client.Option) (*WorkflowyClient, error)
```

This is an intentional source-level API change: the repo has only the CLI and MCP constructor call sites, and preserving the no-error wrapper would either hide or panic on deployment/cache-path failures. In this same task, update both call sites to pass `ProductionAPI` explicitly and propagate constructor errors so the intermediate commit remains buildable; later tasks replace that fixed selection with user input. The constructor validates deployment mappings, resolves its cache path, constructs the generic client, and logs the selected deployment at debug level. `ExportNodesWithCache` passes the instance path to every cache read and write, including stale fallback and rate-limit checks.

- [ ] **Step 4: Run GREEN and verify the constructor migration repo-wide**

Run: `GOCACHE=/private/tmp/workflowy-api-selection-cache go test ./... -count=1`

Expected: PASS, including the CLI and MCP call sites changed for the new constructor signature.

- [ ] **Step 5: Commit the domain and cache slice**

```bash
git add pkg/cache/export.go pkg/cache/export_test.go pkg/workflowy/api_deployment.go pkg/workflowy/api_deployment_test.go pkg/workflowy/workflowy.go pkg/workflowy/workflowy_test.go cmd/workflowy/commands.go pkg/mcp/server.go
git commit -m "feat: add Workflowy API deployments"
```

### Task 3: Expose API selection across CLI commands

**Files:**
- Modify: `cmd/workflowy/flags.go`
- Modify: `cmd/workflowy/flags_test.go`
- Modify: `cmd/workflowy/commands.go`
- Create: `cmd/workflowy/client_test.go`
- Create: `cmd/workflowy/report_api_test.go`

- [ ] **Step 1: Write failing flag-parity tests**

Construct the real command tree and iterate the exact required command paths for this slice: get, list, create, update, move, delete, complete, uncomplete, targets, search, replace, transform, id, and every report subcommand. Assert each command exposes a string flag named `api` with default `production`; this specifically covers `move`, which has its own flag list. Assert both `mcp` (deferred to Task 4) and `version` do not yet expose the command-local flag.

- [ ] **Step 2: Write failing client-factory tests**

Define a test factory that records the `workflowy.APIDeployment` passed to it. With `WORKFLOWY_API_KEY` set, verify the factory-backed client provider selects beta, an empty value selects production, and an invalid value returns the exact contextual error without invoking the factory.

Add `getGetCommandWithClientProvider(provider)` as the same narrow command-construction seam already used by count reports. Execute the real get command with `--api=beta --method=get --depth=0` and assert the provider/fake client sees beta. Execute the optional-client get path with credentials absent and `--api=staging`; assert API validation fails before credential resolution and backup fallback, with zero factory calls.

Using the existing `getCountReportCommandWithDeps` seam and a factory-backed provider, execute `report count --method=backup --backup-file=<fixture> --upload --api=beta`. Assert the report tree is read from the fixture while its create/upload calls go only to the beta fake client. This proves `api` remains active when the read method is offline.

- [ ] **Step 3: Run RED**

Run: `GOCACHE=/private/tmp/workflowy-api-selection-cache go test ./cmd/workflowy -run 'Test.*API.*Flag|Test.*Client.*API|TestGetCommand.*API|TestCountReportBackupUploadUsesSelectedAPI' -count=1`

Expected: FAIL because the flag and factory seam do not exist.

- [ ] **Step 4: Implement the shared CLI flag**

Add:

```go
func getAPIFlag() *cli.StringFlag {
	return &cli.StringFlag{
		Name:  "api",
		Value: string(workflowy.ProductionAPI),
		Usage: "Workflowy API deployment: production or beta",
	}
}
```

Include it beside `api-key-file` in method, write, mirror-report, targets, ID, and the dedicated `move` flag list. Do not add it to MCP yet (Task 4 wires the flag and server config atomically), and do not add it to `version`.

- [ ] **Step 5: Implement the explicit CLI client-factory seam**

Introduce an internal factory function type mapping a validated deployment and client options to `workflowy.Client`. Make the factory-backed provider parse `cmd.String("api")` before resolving the API key, then call `NewWorkflowyClient`. Update `withClient` and `withOptionalClient` to use the provider. The optional provider may fall back to backup only after a valid deployment has been established and credential resolution fails; invalid deployments must fail first. Use `getGetCommandWithClientProvider` for the command-level test without introducing a broader dependency container.

- [ ] **Step 6: Run GREEN and all CLI tests**

Run: `GOCACHE=/private/tmp/workflowy-api-selection-cache go test ./cmd/workflowy -count=1`

Expected: PASS.

- [ ] **Step 7: Commit the CLI slice**

```bash
git add cmd/workflowy/flags.go cmd/workflowy/flags_test.go cmd/workflowy/commands.go cmd/workflowy/client_test.go cmd/workflowy/report_api_test.go
git commit -m "feat: select Workflowy API from CLI"
```

---

## Chunk 2: MCP selection, offline validation, and interface documentation

### Task 4: Select a deployment per MCP invocation

**Files:**
- Modify: `cmd/workflowy/commands.go`
- Modify: `pkg/mcp/server.go`
- Create: `pkg/mcp/server_test.go`
- Modify: `pkg/mcp/tools.go`
- Create: `pkg/mcp/api_selection_test.go`

- [ ] **Step 1: Write failing MCP CLI and startup tests**

Assert the real `workflowy mcp` command exposes `--api` with default `production` and passes it into `mcp.Config.API`.

Introduce injectable MCP credential-resolver and client-factory seams and test startup construction directly: invalid server API fails before credential resolution and calls both seams zero times; a valid API calls the credential resolver exactly once, supplies the returned bearer option to production and beta constructors, constructs both without making a network request, and records the configured default. Apply the option inside each recording factory call and inspect the resulting Authorization header rather than comparing option closures.

- [ ] **Step 2: Write failing exact tool-schema and dispatch tests**

Define one exact `networkToolNames` set using these 17 constants: `ToolGet`, `ToolList`, `ToolSearch`, `ToolTargets`, `ToolID`, `ToolCreate`, `ToolUpdate`, `ToolMove`, `ToolDelete`, `ToolComplete`, `ToolUncomplete`, `ToolReportCount`, `ToolReportChildren`, `ToolReportCreated`, `ToolReportModified`, `ToolReplace`, and `ToolTransform`. Build every MCP tool and assert each member has an `api` string enum containing exactly `production` and `beta`, and that every member's handler is routed through the deployment-selection wrapper. Assert `ToolReportMirrors` has neither the property nor the network wrapper.

Invoke `workflowy_get` with `method=get`, `depth=0`, and a root ID. Assert explicit `api=beta` calls only beta, omission uses the configured server default, and `api=staging` returns an MCP tool error without calling either client.

- [ ] **Step 3: Run RED**

Run: `GOCACHE=/private/tmp/workflowy-api-selection-cache go test ./cmd/workflowy ./pkg/mcp -run 'Test.*MCP.*API|Test.*Tool.*API|Test.*API.*Dispatch' -count=1`

Expected: FAIL because MCP has no flag/config/factory and ToolBuilder accepts only one client.

- [ ] **Step 4: Construct both MCP clients at startup**

Add `API string` to `mcp.Config`, add the CLI flag, and populate the field. Factor server setup so tests can inject a credential resolver plus a constructor mapping `APIDeployment` and client options to `workflowy.Client`. Parse the server default before calling the resolver, call the resolver once, then construct production and beta clients from that same option. Retain raw read/write root values and remove startup network resolution. Construction itself performs no HTTP operation.

- [ ] **Step 5: Centralize schema decoration and immutable handler wrapping**

Refactor the tool factory table to accept a `ToolBuilder` value. In `BuildTools`, use the exact `networkToolNames` set as the single authority: construct the base tool, add the shared `api` schema option, and replace its handler with a wrapper that parses per-call API, selects the matching client, logs the effective deployment, prepares an immutable invocation-specific builder copy, reconstructs the selected tool from that copy, and calls its handler. Never mutate a captured builder.

Build the mirror tool separately without the API option, but wrap it with a dedicated forced-backup invocation wrapper. That wrapper loads the configured/latest backup once, resolves roots locally, and passes the prepared builder to the mirror handler; it never selects or calls an API client.

This centralized loop must make it impossible to add schema without runtime dispatch or vice versa; its test checks the exact set against all constructed tools.

- [ ] **Step 6: Run GREEN and race-sensitive MCP tests**

Run:

```bash
GOCACHE=/private/tmp/workflowy-api-selection-cache go test ./cmd/workflowy ./pkg/mcp -count=1
GOCACHE=/private/tmp/workflowy-api-selection-cache go test -race ./pkg/mcp -count=1
```

Expected: PASS with no race report.

### Task 5: Make backup validation completely offline and deployment-relative

**Files:**
- Create: `pkg/workflowy/resolve.go`
- Create: `pkg/workflowy/resolve_test.go`
- Modify: `pkg/mcp/tools.go`
- Create: `pkg/mcp/tools_test.go`
- Modify: `pkg/mcp/api_selection_test.go`

- [ ] **Step 1: Write failing local-ID resolver tests**

Add tests for a tree-only resolver that verifies a full UUID, resolves one unique 12-character suffix, and returns contextual `Cannot resolve Workflowy node ...` errors for missing or ambiguous IDs. The resolver accepts only full/short IDs; target-key policy remains in MCP where the validation source is known.

- [ ] **Step 2: Write failing invocation-backup tests**

Inject a backup provider with call counters, an explicit backup filename, a known full UUID, and a second node resolvable by its 12-character suffix. Invoke tools using backup method and assert:

- the backup is loaded exactly once per invocation and stored as `invocationTree` with source label equal to the configured filename (or exactly `latest Workflowy backup` when none is configured);
- read/write roots and the exact request ID fields resolve from that same tree: `id` for get/list/search/id/update/move/delete/complete/uncomplete/all four network reports/transform; `parent_id` for create/move/replace; and `to_ancestor` for get before slicing the ancestor chain;
- a complete read-only backup invocation makes zero client calls;
- writes validate locally but send the mutation through the selected deployment client;
- target-key root `inbox` fails without network with `Cannot resolve read root "inbox" from backup "<label>": target keys require an API; configure read_root_id with a full or short node ID`.

- [ ] **Step 3: Write failing deployment-relative network tests**

Configure different node/root lookup results on production and beta fakes. Assert a beta override resolves target keys, short request IDs, restriction roots, and validation only through beta. Assert a beta root failure names the raw root and beta deployment, while a later production invocation succeeds. Assert a write whose validation method is backup still mutates only the selected beta client.

- [ ] **Step 4: Write failing mirror-report offline/scope tests**

Assert the mirror report uses the configured backup provider/file, loads once, makes zero client calls, and scopes results to the resolved read root. Cover full and short roots plus the target-key offline error. This replaces the current direct latest-backup read and prevents the report from bypassing configured roots.

- [ ] **Step 5: Run RED**

Run: `GOCACHE=/private/tmp/workflowy-api-selection-cache go test ./pkg/workflowy ./pkg/mcp -run 'Test.*Resolve.*Tree|Test.*Backup|Test.*RestrictionRoot|Test.*Mirror.*Root' -count=1`

Expected: FAIL because handlers still resolve IDs through the API and mirror reporting bypasses invocation state.

- [ ] **Step 6: Implement one invocation validation source**

Add the tree-only full/short resolver in `pkg/workflowy`. Extend the invocation builder with `invocationTree`, `invocationSourceLabel`, and resolved roots. At wrapper entry, derive the effective method and:

- for backup, load the configured file or latest backup exactly once, assign the exact source label, and resolve roots locally;
- for get/export/auto, resolve roots with the selected client and that deployment's export tree/cache.

Add builder methods for resolving the exact handler fields listed in Step 2. Replace every direct handler call to `workflowy.ResolveNodeID` and `ResolveNodeIDToUUID`, and resolve get's `to_ancestor` before ancestor slicing, so backup mode cannot accidentally perform HTTP. Target keys in backup return the contextual offline error; network target/short resolution uses only the selected client. Do not cache invocation state across calls.

Make `loadTree`, `loadTreeWithRefresh`, `loadBackupTree`, ancestor fetch, restriction validation, report loading, and mirror loading return/reuse the stored `invocationTree` whenever it is present. No code beneath either invocation wrapper may reread the backup provider.

Update mirror reporting to use the prepared backup invocation tree and restrict it to the resolved read-root subtree before collecting mirrors. It remains API-argument-free and offline.

- [ ] **Step 7: Run GREEN and all MCP tests**

Run:

```bash
GOCACHE=/private/tmp/workflowy-api-selection-cache go test ./pkg/workflowy ./pkg/mcp -count=1
GOCACHE=/private/tmp/workflowy-api-selection-cache go test -race ./pkg/mcp -count=1
```

Expected: PASS with no network calls in backup assertions and no race report.

- [ ] **Step 8: Commit the MCP slice**

```bash
git add cmd/workflowy/commands.go pkg/workflowy/resolve.go pkg/workflowy/resolve_test.go pkg/mcp/server.go pkg/mcp/server_test.go pkg/mcp/tools.go pkg/mcp/tools_test.go pkg/mcp/api_selection_test.go
git commit -m "feat: select Workflowy API per MCP call"
```

### Task 6: Document both interfaces and add read-only integration coverage

**Files:**
- Modify: `README.md`
- Modify: `docs/CLI.md`
- Modify: `docs/MCP.md`
- Modify: `test/api_read.bats`

- [ ] **Step 1: Update the README examples and cache notes**

In `Run Your First Command`, add these examples:

```bash
workflowy get --depth=0                         # production (default)
workflowy get --depth=0 --api=beta             # beta deployment
```

Immediately before `Data Access Methods`, add `## API Deployment` explaining that `--api` chooses production/beta while `--method` chooses get/export/backup transport. State that beta exposes mirror metadata such as `data.mirror.origin_id` that production does not currently expose, without claiming normalization. In `Data Access Methods`, list production cache `~/.workflowy/export-cache.json` and beta cache `~/.workflowy/export-cache-beta.json`.

- [ ] **Step 2: Update the complete CLI reference**

Immediately after `Global Options`, add a separate `API Selection` subsection explaining that `--api <production|beta>` is command-local, defaults to production, rejects other values, and is intentionally absent from `version`; do not put it in the global-options table or imply `workflowy --api=beta get` syntax. In each existing command `Options` table for get, list, create, update, move, delete, complete, uncomplete, targets, search, replace, and transform, add the option. Create missing `workflowy id` and `workflowy mcp` command-reference sections (and Table of Contents entries) with their usage/options, including `--api`. Add the same row to `Report Commands`' shared options table so count/children/created/modified/mirrors inherit it. Under `Data Access Methods`, add an `API Deployment vs. Access Method` subsection with:

```bash
workflowy get --api=beta --method=get --depth=0
workflowy report count --api=beta --method=backup --upload
```

Explain that the second reads/validates from backup but uploads through beta, and list the two deployment-specific cache paths.

- [ ] **Step 3: Update the complete MCP reference**

In `Client Configuration`, show `workflowy mcp --api=beta`. In every existing parameter table for the network tools named in Task 4, add optional `api` enum with the production/server default. Create a missing `workflowy_id` subsection and Table of Contents entry documenting its `id` and `api` parameters. Leave the `workflowy_report_mirrors` table unchanged and add a note that it is backup-only and intentionally has no `api`. Insert `## API Deployment` immediately before `Access Method`, documenting precedence `tool api > server --api > production`, API-versus-method semantics, backup full/short ID support and target-key limitation, and the two cache paths. Include this per-call example:

```json
{"id": "None", "depth": 0, "method": "get", "api": "beta"}
```

- [ ] **Step 4: Add opt-in read-only integration tests**

Immediately after the existing get tests, add these concrete read-only cases (the file's `setup` already enforces credentials):

```bash
@test "get root from production API" {
    require_jq
    run run_workflowy get --api=production --method=get --depth=0 --format=json --log=error
    [ "$status" -eq 0 ]
    assert_valid_json "$output"
}

@test "get root from beta API" {
    require_jq
    run run_workflowy get --api=beta --method=get --depth=0 --format=json --log=error
    [ "$status" -eq 0 ]
    assert_valid_json "$output"
}
```

- [ ] **Step 5: Run documentation and integration structure checks**

Run:

```bash
bats --count test/api_read.bats
git diff --check
```

Expected: Bats reports the discovered test count and whitespace checks pass. If Workflowy credentials and network access are available, additionally run `just test-integration-api`; otherwise record that the live check was skipped.

- [ ] **Step 6: Commit documentation parity**

```bash
git add README.md docs/CLI.md docs/MCP.md test/api_read.bats
git commit -m "docs: describe Workflowy API selection"
```

### Task 7: Verify and review the complete feature

**Files:**
- Verify all changed files; commit formatting/review fixes when needed

- [ ] **Step 1: Format before final verification**

Run `gofmt -w` on every changed Go file, inspect `git diff --check`, and commit any formatting changes with the slice they affect (or as a focused follow-up) before claiming a clean implementation.

- [ ] **Step 2: Run unit tests and static analysis**

Run:

```bash
just test
go vet ./...
```

Expected: all packages PASS and vet exits without diagnostics.

- [ ] **Step 3: Run race-sensitive client and MCP tests**

Run: `GOCACHE=/private/tmp/workflowy-api-selection-cache go test -race ./pkg/client ./pkg/workflowy ./pkg/mcp -count=1`

Expected: PASS with no race report.

- [ ] **Step 4: Run focused interface behavior checks**

Run the explicitly created behavior tests:

```bash
GOCACHE=/private/tmp/workflowy-api-selection-cache go test ./cmd/workflowy -run 'Test.*API.*Flag|Test.*Client.*API|TestGetCommand.*API|TestCountReportBackupUploadUsesSelectedAPI' -count=1
GOCACHE=/private/tmp/workflowy-api-selection-cache go test ./pkg/cache ./pkg/workflowy -run 'TestExportCacheIsolation|TestNewWorkflowyClientForAPI|TestParseAPIDeployment|Test.*Resolve.*Tree' -count=1
GOCACHE=/private/tmp/workflowy-api-selection-cache go test ./pkg/mcp -run 'Test.*MCP.*API|Test.*Tool.*API|Test.*API.*Dispatch|Test.*Backup|Test.*RestrictionRoot|Test.*Mirror.*Root' -count=1
```

Expected: the CLI suite includes move and every report in its exact flag-parity table plus the backup-read/beta-upload behavior; MCP covers schema/runtime parity, server/per-call precedence, offline IDs/roots, and mirror scoping; cache tests prove deployment isolation.

- [ ] **Step 5: Inspect final scope**

Run `git status --short` and `git log -12 --oneline`. Expected: only the pre-existing `.gocache/` and `scripts/` remain untracked; all API-selection source, tests, and documentation are committed on `feat/api-selection`.

- [ ] **Step 6: Request completion review**

Use the requesting-code-review workflow against the implementation range and `docs/superpowers/specs/2026-08-03-api-selection-design.md`. Fix every critical and important finding, rerun all verification, commit fixes, and obtain a ready-to-merge verdict.
