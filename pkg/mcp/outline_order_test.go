package mcp

import (
	"context"
	"testing"

	"github.com/mholzen/workflowy/pkg/search"
	"github.com/mholzen/workflowy/pkg/workflowy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Backup files carry no priority at all — BackupNodeToItem hardcodes it to 0 —
// so a backup fixture cannot show the difference between ordering by sibling
// index and ordering by the outline. Only the export path preserves priority,
// which makes it the only fixture that can pin this behaviour.
//
// The outline is deliberately shaped so that ordering the flattened list by
// priority produces a different sequence from the outline itself:
//
//	Alpha       (priority 0)
//	  Alpha one (priority 0)
//	  Alpha two (priority 1)
//	Bravo       (priority 1)
//	  Bravo one (priority 0)
//
// Outline order:                    Alpha, Alpha one, Alpha two, Bravo, Bravo one
// Sorted flat by priority instead:  Alpha, Alpha one, Bravo one, Alpha two, Bravo
func exportOutlineFixture() *workflowy.ExportNodesResponse {
	id := func(s string) *string { return &s }
	return &workflowy.ExportNodesResponse{Nodes: []workflowy.ExportNode{
		{ID: "a", Name: "Alpha", Priority: 0},
		{ID: "b", Name: "Bravo", Priority: 1},
		{ID: "a1", Name: "Alpha one", ParentID: id("a"), Priority: 0},
		{ID: "a2", Name: "Alpha two", ParentID: id("a"), Priority: 1},
		{ID: "b1", Name: "Bravo one", ParentID: id("b"), Priority: 0},
	}}
}

func exportBuilder(t *testing.T) ToolBuilder {
	t.Helper()
	return NewToolBuilder(&exportOnlyClient{export: exportOutlineFixture()}, "", "", "export", "")
}

func itemNames(t *testing.T, page *workflowy.Page) []string {
	t.Helper()
	items, ok := page.Items.([]*workflowy.Item)
	require.True(t, ok)
	names := make([]string, len(items))
	for i, item := range items {
		names[i] = item.Name
	}
	return names
}

func TestListReturnsOutlineOrderByDefault(t *testing.T) {
	tool := mcpToolCase{serverTool: exportBuilder(t).buildListTool()}

	result := tool.call(t, map[string]any{"id": "None", "depth": -1, "limit": float64(10)})

	page, ok := result.StructuredContent.(*workflowy.Page)
	require.True(t, ok)
	assert.Equal(t,
		[]string{"Alpha", "Alpha one", "Alpha two", "Bravo", "Bravo one"},
		itemNames(t, page),
		"the default sort must not reorder the outline")
}

// Unlike the other tests here this one also passes before the fix, by design:
// value sorts were never broken, and this pins that they stayed that way.
func TestListRanksTheWholeResultSetByValueSorts(t *testing.T) {
	tool := mcpToolCase{serverTool: exportBuilder(t).buildListTool()}

	result := tool.call(t, map[string]any{"id": "None", "depth": -1, "sort": "-name", "limit": float64(10)})

	page, ok := result.StructuredContent.(*workflowy.Page)
	require.True(t, ok)
	assert.Equal(t,
		[]string{"Bravo one", "Bravo", "Alpha two", "Alpha one", "Alpha"},
		itemNames(t, page),
		"a value sort ranks every node, regardless of where it sits in the outline")
}

func resultNames(t *testing.T, result any) []string {
	t.Helper()
	structured, ok := result.(map[string]any)
	require.True(t, ok)
	results, ok := structured["results"].([]search.Result)
	require.True(t, ok)
	names := make([]string, len(results))
	for i, r := range results {
		names[i] = r.Name
	}
	return names
}

// The pattern deliberately matches nodes whose sibling indexes collide across
// different parents, which is the only shape that can tell outline order apart
// from a global sort by priority.
func TestSearchReturnsOutlineOrderByDefault(t *testing.T) {
	tool := mcpToolCase{serverTool: exportBuilder(t).buildSearchTool()}

	result := tool.call(t, map[string]any{"pattern": "o", "method": "export"})

	// Sorting these matches by sibling index instead would yield
	// Alpha one, Bravo one, Alpha two, Bravo.
	assert.Equal(t,
		[]string{"Alpha one", "Alpha two", "Bravo", "Bravo one"},
		resultNames(t, result.StructuredContent))
}

// A descending outline order has to agree with what the same order does to
// list: siblings reversed, but a parent still ahead of its own children.
// Reversing the collected matches would yield Bravo one, Bravo, Alpha two,
// Alpha one, putting a child ahead of its own parent.
func TestSearchDescendingOutlineOrderKeepsParentsAhead(t *testing.T) {
	tool := mcpToolCase{serverTool: exportBuilder(t).buildSearchTool()}

	result := tool.call(t, map[string]any{"pattern": "o", "sort": "-priority", "method": "export"})

	assert.Equal(t,
		[]string{"Bravo", "Bravo one", "Alpha two", "Alpha one"},
		resultNames(t, result.StructuredContent))
}

// exportOnlyClient serves a fixed export payload; every other call is unused by
// the read tools under test and fails loudly if that ever stops being true.
type exportOnlyClient struct {
	workflowy.Client
	export *workflowy.ExportNodesResponse
}

func (c *exportOnlyClient) ExportNodesWithCache(context.Context, bool) (*workflowy.ExportNodesResponse, error) {
	return c.export, nil
}

func (c *exportOnlyClient) ListTargets(context.Context) (*workflowy.ListTargetsResponse, error) {
	return &workflowy.ListTargetsResponse{}, nil
}
