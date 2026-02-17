# Ancestor Depth & To-Ancestor Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `--ancestor-depth` and `--to-ancestor` options to `get`, giving fine-grained control over how many ancestors are included in the spine.

**Architecture:** Two new tree helper functions (`TruncateAncestors`, `SliceAncestorsTo`) process the ancestor chain before passing to the existing `BuildAncestorSpine`. A new `resolveAncestorOptions` function in fetch.go validates conflicts and resolves the three flags into a unified ancestor config. CLI and MCP both gain the two new parameters.

**Tech Stack:** Go, urfave/cli/v3, mcp-go, testify

---

### Task 1: TruncateAncestors - failing tests

**Files:**
- Modify: `pkg/workflowy/tree_test.go`

**Step 1: Write failing tests**

Append to `pkg/workflowy/tree_test.go`:

```go
func TestTruncateAncestors_KeepLastN(t *testing.T) {
	ancestors := []*Item{
		{ID: "a", Name: "A"},
		{ID: "b", Name: "B"},
		{ID: "c", Name: "C"},
	}

	result := TruncateAncestors(ancestors, 2)
	assert.Len(t, result, 2)
	assert.Equal(t, "b", result[0].ID)
	assert.Equal(t, "c", result[1].ID)
}

func TestTruncateAncestors_DepthExceedsLength(t *testing.T) {
	ancestors := []*Item{
		{ID: "a", Name: "A"},
		{ID: "b", Name: "B"},
	}

	result := TruncateAncestors(ancestors, 5)
	assert.Len(t, result, 2)
}

func TestTruncateAncestors_DepthMinusOne(t *testing.T) {
	ancestors := []*Item{
		{ID: "a", Name: "A"},
		{ID: "b", Name: "B"},
		{ID: "c", Name: "C"},
	}

	result := TruncateAncestors(ancestors, -1)
	assert.Len(t, result, 3)
}

func TestTruncateAncestors_DepthZero(t *testing.T) {
	ancestors := []*Item{
		{ID: "a", Name: "A"},
	}

	result := TruncateAncestors(ancestors, 0)
	assert.Nil(t, result)
}

func TestTruncateAncestors_EmptyAncestors(t *testing.T) {
	result := TruncateAncestors(nil, 3)
	assert.Nil(t, result)
}

func TestTruncateAncestors_DepthOne(t *testing.T) {
	ancestors := []*Item{
		{ID: "a", Name: "A"},
		{ID: "b", Name: "B"},
		{ID: "c", Name: "C"},
	}

	result := TruncateAncestors(ancestors, 1)
	assert.Len(t, result, 1)
	assert.Equal(t, "c", result[0].ID)
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./pkg/workflowy/ -run TestTruncateAncestors -v`
Expected: FAIL with "undefined: TruncateAncestors"

**Step 3: Commit**

```bash
git add pkg/workflowy/tree_test.go
git commit -m "test: add failing tests for TruncateAncestors"
```

---

### Task 2: TruncateAncestors - implementation

**Files:**
- Modify: `pkg/workflowy/tree.go`

**Step 1: Implement TruncateAncestors**

Add after `BuildAncestorSpine` in `pkg/workflowy/tree.go`:

```go
// TruncateAncestors returns the last depth elements of the ancestors slice.
// If depth is -1 or >= len(ancestors), returns the full slice.
// If depth is 0, returns nil.
func TruncateAncestors(ancestors []*Item, depth int) []*Item {
	if depth == 0 {
		return nil
	}
	if ancestors == nil {
		return nil
	}
	if depth < 0 || depth >= len(ancestors) {
		return ancestors
	}
	return ancestors[len(ancestors)-depth:]
}
```

**Step 2: Run tests**

Run: `go test ./pkg/workflowy/ -run TestTruncateAncestors -v`
Expected: All 6 tests PASS

**Step 3: Commit**

```bash
git add pkg/workflowy/tree.go
git commit -m "feat: add TruncateAncestors for limiting ancestor depth"
```

---

### Task 3: SliceAncestorsTo - failing tests

**Files:**
- Modify: `pkg/workflowy/tree_test.go`

