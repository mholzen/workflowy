# HTTP Debug Logging Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Emit a safe debug record for every successfully constructed HTTP request made by the shared Workflowy client.

**Architecture:** Keep HTTP diagnostics inside `pkg/client.Client.Do`, the seam crossed by CLI and MCP network operations. Let `slog` level filtering control visibility, remove the GET-only context plumbing, and document the same behavior for both interfaces.

**Tech Stack:** Go 1.24, `log/slog`, `net/http`, `testify`, urfave/cli, Markdown, `just`

---

## Chunk 1: Implement and document all-request HTTP debug logging

### Task 1: Make client tests describe unconditional debug logging

**Files:**
- Modify and add to Git: `pkg/client/client_test.go` (currently untracked)

- [ ] **Step 1: Remove the context opt-in from the debug-level test**

Call `Client.Do` with `context.Background()` and keep assertions for the debug level, message, method, and path:

```go
err := client.Do(context.Background(), method, "/nodes/test-id", nil, nil)
```

- [ ] **Step 2: Make the logger threshold explicit**

Replace the debug-only test helper with:

```go
func captureLogs(t *testing.T, level slog.Level) *bytes.Buffer {
	t.Helper()

	var output bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&output, &slog.HandlerOptions{Level: level}))
	previous := slog.Default()
	slog.SetDefault(logger)
	t.Cleanup(func() {
		slog.SetDefault(previous)
	})
	return &output
}
```

Use `slog.LevelDebug` in the emission test. Rename the suppression test to `TestClientDo_DoesNotLogHTTPRequestsAtInfoLevel`, use `slog.LevelInfo`, and loop over the same GET, POST, and DELETE cases so both level behaviors cover every supported method shape. Do not mark either test parallel because they replace the process-global logger.

- [ ] **Step 3: Run the focused test and verify RED**

Run: `go test ./pkg/client -run 'TestClientDo' -count=1`

Expected: FAIL because `Client.Do` still requires the removed context marker before submitting the record.

### Task 2: Centralize logging at the HTTP transport seam

**Files:**
- Modify: `pkg/client/client.go`
- Delete from the working tree: `pkg/client/context.go` (currently untracked, so no staged deletion is expected)
- Modify: `cmd/workflowy/fetch.go`
- Modify: `cmd/workflowy/fetch_test.go`

- [ ] **Step 1: Log every successfully constructed request**

Remove the context condition near the start of `Client.Do`. Immediately before `c.http.Do(req)`, add:

```go
slog.Debug("http request", "method", method, "path", path)

resp, err := c.http.Do(req)
```

Do not log the resolved URL, headers, authentication, request body, or response body.

- [ ] **Step 2: Remove the obsolete context interface**

Delete `pkg/client/context.go`, which contains `WithDebugHTTPLogging` and `DebugHTTPLoggingEnabled`.

- [ ] **Step 3: Remove GET-specific caller plumbing**

In `cmd/workflowy/fetch.go`, remove the `pkg/client` import and this line from the GET branch:

```go
apiCtx = clientpkg.WithDebugHTTPLogging(apiCtx)
```

- [ ] **Step 4: Remove the obsolete context-propagation test**

In `cmd/workflowy/fetch_test.go`, remove `TestFetchItems_GetMethod_EnablesHTTPDebugLoggingOnFetcherRequests`, `debugLoggingClient`, and the imports used only by them.

- [ ] **Step 5: Format and run focused tests to verify GREEN**

Run:

```bash
gofmt -w pkg/client/client.go pkg/client/client_test.go cmd/workflowy/fetch.go cmd/workflowy/fetch_test.go
go test ./pkg/client ./cmd/workflowy -count=1
```

Expected: both packages PASS.

- [ ] **Step 6: Commit the focused code change**

```bash
git add pkg/client/client.go pkg/client/client_test.go cmd/workflowy/fetch.go cmd/workflowy/fetch_test.go
git commit -m "feat: log all HTTP requests at debug level"
```

Only files with actual tracked changes will be included; the restored fetch files should match `HEAD`.

### Task 3: Document CLI and MCP troubleshooting behavior

**Files:**
- Modify: `docs/CLI.md`
- Modify: `docs/MCP.md`

- [ ] **Step 1: Add the CLI troubleshooting example**

Under `docs/CLI.md` → `Troubleshooting`, document that `workflowy get <id> --log=debug` shows the method and path for every API request and never logs API keys, authorization headers, or request bodies.

- [ ] **Step 2: Add the MCP logging example**

Under `docs/MCP.md` → `Logging and Debugging`, add this example and the same safety statement:

```text
DEBUG: http request (method='GET', path='/nodes/example')
```

- [ ] **Step 3: Commit documentation parity**

```bash
git add docs/CLI.md docs/MCP.md
git commit -m "docs: describe HTTP debug logging"
```

### Task 4: Verify the complete change

**Files:**
- Verify only; no planned file changes

- [ ] **Step 1: Check formatting and whitespace**

Run:

```bash
gofmt -w pkg/client/client.go pkg/client/client_test.go cmd/workflowy/fetch.go cmd/workflowy/fetch_test.go
git diff --check
```

Expected: no output from `git diff --check`.

- [ ] **Step 2: Run all unit tests**

Run: `just test`

Expected: all packages PASS, including `pkg/client`, `cmd/workflowy`, and `scripts`.

- [ ] **Step 3: Run static analysis**

Run: `go vet ./...`

Expected: exit status 0 with no diagnostics.

- [ ] **Step 4: Inspect final scope**

Run `git status --short` and `git log -5 --oneline`.

Expected: because this plan is committed before execution, only the pre-existing untracked curl helper and generated `.gocache/` remain outside the committed HTTP logging change.
