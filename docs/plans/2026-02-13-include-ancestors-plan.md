# Include Ancestors Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `--include-ancestors` option to `get` that wraps the target node in its ancestor spine from root.

**Architecture:** Walk the full tree (export/backup) to find the target and collect ancestors. Build a spine tree where each ancestor has one child (the next in the path) and the target keeps its children per depth. Expose via CLI flag and MCP parameter.

**Tech Stack:** Go, urfave/cli/v3, mcp-go, testify

---

### Task 1: FindItemWithAncestors - failing tests

**Files:**
- Create: `pkg/workflowy/tree_test.go`

**Step 1: Write failing tests for FindItemWithAncestors**

```go
package workflowy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func makeTestTree() []*Item {
	return []*Item{
		{
			ID:   "a",
			Name: "A",
			Children: []*Item{
				{
					ID:   "b",
					Name: "B",
					Children: []*Item{
						{
							ID:       "c",
							Name:     "C",
							Children: []*Item{
								{ID: "d", Name: "D"},
							},
						},
					},
				},
				{ID: "e", Name: "E"},
			},
		},
		{ID: "f", Name: "F"},
	}
}

func TestFindItemWithAncestors_DeepNode(t *testing.T) {
	tree := makeTestTree()
	item, ancestors := FindItemWithAncestors(tree, "c")

	assert.NotNil(t, item)
	assert.Equal(t, "c", item.ID)
	assert.Len(t, ancestors, 2)
	assert.Equal(t, "a", ancestors[0].ID)
	assert.Equal(t, "b", ancestors[1].ID)
}

func TestFindItemWithAncestors_TopLevel(t *testing.T) {
	tree := makeTestTree()
	item, ancestors := FindItemWithAncestors(tree, "a")

	assert.NotNil(t, item)
	assert.Equal(t, "a", item.ID)
	assert.Empty(t, ancestors)
}

func TestFindItemWithAncestors_NotFound(t *testing.T) {
	tree := makeTestTree()
	item, ancestors := FindItemWithAncestors(tree, "nonexistent")

	assert.Nil(t, item)
	assert.Nil(t, ancestors)
}

func TestFindItemWithAncestors_LeafNode(t *testing.T) {
	tree := makeTestTree()
	item, ancestors := FindItemWithAncestors(tree, "d")

	assert.NotNil(t, item)
	assert.Equal(t, "d", item.ID)
	assert.Len(t, ancestors, 3)
	assert.Equal(t, "a", ancestors[0].ID)
	assert.Equal(t, "b", ancestors[1].ID)
	assert.Equal(t, "c", ancestors[2].ID)
}

func TestFindItemWithAncestors_SecondTopLevel(t *testing.T) {
	tree := makeTestTree()
	item, ancestors := FindItemWithAncestors(tree, "f")

	assert.NotNil(t, item)
	assert.Equal(t, "f", item.ID)
	assert.Empty(t, ancestors)
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./pkg/workflowy/ -run TestFindItemWithAncestors -v`
Expected: FAIL with "undefined: FindItemWithAncestors"

**Step 3: Commit failing tests**

```bash
git add pkg/workflowy/tree_test.go
git commit -m "test: add failing tests for FindItemWithAncestors"
```

---

### Task 2: FindItemWithAncestors - implementation

**Files:**
- Modify: `pkg/workflowy/tree.go`

**Step 1: Implement FindItemWithAncestors**

Add to `pkg/workflowy/tree.go`:

```go
// FindItemWithAncestors searches for targetID in the tree and returns the item
// along with its ancestor chain. ancestors[0] is the top-level parent,
// ancestors[len-1] is the immediate parent. Returns (nil, nil) if not found.
func FindItemWithAncestors(items []*Item, targetID string) (*Item, []*Item) {
	return findItemWithAncestors(items, targetID, nil)
}

func findItemWithAncestors(items []*Item, targetID string, ancestors []*Item) (*Item, []*Item) {
	for _, item := range items {
		if item.ID == targetID {
			return item, ancestors
		}
		if found, chain := findItemWithAncestors(item.Children, targetID, append(ancestors, item)); found != nil {
			return found, chain
		}
	}
	return nil, nil
}
```

**Step 2: Run tests to verify they pass**

Run: `go test ./pkg/workflowy/ -run TestFindItemWithAncestors -v`
Expected: All 5 tests PASS

**Step 3: Commit**

```bash
git add pkg/workflowy/tree.go
git commit -m "feat: add FindItemWithAncestors tree traversal"
```

---

### Task 3: BuildAncestorSpine - failing tests

**Files:**
- Modify: `pkg/workflowy/tree_test.go`

**Step 1: Write failing tests for BuildAncestorSpine**

Append to `pkg/workflowy/tree_test.go`:

```go
func TestBuildAncestorSpine_WithAncestors(t *testing.T) {
	tree := makeTestTree()
	item, ancestors := FindItemWithAncestors(tree, "c")

	spine := BuildAncestorSpine(item, ancestors, -1)

	// Spine root is a copy of "a"
	assert.Equal(t, "a", spine.ID)
	assert.Equal(t, "A", spine.Name)
	assert.Len(t, spine.Children, 1)

	// Next is copy of "b"
	b := spine.Children[0]
	assert.Equal(t, "b", b.ID)
	assert.Len(t, b.Children, 1)

	// Leaf is "c" with its original children
	c := b.Children[0]
	assert.Equal(t, "c", c.ID)
	assert.Len(t, c.Children, 1)
	assert.Equal(t, "d", c.Children[0].ID)
}

func TestBuildAncestorSpine_NoAncestors(t *testing.T) {
	tree := makeTestTree()
	item, ancestors := FindItemWithAncestors(tree, "a")

	spine := BuildAncestorSpine(item, ancestors, -1)

	// No ancestors, so the spine is the item itself
	assert.Equal(t, "a", spine.ID)
	assert.Len(t, spine.Children, 2) // original children preserved
}

func TestBuildAncestorSpine_DepthLimitsTargetChildren(t *testing.T) {
	tree := makeTestTree()
	item, ancestors := FindItemWithAncestors(tree, "c")

	spine := BuildAncestorSpine(item, ancestors, 0)

	// Walk to target
	c := spine.Children[0].Children[0]
	assert.Equal(t, "c", c.ID)
	// depth=0 means no children on target
	assert.Nil(t, c.Children)
}

func TestBuildAncestorSpine_AncestorsAreCopies(t *testing.T) {
	tree := makeTestTree()
	item, ancestors := FindItemWithAncestors(tree, "c")

	spine := BuildAncestorSpine(item, ancestors, -1)

	// Original "a" still has 2 children (b, e) -- spine copy has 1
	original := tree[0]
	assert.Len(t, original.Children, 2)
	assert.Len(t, spine.Children, 1)
}

func TestBuildAncestorSpine_PreservesMetadata(t *testing.T) {
	note := "test note"
	items := []*Item{
		{
			ID: "parent", Name: "Parent", Note: &note,
			CreatedAt: 1000, ModifiedAt: 2000,
			Children: []*Item{
				{ID: "child", Name: "Child"},
			},
		},
	}

	item, ancestors := FindItemWithAncestors(items, "child")
	spine := BuildAncestorSpine(item, ancestors, -1)

	assert.Equal(t, "parent", spine.ID)
	assert.Equal(t, "Parent", spine.Name)
	assert.Equal(t, &note, spine.Note)
	assert.Equal(t, int64(1000), spine.CreatedAt)
	assert.Equal(t, int64(2000), spine.ModifiedAt)
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./pkg/workflowy/ -run TestBuildAncestorSpine -v`
Expected: FAIL with "undefined: BuildAncestorSpine"

**Step 3: Commit failing tests**

```bash
git add pkg/workflowy/tree_test.go
git commit -m "test: add failing tests for BuildAncestorSpine"
```

---

### Task 4: BuildAncestorSpine - implementation

**Files:**
- Modify: `pkg/workflowy/tree.go`

**Step 1: Implement BuildAncestorSpine**

Add to `pkg/workflowy/tree.go`:

```go
// BuildAncestorSpine creates a spine tree from ancestors and target.
// Each ancestor is shallow-copied with only the path child in its Children.
// The target keeps its children, depth-limited by maxDepth.
// Returns the target as-is when ancestors is empty.
func BuildAncestorSpine(target *Item, ancestors []*Item, maxDepth int) *Item {
	if maxDepth >= 0 {
		LimitItemDepth(target, maxDepth)
	}

	if len(ancestors) == 0 {
		return target
	}

	// Build spine from bottom up: start with target, wrap in ancestor copies
	child := target
	for i := len(ancestors) - 1; i >= 0; i-- {
		ancestor := ancestors[i]
		copy := &Item{
			ID:          ancestor.ID,
			Name:        ancestor.Name,
			Note:        ancestor.Note,
			Priority:    ancestor.Priority,
			Data:        ancestor.Data,
			CreatedAt:   ancestor.CreatedAt,
			ModifiedAt:  ancestor.ModifiedAt,
			CompletedAt: ancestor.CompletedAt,
			Children:    []*Item{child},
		}
		child = copy
	}

	return child
}
```

**Step 2: Run tests to verify they pass**

Run: `go test ./pkg/workflowy/ -run "TestFindItemWithAncestors|TestBuildAncestorSpine" -v`
Expected: All tests PASS