**Step 1: Write failing tests**

Append to `pkg/workflowy/tree_test.go`:

```go
func TestSliceAncestorsTo_MiddleAncestor(t *testing.T) {
	ancestors := []*Item{
		{ID: "a", Name: "A"},
		{ID: "b", Name: "B"},
		{ID: "c", Name: "C"},
	}

	result, err := SliceAncestorsTo(ancestors, "b")
	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "b", result[0].ID)
	assert.Equal(t, "c", result[1].ID)
}

func TestSliceAncestorsTo_FirstAncestor(t *testing.T) {
	ancestors := []*Item{
		{ID: "a", Name: "A"},
		{ID: "b", Name: "B"},
	}

	result, err := SliceAncestorsTo(ancestors, "a")
	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "a", result[0].ID)
}

func TestSliceAncestorsTo_LastAncestor(t *testing.T) {
	ancestors := []*Item{
		{ID: "a", Name: "A"},
		{ID: "b", Name: "B"},
		{ID: "c", Name: "C"},
	}

	result, err := SliceAncestorsTo(ancestors, "c")
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "c", result[0].ID)
}

func TestSliceAncestorsTo_NotFound(t *testing.T) {
	ancestors := []*Item{
		{ID: "a", Name: "A"},
		{ID: "b", Name: "B"},
	}

	_, err := SliceAncestorsTo(ancestors, "z")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ancestor z not found")
}

func TestSliceAncestorsTo_EmptyAncestors(t *testing.T) {
	_, err := SliceAncestorsTo(nil, "a")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ancestor a not found")
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./pkg/workflowy/ -run TestSliceAncestorsTo -v`
Expected: FAIL with "undefined: SliceAncestorsTo"

**Step 3: Commit**

```bash
git add pkg/workflowy/tree_test.go
git commit -m "test: add failing tests for SliceAncestorsTo"
```

---

### Task 4: SliceAncestorsTo - implementation

**Files:**
- Modify: `pkg/workflowy/tree.go`

**Step 1: Implement SliceAncestorsTo**

Add after `TruncateAncestors` in `pkg/workflowy/tree.go`:

```go
// SliceAncestorsTo returns ancestors from the matching ancestorID onward.
// Returns an error if ancestorID is not found in the ancestors slice.
func SliceAncestorsTo(ancestors []*Item, ancestorID string) ([]*Item, error) {
	for i, a := range ancestors {
		if a.ID == ancestorID {
			return ancestors[i:], nil
		}
	}
	return nil, fmt.Errorf("ancestor %s not found in ancestor chain", ancestorID)
}
```

**Step 2: Run tests**

Run: `go test ./pkg/workflowy/ -run "TestTruncateAncestors|TestSliceAncestorsTo" -v`
Expected: All 11 tests PASS

**Step 3: Commit**

```bash
git add pkg/workflowy/tree.go
git commit -m "feat: add SliceAncestorsTo for slicing ancestor chain to a specific node"
```

---

### Task 5: CLI flags and ancestor option resolution

**Files:**
- Modify: `cmd/workflowy/flags.go`
- Modify: `cmd/workflowy/fetch.go`

**Step 1: Add new flags to getFetchFlags() in flags.go**

In `cmd/workflowy/flags.go`, add two new flags after the `include-ancestors` flag in `getFetchFlags()`. The flags block should become:

```go
&cli.BoolFlag{
	Name:  "include-ancestors",
	Usage: "Wrap result in ancestor path from root to target node (requires export or backup method)",
},
&cli.IntFlag{
	Name:  "ancestor-depth",
	Usage: "Include N levels of ancestors (-1 for all, 0 for none; requires export or backup method)",
},
&cli.StringFlag{
	Name:  "to-ancestor",
	Usage: "Include ancestors up to and including this node ID (requires export or backup method)",
},
```

**Step 2: Add ancestorOptions struct and resolveAncestorOptions to fetch.go**

In `cmd/workflowy/fetch.go`, add before the `fetchItems` function:

