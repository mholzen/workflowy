package mcp

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	mcptypes "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/mholzen/workflowy/pkg/search"
	"github.com/mholzen/workflowy/pkg/workflowy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadToolsOnlyWrapResultsWhenPaginationIsRequested(t *testing.T) {
	backupFile := writeTestBackup(t)
	builder := NewToolBuilder(nil, "", "", "backup", backupFile)
	tests := []struct {
		name     string
		tool     mcpToolCase
		baseArgs map[string]any
	}{
		{
			name:     "get",
			tool:     mcpToolCase{serverTool: builder.buildGetTool()},
			baseArgs: map[string]any{"id": "None", "depth": 2},
		},
		{
			name:     "list",
			tool:     mcpToolCase{serverTool: builder.buildListTool()},
			baseArgs: map[string]any{"id": "None", "depth": 2},
		},
		{
			name:     "search",
			tool:     mcpToolCase{serverTool: builder.buildSearchTool()},
			baseArgs: map[string]any{"pattern": "a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unpaginated := tt.tool.call(t, tt.baseArgs)
			_, isPage := unpaginated.StructuredContent.(*workflowy.Page)
			assert.False(t, isPage, "legacy response should not gain pagination metadata")

			args := cloneArguments(tt.baseArgs)
			args["limit"] = float64(1)
			args["offset"] = float64(1)
			args["sort"] = "name"
			paginated := tt.tool.call(t, args)
			page, ok := paginated.StructuredContent.(*workflowy.Page)
			require.True(t, ok)
			assert.Equal(t, 1, page.Limit)
			assert.Equal(t, 1, page.Offset)
			assert.True(t, page.Total >= 2)
			assert.Len(t, page.Items, 1)
		})
	}
}

func TestSearchOrderByRemainsACompatibilityAlias(t *testing.T) {
	builder := NewToolBuilder(nil, "", "", "backup", writeTestBackup(t))
	tool := mcpToolCase{serverTool: builder.buildSearchTool()}
	result := tool.call(t, map[string]any{
		"pattern":  "a",
		"order_by": "-name",
	})

	structured, ok := result.StructuredContent.(map[string]any)
	require.True(t, ok)
	results, ok := structured["results"].([]search.Result)
	require.True(t, ok)
	require.Len(t, results, 4)
	assert.Equal(t, "Charlie child", results[0].Name)
	assert.Equal(t, "Charlie", results[1].Name)
}

func TestPaginatedGetNamesTheNodeItPages(t *testing.T) {
	builder := NewToolBuilder(nil, "", "", "backup", writeTestBackup(t))
	tool := mcpToolCase{serverTool: builder.buildGetTool()}

	result := tool.call(t, map[string]any{"id": "c", "depth": 2, "limit": float64(1)})

	page, ok := result.StructuredContent.(*workflowy.Page)
	require.True(t, ok)
	require.NotNil(t, page.Node, "a page of children is meaningless without the node it belongs to")
	assert.Equal(t, "c", page.Node.ID)
	assert.Equal(t, "Charlie", page.Node.Name)
}

func TestPaginatedListHasNoNodeToName(t *testing.T) {
	builder := NewToolBuilder(nil, "", "", "backup", writeTestBackup(t))
	tool := mcpToolCase{serverTool: builder.buildListTool()}

	result := tool.call(t, map[string]any{"id": "None", "depth": 2, "limit": float64(1)})

	page, ok := result.StructuredContent.(*workflowy.Page)
	require.True(t, ok)
	assert.Nil(t, page.Node)
}

func TestPaginationRejectsAnExplicitZeroLimit(t *testing.T) {
	builder := NewToolBuilder(nil, "", "", "backup", writeTestBackup(t))
	tool := mcpToolCase{serverTool: builder.buildListTool()}

	result := tool.call(t, map[string]any{"id": "None", "limit": float64(0)})

	assert.True(t, result.IsError, "zero is an empty page, not a request for the default limit")
}

func TestPaginationDefaultsToTheDefaultLimitWhenOnlyOffsetIsGiven(t *testing.T) {
	builder := NewToolBuilder(nil, "", "", "backup", writeTestBackup(t))
	tool := mcpToolCase{serverTool: builder.buildListTool()}

	result := tool.call(t, map[string]any{"id": "None", "offset": float64(0)})

	page, ok := result.StructuredContent.(*workflowy.Page)
	require.True(t, ok)
	assert.Equal(t, workflowy.DefaultPageLimit, page.Limit)
}

// The default sort is priority, which is a node's index among its own siblings.
// Search results span many parents, so the default must leave them in the order
// the outline produced rather than interleave them by sibling index.
func TestSearchDefaultSortKeepsOutlineOrder(t *testing.T) {
	builder := NewToolBuilder(nil, "", "", "backup", writeTestBackup(t))
	tool := mcpToolCase{serverTool: builder.buildSearchTool()}

	result := tool.call(t, map[string]any{"pattern": "a"})

	structured, ok := result.StructuredContent.(map[string]any)
	require.True(t, ok)
	results, ok := structured["results"].([]search.Result)
	require.True(t, ok)

	names := make([]string, len(results))
	for i, r := range results {
		names[i] = r.Name
	}
	assert.Equal(t, []string{"Charlie", "Charlie child", "Alpha", "Bravo"}, names)
}

func TestGetPaginationRejectsAncestorOptions(t *testing.T) {
	builder := NewToolBuilder(nil, "", "", "backup", filepath.Join(t.TempDir(), "unused.json"))
	tool := mcpToolCase{serverTool: builder.buildGetTool()}
	result := tool.call(t, map[string]any{
		"id":                "None",
		"limit":             float64(10),
		"include_ancestors": true,
	})
	assert.True(t, result.IsError)
}

type mcpToolCase struct {
	serverTool mcpserver.ServerTool
}

func (tc mcpToolCase) call(t *testing.T, args map[string]any) *mcptypes.CallToolResult {
	t.Helper()
	result, err := tc.serverTool.Handler(context.Background(), mcptypes.CallToolRequest{
		Params: mcptypes.CallToolParams{Arguments: args},
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	return result
}

func cloneArguments(input map[string]any) map[string]any {
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func writeTestBackup(t *testing.T) string {
	t.Helper()
	backupFile := filepath.Join(t.TempDir(), "workflowy-backup.json")
	require.NoError(t, os.WriteFile(backupFile, []byte(`[
		{"id":"c","nm":"Charlie","ch":[{"id":"c1","nm":"Charlie child"}]},
		{"id":"a","nm":"Alpha"},
		{"id":"b","nm":"Bravo"}
	]`), 0o600))
	return backupFile
}