**Step 3: Commit**

```bash
git add pkg/workflowy/tree.go
git commit -m "feat: add BuildAncestorSpine for wrapping nodes in ancestor path"
```

---

### Task 5: CLI flag and fetch.go changes

**Files:**
- Modify: `cmd/workflowy/flags.go`
- Modify: `cmd/workflowy/fetch.go`

**Step 1: Add `--include-ancestors` flag to getFetchFlags()**

In `cmd/workflowy/flags.go`, add to the `getFetchFlags()` slice (after the `include-empty-names` flag):

```go
&cli.BoolFlag{
	Name:  "include-ancestors",
	Usage: "Wrap result in ancestor path from root to target node (requires export or backup method)",
},
```

**Step 2: Update fetchItems in fetch.go**

The function signature stays the same. Add the `includeAncestors` parameter by reading it from `cmd`. The key changes:

1. At the top of `fetchItems`, read `includeAncestors := cmd.Bool("include-ancestors")`
2. In method auto-selection: if `includeAncestors` is true and method is unspecified, force `export`
3. If method is explicitly `get` and `includeAncestors` is true, return error
4. In the `export` and `backup` branches, when `itemID != "None"` and `includeAncestors` is true: use `FindItemWithAncestors` + `BuildAncestorSpine` instead of `FindItemInTree`

The modified `fetchItems` function in `cmd/workflowy/fetch.go`:

```go
func fetchItems(cmd *cli.Command, apiCtx context.Context, client workflowy.Client, itemID string, depth int) (interface{}, error) {
	method := cmd.String("method")
	backupFile := cmd.String("backup-file")
	includeAncestors := cmd.Bool("include-ancestors")

	if method != "" && method != "get" && method != "export" && method != "backup" {
		return nil, fmt.Errorf("method must be 'get', 'export', or 'backup'")
	}

	if includeAncestors && method == "get" {
		return nil, fmt.Errorf("cannot use --include-ancestors with --method=get (requires full tree)")
	}

	var useMethod string
	if method != "" {
		useMethod = method
	} else if client == nil {
		useMethod = "backup"
	} else if includeAncestors {
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

	slog.Debug("access method determined", "method", useMethod, "depth", depth, "include_ancestors", includeAncestors)

	var result interface{}

	switch useMethod {
	case "backup":
		return fetchFromBackup(backupFile, itemID, depth, includeAncestors)

	case "export":
		slog.Debug("using export API", "depth", depth)
		forceRefresh := cmd.Bool("force-refresh")
		response, err := client.ExportNodesWithCache(apiCtx, forceRefresh)
		if err != nil {
			if method == "" {
				slog.Warn("export failed, falling back to backup", "error", err)
				return fetchFromBackup(backupFile, itemID, depth, includeAncestors)
			}
			return nil, fmt.Errorf("cannot export nodes: %w", err)
		}

		slog.Debug("reconstructing tree from export data")
		root := workflowy.BuildTreeFromExport(response.Nodes)

		if itemID != "None" {
			if includeAncestors {
				found, ancestors := workflowy.FindItemWithAncestors(root.Children, itemID)
				if found == nil {
					return nil, fmt.Errorf("item %s not found", itemID)
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
					return fetchFromBackup(backupFile, itemID, depth, includeAncestors)
				}
				return nil, fmt.Errorf("cannot fetch root items: %w", err)
			}
		} else {
			slog.Debug("fetching item", "item_id", itemID, "depth", depth)
			item, err := client.GetItem(apiCtx, itemID)
			if err != nil {
				if method == "" {
					slog.Warn("get API failed, falling back to backup", "error", err)
					return fetchFromBackup(backupFile, itemID, depth, includeAncestors)
				}
				return nil, fmt.Errorf("cannot get item: %w", err)
			}

			if depth > 0 {
				childrenResp, err := client.ListChildrenRecursiveWithDepth(apiCtx, itemID, depth)
				if err != nil {
					if method == "" {
						slog.Warn("get API failed fetching children, falling back to backup", "error", err)
						return fetchFromBackup(backupFile, itemID, depth, includeAncestors)
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
```

**Step 3: Update fetchFromBackup to support includeAncestors**