```go
type ancestorOptions struct {
	enabled       bool
	ancestorDepth int
	toAncestorID  string
}

func resolveAncestorOptions(cmd *cli.Command) (ancestorOptions, error) {
	includeAncestors := cmd.Bool("include-ancestors")
	ancestorDepth := cmd.Int("ancestor-depth")
	toAncestor := cmd.String("to-ancestor")

	optCount := 0
	if includeAncestors {
		optCount++
	}
	if ancestorDepth != 0 {
		optCount++
	}
	if toAncestor != "" {
		optCount++
	}

	if optCount > 1 {
		return ancestorOptions{}, fmt.Errorf("cannot combine ancestor options: use only one of --include-ancestors, --ancestor-depth, or --to-ancestor")
	}

	if includeAncestors {
		return ancestorOptions{enabled: true, ancestorDepth: -1}, nil
	}
	if ancestorDepth != 0 {
		return ancestorOptions{enabled: true, ancestorDepth: int(ancestorDepth)}, nil
	}
	if toAncestor != "" {
		return ancestorOptions{enabled: true, toAncestorID: toAncestor}, nil
	}

	return ancestorOptions{}, nil
}
```

**Step 3: Update fetchItems to use ancestorOptions**

Replace the `fetchItems` function. Key changes:
- Replace `includeAncestors := cmd.Bool("include-ancestors")` with `ancestorOpts, err := resolveAncestorOptions(cmd)` + error check
- Replace all references to `includeAncestors` with `ancestorOpts.enabled`
- Replace `--include-ancestors` in the method=get error with ancestor option wording
- Pass `ancestorOpts` to `fetchFromBackup` instead of `bool`
- In the export/backup ancestor branches, add the truncation/slicing logic after `FindItemWithAncestors`

The updated `fetchItems`:

```go
func fetchItems(cmd *cli.Command, apiCtx context.Context, client workflowy.Client, itemID string, depth int) (interface{}, error) {
	method := cmd.String("method")
	backupFile := cmd.String("backup-file")

	ancestorOpts, err := resolveAncestorOptions(cmd)
	if err != nil {
		return nil, err
	}

	if method != "" && method != "get" && method != "export" && method != "backup" {
		return nil, fmt.Errorf("method must be 'get', 'export', or 'backup'")
	}

	if ancestorOpts.enabled && method == "get" {
		return nil, fmt.Errorf("cannot use ancestor options with --method=get (requires full tree)")
	}

	var useMethod string
	if method != "" {
		useMethod = method
	} else if client == nil {
		useMethod = "backup"
	} else if ancestorOpts.enabled {
		useMethod = "export"
	} else {
		if depth == -1 || depth >= 4 {
			useMethod = "export"
		} else {
			useMethod = "get"
		}
	}

	if client == nil && (useMethod == "get" || useMethod == "export") {
		return nil, fmt.Errorf("cannot use method '%s' without using the API", useMethod)
	}

	slog.Debug("access method determined", "method", useMethod, "depth", depth, "ancestors_enabled", ancestorOpts.enabled)

	var result interface{}

	switch useMethod {
	case "backup":
		return fetchFromBackup(backupFile, itemID, depth, ancestorOpts)

	case "export":
		slog.Debug("using export API", "depth", depth)
		forceRefresh := cmd.Bool("force-refresh")
		response, err := client.ExportNodesWithCache(apiCtx, forceRefresh)
		if err != nil {
			if method == "" {
				slog.Warn("export failed, falling back to backup", "error", err)
				return fetchFromBackup(backupFile, itemID, depth, ancestorOpts)
			}
			return nil, fmt.Errorf("cannot export nodes: %w", err)
		}

		slog.Debug("reconstructing tree from export data")
		root := workflowy.BuildTreeFromExport(response.Nodes)

		if itemID != "None" {
			if ancestorOpts.enabled {
				found, ancestors := workflowy.FindItemWithAncestors(root.Children, itemID)
				if found == nil {
					return nil, fmt.Errorf("item %s not found", itemID)
				}
				ancestors, err = applyAncestorOptions(ancestors, ancestorOpts)
				if err != nil {
					return nil, err
				}
				result = workflowy.BuildAncestorSpine(found, ancestors, depth)
			} else {
				found := workflowy.FindItemInTree(root.Children, itemID, depth)
				if found == nil {
					return nil, fmt.Errorf("item %s not found", itemID)
				}
				result = found
			}
		} else {
			if depth >= 0 {
				slog.Debug("limiting depth for export results", "depth", depth, "item_count", len(root.Children))
				workflowy.LimitItemsDepth(root.Children, depth)
			}
			result = &workflowy.ListChildrenResponse{Items: root.Children}
		}

	case "get":
		slog.Debug("using GET API", "depth", depth)
		if depth < 0 {
			return nil, fmt.Errorf("depth must be non-negative when using GET API (use --method=export for depth=-1)")
		}

		var err error
		if itemID == "None" {
			slog.Debug("fetching root items", "depth", depth)
			result, err = client.ListChildrenRecursiveWithDepth(apiCtx, itemID, depth)
			if err != nil {
				if method == "" {
					slog.Warn("get API failed, falling back to backup", "error", err)
					return fetchFromBackup(backupFile, itemID, depth, ancestorOpts)
				}
				return nil, fmt.Errorf("cannot fetch root items: %w", err)
			}
		} else {
			slog.Debug("fetching item", "item_id", itemID, "depth", depth)
			item, err := client.GetItem(apiCtx, itemID)
			if err != nil {
				if method == "" {
					slog.Warn("get API failed, falling back to backup", "error", err)
					return fetchFromBackup(backupFile, itemID, depth, ancestorOpts)
				}
				return nil, fmt.Errorf("cannot get item: %w", err)
			}

			if depth > 0 {
				childrenResp, err := client.ListChildrenRecursiveWithDepth(apiCtx, itemID, depth)
				if err != nil {
					if method == "" {
						slog.Warn("get API failed fetching children, falling back to backup", "error", err)
						return fetchFromBackup(backupFile, itemID, depth, ancestorOpts)
					}
					return nil, fmt.Errorf("cannot fetch children: %w", err)
				}
				item.Children = childrenResp.Items
			}
			result = item
		}

	default:
		return nil, fmt.Errorf("unknown access method: %s", useMethod)
	}

	return result, nil
}

func applyAncestorOptions(ancestors []*workflowy.Item, opts ancestorOptions) ([]*workflowy.Item, error) {
	if opts.toAncestorID != "" {
		return workflowy.SliceAncestorsTo(ancestors, opts.toAncestorID)
	}
	return workflowy.TruncateAncestors(ancestors, opts.ancestorDepth), nil
}
```

**Step 4: Update fetchFromBackup**

```go
func fetchFromBackup(backupFile string, itemID string, depth int, ancestorOpts ancestorOptions) (interface{}, error) {
	items, err := loadFromBackupProvider(backupFile, workflowy.DefaultBackupProvider)
	if err != nil {
		return nil, err
	}

	if itemID != "None" {
		if ancestorOpts.enabled {
			found, ancestors := workflowy.FindItemWithAncestors(items, itemID)
			if found == nil {
				return nil, fmt.Errorf("item %s not found in backup", itemID)
			}
			ancestors, err = applyAncestorOptions(ancestors, ancestorOpts)
			if err != nil {
				return nil, err
			}
			return workflowy.BuildAncestorSpine(found, ancestors, depth), nil
		}
		found := workflowy.FindItemInTree(items, itemID, depth)
		if found == nil {
			return nil, fmt.Errorf("item %s not found in backup", itemID)
		}
		return found, nil
	}

	if depth >= 0 {
		workflowy.LimitItemsDepth(items, depth)
	}
	return &workflowy.ListChildrenResponse{Items: items}, nil
}
```

**Step 5: Run existing tests**

Run: `go test ./cmd/workflowy/ -v`
Expected: All tests PASS

**Step 6: Commit**

```bash
git add cmd/workflowy/flags.go cmd/workflowy/fetch.go
git commit -m "feat: add --ancestor-depth and --to-ancestor flags to CLI get command"
```

---

### Task 6: CLI tests for new ancestor options

**Files:**
- Modify: `cmd/workflowy/fetch_test.go`

**Step 1: Add conflict validation tests and method validation tests**