```go
func fetchFromBackup(backupFile string, itemID string, depth int, includeAncestors bool) (interface{}, error) {
	items, err := loadFromBackupProvider(backupFile, workflowy.DefaultBackupProvider)
	if err != nil {
		return nil, err
	}

	if itemID != "None" {
		if includeAncestors {
			found, ancestors := workflowy.FindItemWithAncestors(items, itemID)
			if found == nil {
				return nil, fmt.Errorf("item %s not found in backup", itemID)
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

**Step 4: Run existing tests to verify no regressions**

Run: `go test ./cmd/workflowy/ -v`
Expected: All existing tests PASS

**Step 5: Commit**

```bash
git add cmd/workflowy/flags.go cmd/workflowy/fetch.go
git commit -m "feat: add --include-ancestors flag to CLI get command"
```

---

### Task 6: CLI fetch tests for include-ancestors

**Files:**
- Modify: `cmd/workflowy/fetch_test.go`

**Step 1: Add tests for include-ancestors method validation**

Append to `cmd/workflowy/fetch_test.go`:

```go
func TestFetchItems_IncludeAncestors_WithGetMethod_ReturnsError(t *testing.T) {
	flags := getFetchFlags()
	cmd := &cli.Command{
		Flags: flags,
		Action: func(ctx context.Context, c *cli.Command) error {
			_, err := fetchItems(c, ctx, nil, "some-id", 2)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "cannot use --include-ancestors with --method=get")
			return nil
		},
	}
	err := cmd.Run(context.Background(), []string{"test", "--method=get", "--include-ancestors"})
	assert.NoError(t, err)
}
```

**Step 2: Run the test**

Run: `go test ./cmd/workflowy/ -run TestFetchItems_IncludeAncestors -v`
Expected: PASS

**Step 3: Commit**

```bash
git add cmd/workflowy/fetch_test.go
git commit -m "test: add CLI tests for --include-ancestors method validation"
```

---

### Task 7: MCP tool changes

**Files:**
- Modify: `pkg/mcp/tools.go`

**Step 1: Add include_ancestors parameter to buildGetTool schema**

In `pkg/mcp/tools.go`, in the `buildGetTool()` method, add a new parameter to the tool schema (after the `method` parameter):

```go
mcptypes.WithBoolean("include_ancestors",
	mcptypes.Description("Wrap result in ancestor path from root to target node (requires export or backup method)"),
	mcptypes.DefaultBool(false),
),
```

**Step 2: Update the handler in buildGetTool**

In the handler function, read the new parameter and add validation + wrapping logic. The handler becomes:

```go
Handler: func(ctx context.Context, req mcptypes.CallToolRequest) (*mcptypes.CallToolResult, error) {
	rawItemID := b.defaultReadID(req.GetString("id", "None"))
	depth := req.GetInt("depth", 2)
	includeEmpty := req.GetBool("include_empty_names", false)
	method := req.GetString("method", "")
	includeAncestors := req.GetBool("include_ancestors", false)

	if includeAncestors && b.resolveMethod(method) == "get" {
		return mcptypes.NewToolResultError("cannot use include_ancestors with method=get (requires full tree)"), nil
	}

	itemID, err := workflowy.ResolveNodeID(ctx, b.client, rawItemID)
	if err != nil {
		return mcptypes.NewToolResultErrorFromErr("cannot resolve ID", err), nil
	}

	if err := b.validateReadTarget(ctx, itemID, "get", method); err != nil {
		return mcptypes.NewToolResultError(err.Error()), nil
	}

	if includeAncestors {
		result, err := b.fetchItemWithAncestors(ctx, itemID, depth, method)
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

**Step 3: Add fetchItemWithAncestors method to ToolBuilder**

Add near the existing `fetchItems` method in `pkg/mcp/tools.go`:

```go
// fetchItemWithAncestors loads the full tree and returns the target wrapped in its ancestor spine.
func (b ToolBuilder) fetchItemWithAncestors(ctx context.Context, itemID string, depth int, method string) (*workflowy.Item, error) {
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
		return nil, fmt.Errorf("cannot use method '%s' with include_ancestors", useMethod)
	}
	if err != nil {
		return nil, err
	}

	found, ancestors := workflowy.FindItemWithAncestors(tree, itemID)
	if found == nil {
		return nil, fmt.Errorf("item %s not found", itemID)
	}

	return workflowy.BuildAncestorSpine(found, ancestors, depth), nil
}
```

**Step 4: Run all tests**

Run: `go test ./... -v`
Expected: All tests PASS

**Step 5: Build to verify compilation**

Run: `go build ./cmd/workflowy`
Expected: Success, no errors

**Step 6: Commit**

```bash
git add pkg/mcp/tools.go
git commit -m "feat: add include_ancestors parameter to MCP workflowy_get tool"
```

---

### Task 8: End-to-end verification and cleanup

**Files:**
- None new

**Step 1: Run full test suite**

Run: `just test`
Expected: All tests PASS

**Step 2: Build binary**

Run: `just build`
Expected: Success

**Step 3: Verify CLI help shows the new flag**

Run: `./workflowy get --help`
Expected: Output includes `--include-ancestors` flag with description

**Step 4: Commit any final adjustments if needed**

If all looks good, no commit needed for this task.