Append to `cmd/workflowy/fetch_test.go`:

```go
func TestFetchItems_AncestorDepth_WithGetMethod_ReturnsError(t *testing.T) {
	cmd := &cli.Command{
		Flags: getFetchFlags(),
		Action: func(ctx context.Context, c *cli.Command) error {
			_, err := fetchItems(c, ctx, nil, "some-id", 2)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "cannot use ancestor options with --method=get")
			return nil
		},
	}
	err := cmd.Run(context.Background(), []string{"test", "--method=get", "--ancestor-depth=2"})
	assert.NoError(t, err)
}

func TestFetchItems_ToAncestor_WithGetMethod_ReturnsError(t *testing.T) {
	cmd := &cli.Command{
		Flags: getFetchFlags(),
		Action: func(ctx context.Context, c *cli.Command) error {
			_, err := fetchItems(c, ctx, nil, "some-id", 2)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "cannot use ancestor options with --method=get")
			return nil
		},
	}
	err := cmd.Run(context.Background(), []string{"test", "--method=get", "--to-ancestor=abc"})
	assert.NoError(t, err)
}

func TestFetchItems_ConflictingAncestorOptions_ReturnsError(t *testing.T) {
	cmd := &cli.Command{
		Flags: getFetchFlags(),
		Action: func(ctx context.Context, c *cli.Command) error {
			_, err := fetchItems(c, ctx, nil, "some-id", 2)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "cannot combine ancestor options")
			return nil
		},
	}
	err := cmd.Run(context.Background(), []string{"test", "--include-ancestors", "--ancestor-depth=2"})
	assert.NoError(t, err)
}

func TestFetchItems_ConflictingAncestorAndToAncestor_ReturnsError(t *testing.T) {
	cmd := &cli.Command{
		Flags: getFetchFlags(),
		Action: func(ctx context.Context, c *cli.Command) error {
			_, err := fetchItems(c, ctx, nil, "some-id", 2)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "cannot combine ancestor options")
			return nil
		},
	}
	err := cmd.Run(context.Background(), []string{"test", "--include-ancestors", "--to-ancestor=abc"})
	assert.NoError(t, err)
}
```

**Step 2: Run the tests**

Run: `go test ./cmd/workflowy/ -run "TestFetchItems_Ancestor|TestFetchItems_ToAncestor|TestFetchItems_Conflicting" -v`
Expected: All PASS

**Step 3: Commit**

```bash
git add cmd/workflowy/fetch_test.go
git commit -m "test: add CLI tests for --ancestor-depth and --to-ancestor validation"
```

---

### Task 7: MCP tool changes

**Files:**
- Modify: `pkg/mcp/tools.go`

**Step 1: Add parameters to buildGetTool schema**

In `buildGetTool()`, after the `include_ancestors` parameter, add:

```go
mcptypes.WithNumber("ancestor_depth",
	mcptypes.Description("Include N levels of ancestors (-1 for all, 0 for none; requires export or backup method)"),
	mcptypes.DefaultNumber(0),
),
mcptypes.WithString("to_ancestor",
	mcptypes.Description("Include ancestors up to and including this node ID (requires export or backup method)"),
),
```

**Step 2: Update the handler**

Replace the handler in `buildGetTool()`. The key changes:
- Read `ancestorDepth` and `toAncestor` parameters
- Validate conflicts (same logic as CLI)
- Determine if ancestors are needed
- Update error message for method=get validation
- Pass ancestor options to `fetchItemWithAncestors`

The updated handler:

```go
Handler: func(ctx context.Context, req mcptypes.CallToolRequest) (*mcptypes.CallToolResult, error) {
	rawItemID := b.defaultReadID(req.GetString("id", "None"))
	depth := req.GetInt("depth", 2)
	includeEmpty := req.GetBool("include_empty_names", false)
	method := req.GetString("method", "")
	includeAncestors := req.GetBool("include_ancestors", false)
	ancestorDepth := req.GetInt("ancestor_depth", 0)
	toAncestor := req.GetString("to_ancestor", "")

	// Validate conflict
	optCount := 0
	if includeAncestors {
		optCount++
	}
	if ancestorDepth != 0 {
		optCount++
	}
	if toAncestor != "" {
		optCount++
	}
	if optCount > 1 {
		return mcptypes.NewToolResultError("cannot combine ancestor options: use only one of include_ancestors, ancestor_depth, or to_ancestor"), nil
	}

	// Resolve ancestor mode
	ancestorsEnabled := includeAncestors || ancestorDepth != 0 || toAncestor != ""
	if includeAncestors {
		ancestorDepth = -1
	}

	if ancestorsEnabled && b.resolveMethod(method) == "get" {
		return mcptypes.NewToolResultError("cannot use ancestor options with method=get (requires full tree)"), nil
	}

	itemID, err := workflowy.ResolveNodeID(ctx, b.client, rawItemID)
	if err != nil {
		return mcptypes.NewToolResultErrorFromErr("cannot resolve ID", err), nil
	}

	if err := b.validateReadTarget(ctx, itemID, "get", method); err != nil {
		return mcptypes.NewToolResultError(err.Error()), nil
	}

	if ancestorsEnabled {
		result, err := b.fetchItemWithAncestors(ctx, itemID, depth, method, ancestorDepth, toAncestor)
		if err != nil {
			return mcptypes.NewToolResultErrorFromErr("cannot get item with ancestors", err), nil
		}

		if !includeEmpty {
			result = workflowy.FilterEmptyItem(result)
		}

		return mcptypes.NewToolResultJSON(result)
	}

	result, err := b.fetchItems(ctx, itemID, depth, method)
	if err != nil {
		return mcptypes.NewToolResultErrorFromErr("cannot get item", err), nil
	}

	if !includeEmpty {
		switch v := result.(type) {
		case *workflowy.Item:
			result = workflowy.FilterEmptyItem(v)
		case *workflowy.ListChildrenResponse:
			result = workflowy.FilterEmptyList(v)
		}
	}

	return mcptypes.NewToolResultJSON(result)
},
```

**Step 3: Update fetchItemWithAncestors signature and body**

Update the method to accept `ancestorDepth` and `toAncestorID`:

```go
func (b ToolBuilder) fetchItemWithAncestors(ctx context.Context, itemID string, depth int, method string, ancestorDepth int, toAncestorID string) (*workflowy.Item, error) {
	useMethod := b.resolveMethod(method)
	if useMethod == "" || useMethod == "get" {
		useMethod = "export"
	}

	var tree []*workflowy.Item
	var err error

	switch useMethod {
	case "export":
		tree, err = b.loadExportTree(ctx, false)
	case "backup":
		tree, err = b.loadBackupTree()
	default:
		return nil, fmt.Errorf("cannot use method '%s' with ancestor options", useMethod)
	}
	if err != nil {
		return nil, err
	}

	found, ancestors := workflowy.FindItemWithAncestors(tree, itemID)
	if found == nil {
		return nil, fmt.Errorf("item %s not found", itemID)
	}

	if toAncestorID != "" {
		ancestors, err = workflowy.SliceAncestorsTo(ancestors, toAncestorID)
		if err != nil {
			return nil, err
		}
	} else {
		ancestors = workflowy.TruncateAncestors(ancestors, ancestorDepth)
	}

	return workflowy.BuildAncestorSpine(found, ancestors, depth), nil
}
```

**Step 4: Build and test**

Run: `go build ./...`
Expected: Success

Run: `go test ./...`
Expected: All tests PASS

**Step 5: Commit**

```bash
git add pkg/mcp/tools.go
git commit -m "feat: add ancestor_depth and to_ancestor parameters to MCP workflowy_get tool"
```

---

### Task 8: End-to-end verification

**Files:**
- None new

**Step 1: Run full test suite**

Run: `just test`
Expected: All tests PASS

**Step 2: Build binary**

Run: `just build`
Expected: Success

**Step 3: Verify CLI help**

Run: `./workflowy get --help`
Expected: Output includes `--include-ancestors`, `--ancestor-depth`, and `--to-ancestor` flags

**Step 4: Verify no regressions**

Run: `go vet ./...`
Expected: No issues
