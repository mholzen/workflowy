package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	mcptypes "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/mholzen/workflowy/pkg/replace"
	"github.com/mholzen/workflowy/pkg/reports"
	"github.com/mholzen/workflowy/pkg/search"
	"github.com/mholzen/workflowy/pkg/transform"
	"github.com/mholzen/workflowy/pkg/workflowy"
)

const (
	ToolGet            = "workflowy_get"
	ToolList           = "workflowy_list"
	ToolSearch         = "workflowy_search"
	ToolTargets        = "workflowy_targets"
	ToolID             = "workflowy_id"
	ToolCreate         = "workflowy_create"
	ToolUpdate         = "workflowy_update"
	ToolDelete         = "workflowy_delete"
	ToolComplete       = "workflowy_complete"
	ToolUncomplete     = "workflowy_uncomplete"
	ToolReportCount    = "workflowy_report_count"
	ToolReportChildren = "workflowy_report_children"
	ToolReportCreated  = "workflowy_report_created"
	ToolReportModified = "workflowy_report_modified"
	ToolReplace        = "workflowy_replace"
	ToolTransform      = "workflowy_transform"
)

// ToolBuilder wires Workflowy operations into MCP tool handlers.
type ToolBuilder struct {
	client workflowy.Client
}

// NewToolBuilder creates a builder bound to the provided Workflowy client.
func NewToolBuilder(client workflowy.Client) ToolBuilder {
	return ToolBuilder{client: client}
}

// BuildTools constructs the requested tools in the order provided.
func (b ToolBuilder) BuildTools(toolNames []string) ([]mcpserver.ServerTool, error) {
	factories := map[string]func() mcpserver.ServerTool{
		ToolGet:            b.buildGetTool,
		ToolList:           b.buildListTool,
		ToolSearch:         b.buildSearchTool,
		ToolTargets:        b.buildTargetsTool,
		ToolID:             b.buildIDTool,
		ToolCreate:         b.buildCreateTool,
		ToolUpdate:         b.buildUpdateTool,
		ToolDelete:         b.buildDeleteTool,
		ToolComplete:       b.buildCompleteTool,
		ToolUncomplete:     b.buildUncompleteTool,
		ToolReportCount:    b.buildReportCountTool,
		ToolReportChildren: b.buildReportChildrenTool,
		ToolReportCreated:  b.buildReportCreatedTool,
		ToolReportModified: b.buildReportModifiedTool,
		ToolReplace:        b.buildReplaceTool,
		ToolTransform:      b.buildTransformTool,
	}

	var tools []mcpserver.ServerTool
	for _, name := range toolNames {
		factory, ok := factories[name]
		if !ok {
			return nil, fmt.Errorf("unknown tool: %s", name)
		}
		tools = append(tools, factory())
	}
	return tools, nil
}

func (b ToolBuilder) buildGetTool() mcpserver.ServerTool {
	return mcpserver.ServerTool{
		Tool: mcptypes.NewTool(
			ToolGet,
			mcptypes.WithDescription("Get a node and optional descendants"),
			mcptypes.WithString("item_id",
				mcptypes.Description("Workflowy item ID (None for root)"),
				mcptypes.DefaultString("None"),
			),
			mcptypes.WithNumber("depth",
				mcptypes.Description("Recursion depth (-1 for all, default 2)"),
				mcptypes.DefaultNumber(2),
			),
			mcptypes.WithBoolean("include_empty_names",
				mcptypes.Description("Include items with empty names"),
				mcptypes.DefaultBool(false),
			),
		),
		Handler: func(ctx context.Context, req mcptypes.CallToolRequest) (*mcptypes.CallToolResult, error) {
			rawItemID := req.GetString("item_id", "None")
			depth := req.GetInt("depth", 2)
			includeEmpty := req.GetBool("include_empty_names", false)

			itemID, err := workflowy.ResolveNodeID(ctx, b.client, rawItemID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot resolve item ID", err), nil
			}

			result, err := b.fetchItems(ctx, itemID, depth)
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
	}
}

func (b ToolBuilder) buildListTool() mcpserver.ServerTool {
	return mcpserver.ServerTool{
		Tool: mcptypes.NewTool(
			ToolList,
			mcptypes.WithDescription("List descendants of a node as a flat list"),
			mcptypes.WithString("item_id",
				mcptypes.Description("Workflowy item ID (None for root)"),
				mcptypes.DefaultString("None"),
			),
			mcptypes.WithNumber("depth",
				mcptypes.Description("Recursion depth (-1 for all, default 2)"),
				mcptypes.DefaultNumber(2),
			),
			mcptypes.WithBoolean("include_empty_names",
				mcptypes.Description("Include items with empty names"),
				mcptypes.DefaultBool(false),
			),
		),
		Handler: func(ctx context.Context, req mcptypes.CallToolRequest) (*mcptypes.CallToolResult, error) {
			rawItemID := req.GetString("item_id", "None")
			depth := req.GetInt("depth", 2)
			includeEmpty := req.GetBool("include_empty_names", false)

			itemID, err := workflowy.ResolveNodeID(ctx, b.client, rawItemID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot resolve item ID", err), nil
			}

			data, err := b.fetchItems(ctx, itemID, depth)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot list items", err), nil
			}

			flattened := workflowy.FlattenTree(data)
			if !includeEmpty {
				flattened = workflowy.FilterEmptyList(flattened)
			}

			return mcptypes.NewToolResultJSON(map[string]any{"items": flattened.Items})
		},
	}
}

func (b ToolBuilder) buildSearchTool() mcpserver.ServerTool {
	return mcpserver.ServerTool{
		Tool: mcptypes.NewTool(
			ToolSearch,
			mcptypes.WithDescription("Search node names by text or regular expression"),
			mcptypes.WithString("pattern",
				mcptypes.Description("Search text or regular expression"),
				mcptypes.Required(),
			),
			mcptypes.WithString("item_id",
				mcptypes.Description("Limit search to this subtree (None for root)"),
				mcptypes.DefaultString("None"),
			),
			mcptypes.WithBoolean("regexp",
				mcptypes.Description("Treat pattern as regular expression"),
				mcptypes.DefaultBool(false),
			),
			mcptypes.WithBoolean("ignore_case",
				mcptypes.Description("Case-insensitive search"),
				mcptypes.DefaultBool(false),
			),
		),
		Handler: func(ctx context.Context, req mcptypes.CallToolRequest) (*mcptypes.CallToolResult, error) {
			pattern := strings.TrimSpace(req.GetString("pattern", ""))
			if pattern == "" {
				return mcptypes.NewToolResultError("pattern is required"), nil
			}

			rawItemID := req.GetString("item_id", "None")
			useRegexp := req.GetBool("regexp", false)
			ignoreCase := req.GetBool("ignore_case", false)

			itemID, err := workflowy.ResolveNodeID(ctx, b.client, rawItemID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot resolve item ID", err), nil
			}

			items, err := b.loadExportTree(ctx)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot load tree for search", err), nil
			}

			rootItem := workflowy.FindRootItem(items, itemID)
			if rootItem == nil && itemID != "None" {
				return mcptypes.NewToolResultErrorf("item not found: %s", itemID), nil
			}

			searchRoot := items
			if rootItem != nil {
				searchRoot = []*workflowy.Item{rootItem}
			}

			results := search.SearchItems(searchRoot, pattern, useRegexp, ignoreCase)
			return mcptypes.NewToolResultJSON(map[string]any{"results": results})
		},
	}
}

func (b ToolBuilder) buildTargetsTool() mcpserver.ServerTool {
	return mcpserver.ServerTool{
		Tool: mcptypes.NewTool(
			ToolTargets,
			mcptypes.WithDescription("List available Workflowy targets (shortcuts and system targets)"),
		),
		Handler: func(ctx context.Context, req mcptypes.CallToolRequest) (*mcptypes.CallToolResult, error) {
			response, err := b.client.ListTargets(ctx)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot list targets", err), nil
			}
			return mcptypes.NewToolResultJSON(map[string]any{"targets": response.Targets})
		},
	}
}

func (b ToolBuilder) buildIDTool() mcpserver.ServerTool {
	return mcpserver.ServerTool{
		Tool: mcptypes.NewTool(
			ToolID,
			mcptypes.WithDescription("Resolve a short ID or target key to full UUID"),
			mcptypes.WithString("id",
				mcptypes.Description("Short ID (12 hex chars) or target key to resolve"),
				mcptypes.Required(),
			),
		),
		Handler: func(ctx context.Context, req mcptypes.CallToolRequest) (*mcptypes.CallToolResult, error) {
			rawID := strings.TrimSpace(req.GetString("id", ""))
			if rawID == "" {
				return mcptypes.NewToolResultError("id is required"), nil
			}

			fullID, err := workflowy.ResolveNodeID(ctx, b.client, rawID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot resolve ID", err), nil
			}

			return mcptypes.NewToolResultJSON(map[string]string{"id": fullID})
		},
	}
}

func (b ToolBuilder) buildCreateTool() mcpserver.ServerTool {
	return mcpserver.ServerTool{
		Tool: mcptypes.NewTool(
			ToolCreate,
			mcptypes.WithDescription("Create a new node"),
			mcptypes.WithString("name",
				mcptypes.Description("Node name"),
				mcptypes.Required(),
			),
			mcptypes.WithString("parent_id",
				mcptypes.Description(`Parent node ID or "None" for top-level`),
				mcptypes.DefaultString("None"),
			),
			mcptypes.WithString("position",
				mcptypes.Description(`Position: "top" or "bottom"`),
			),
			mcptypes.WithString("layout_mode",
				mcptypes.Description("Display mode: bullets, todo, h1, h2, h3"),
			),
			mcptypes.WithString("note",
				mcptypes.Description("Optional note content"),
			),
		),
		Handler: func(ctx context.Context, req mcptypes.CallToolRequest) (*mcptypes.CallToolResult, error) {
			name := strings.TrimSpace(req.GetString("name", ""))
			if name == "" {
				return mcptypes.NewToolResultError("name is required"), nil
			}

			position := strings.TrimSpace(req.GetString("position", ""))
			if err := validatePosition(position); err != nil {
				return mcptypes.NewToolResultError(err.Error()), nil
			}

			layoutMode := strings.TrimSpace(req.GetString("layout_mode", ""))
			note := strings.TrimSpace(req.GetString("note", ""))

			parentID, err := workflowy.ResolveNodeID(ctx, b.client, req.GetString("parent_id", "None"))
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot resolve parent ID", err), nil
			}

			request := &workflowy.CreateNodeRequest{
				ParentID: parentID,
				Name:     name,
			}

			if position != "" {
				request.Position = &position
			}
			if layoutMode != "" {
				request.LayoutMode = &layoutMode
			}
			if note != "" {
				request.Note = &note
			}

			response, err := b.client.CreateNode(ctx, request)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot create node", err), nil
			}

			return mcptypes.NewToolResultJSON(response)
		},
	}
}

func (b ToolBuilder) buildUpdateTool() mcpserver.ServerTool {
	return mcpserver.ServerTool{
		Tool: mcptypes.NewTool(
			ToolUpdate,
			mcptypes.WithDescription("Update an existing node"),
			mcptypes.WithString("item_id",
				mcptypes.Description("Node ID to update"),
				mcptypes.Required(),
			),
			mcptypes.WithString("name",
				mcptypes.Description("New node name"),
			),
			mcptypes.WithString("note",
				mcptypes.Description("New note content"),
			),
			mcptypes.WithString("layout_mode",
				mcptypes.Description("Display mode: bullets, todo, h1, h2, h3"),
			),
		),
		Handler: func(ctx context.Context, req mcptypes.CallToolRequest) (*mcptypes.CallToolResult, error) {
			rawItemID := strings.TrimSpace(req.GetString("item_id", ""))
			if rawItemID == "" {
				return mcptypes.NewToolResultError("item_id is required"), nil
			}

			itemID, err := workflowy.ResolveNodeID(ctx, b.client, rawItemID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot resolve item ID", err), nil
			}

			name := strings.TrimSpace(req.GetString("name", ""))
			note := strings.TrimSpace(req.GetString("note", ""))
			layoutMode := strings.TrimSpace(req.GetString("layout_mode", ""))

			request := &workflowy.UpdateNodeRequest{}

			if name != "" {
				request.Name = &name
			}
			if note != "" {
				request.Note = &note
			}
			if layoutMode != "" {
				request.LayoutMode = &layoutMode
			}

			if request.Name == nil && request.Note == nil && request.LayoutMode == nil {
				return mcptypes.NewToolResultError("specify at least one of name, note, or layout_mode"), nil
			}

			response, err := b.client.UpdateNode(ctx, itemID, request)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot update node", err), nil
			}

			return mcptypes.NewToolResultJSON(response)
		},
	}
}

func (b ToolBuilder) buildDeleteTool() mcpserver.ServerTool {
	return mcpserver.ServerTool{
		Tool: mcptypes.NewTool(
			ToolDelete,
			mcptypes.WithDescription("Delete a node"),
			mcptypes.WithString("item_id",
				mcptypes.Description("Node ID to delete"),
				mcptypes.Required(),
			),
		),
		Handler: func(ctx context.Context, req mcptypes.CallToolRequest) (*mcptypes.CallToolResult, error) {
			rawItemID := strings.TrimSpace(req.GetString("item_id", ""))
			if rawItemID == "" {
				return mcptypes.NewToolResultError("item_id is required"), nil
			}

			itemID, err := workflowy.ResolveNodeID(ctx, b.client, rawItemID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot resolve item ID", err), nil
			}

			response, err := b.client.DeleteNode(ctx, itemID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot delete node", err), nil
			}

			return mcptypes.NewToolResultJSON(response)
		},
	}
}

func (b ToolBuilder) buildCompleteTool() mcpserver.ServerTool {
	return mcpserver.ServerTool{
		Tool: mcptypes.NewTool(
			ToolComplete,
			mcptypes.WithDescription("Mark a node as complete"),
			mcptypes.WithString("item_id",
				mcptypes.Description("Node ID to complete"),
				mcptypes.Required(),
			),
		),
		Handler: func(ctx context.Context, req mcptypes.CallToolRequest) (*mcptypes.CallToolResult, error) {
			rawItemID := strings.TrimSpace(req.GetString("item_id", ""))
			if rawItemID == "" {
				return mcptypes.NewToolResultError("item_id is required"), nil
			}

			itemID, err := workflowy.ResolveNodeID(ctx, b.client, rawItemID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot resolve item ID", err), nil
			}

			response, err := b.client.CompleteNode(ctx, itemID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot complete node", err), nil
			}

			return mcptypes.NewToolResultJSON(response)
		},
	}
}

func (b ToolBuilder) buildUncompleteTool() mcpserver.ServerTool {
	return mcpserver.ServerTool{
		Tool: mcptypes.NewTool(
			ToolUncomplete,
			mcptypes.WithDescription("Mark a node as uncomplete"),
			mcptypes.WithString("item_id",
				mcptypes.Description("Node ID to uncomplete"),
				mcptypes.Required(),
			),
		),
		Handler: func(ctx context.Context, req mcptypes.CallToolRequest) (*mcptypes.CallToolResult, error) {
			rawItemID := strings.TrimSpace(req.GetString("item_id", ""))
			if rawItemID == "" {
				return mcptypes.NewToolResultError("item_id is required"), nil
			}

			itemID, err := workflowy.ResolveNodeID(ctx, b.client, rawItemID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot resolve item ID", err), nil
			}

			response, err := b.client.UncompleteNode(ctx, itemID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot uncomplete node", err), nil
			}

			return mcptypes.NewToolResultJSON(response)
		},
	}
}

func (b ToolBuilder) buildReportCountTool() mcpserver.ServerTool {
	return mcpserver.ServerTool{
		Tool: mcptypes.NewTool(
			ToolReportCount,
			mcptypes.WithDescription("Generate descendant count report"),
			mcptypes.WithString("item_id",
				mcptypes.Description("Workflowy item ID (None for root)"),
				mcptypes.DefaultString("None"),
			),
			mcptypes.WithNumber("threshold",
				mcptypes.Description("Minimum ratio threshold (0.0 to 1.0)"),
				mcptypes.DefaultNumber(0.01),
			),
			mcptypes.WithBoolean("preserve_tags",
				mcptypes.Description("Preserve HTML tags in output"),
				mcptypes.DefaultBool(false),
			),
		),
		Handler: func(ctx context.Context, req mcptypes.CallToolRequest) (*mcptypes.CallToolResult, error) {
			rawItemID := req.GetString("item_id", "None")
			threshold := req.GetFloat("threshold", 0.01)

			itemID, err := workflowy.ResolveNodeID(ctx, b.client, rawItemID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot resolve item ID", err), nil
			}

			root, err := b.buildReportRoot(ctx, itemID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot load tree", err), nil
			}

			descendants := workflowy.CountDescendants(root, threshold)

			output := &reports.CountReportOutput{
				RootItem:    root,
				Descendants: descendants,
				Threshold:   threshold,
			}
			nodes, err := output.ToNodes()
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot convert to nodes", err), nil
			}
			slog.Debug("nodes", "nodes", nodes)
			return mcptypes.NewToolResultJSON(nodes)
		},
	}
}

func (b ToolBuilder) buildReportChildrenTool() mcpserver.ServerTool {
	return mcpserver.ServerTool{
		Tool: mcptypes.NewTool(
			ToolReportChildren,
			mcptypes.WithDescription("Rank nodes by immediate children count"),
			mcptypes.WithString("item_id",
				mcptypes.Description("Workflowy item ID (None for root)"),
				mcptypes.DefaultString("None"),
			),
			mcptypes.WithNumber("top_n",
				mcptypes.Description("Number of top results to include (0 for all)"),
				mcptypes.DefaultNumber(20),
			),
			mcptypes.WithBoolean("preserve_tags",
				mcptypes.Description("Preserve HTML tags in output"),
				mcptypes.DefaultBool(false),
			),
		),
		Handler: func(ctx context.Context, req mcptypes.CallToolRequest) (*mcptypes.CallToolResult, error) {
			rawItemID := req.GetString("item_id", "None")
			topN := req.GetInt("top_n", 20)

			itemID, err := workflowy.ResolveNodeID(ctx, b.client, rawItemID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot resolve item ID", err), nil
			}

			root, err := b.buildReportRoot(ctx, itemID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot load tree", err), nil
			}

			descendants := workflowy.CountDescendants(root, 0.0)
			nodesWithTimestamps := workflowy.CollectNodesWithTimestamps(descendants)
			ranked := workflowy.RankByChildrenCount(nodesWithTimestamps, topN)

			output := &reports.ChildrenCountReportOutput{
				Ranked: ranked,
				TopN:   topN,
			}

			return mcptypes.NewToolResultJSON(output)
		},
	}
}

func (b ToolBuilder) buildReportCreatedTool() mcpserver.ServerTool {
	return mcpserver.ServerTool{
		Tool: mcptypes.NewTool(
			ToolReportCreated,
			mcptypes.WithDescription("Rank nodes by creation date (oldest first)"),
			mcptypes.WithString("item_id",
				mcptypes.Description("Workflowy item ID (None for root)"),
				mcptypes.DefaultString("None"),
			),
			mcptypes.WithNumber("top_n",
				mcptypes.Description("Number of top results to include (0 for all)"),
				mcptypes.DefaultNumber(20),
			),
			mcptypes.WithBoolean("preserve_tags",
				mcptypes.Description("Preserve HTML tags in output"),
				mcptypes.DefaultBool(false),
			),
		),
		Handler: func(ctx context.Context, req mcptypes.CallToolRequest) (*mcptypes.CallToolResult, error) {
			rawItemID := req.GetString("item_id", "None")
			topN := req.GetInt("top_n", 20)

			itemID, err := workflowy.ResolveNodeID(ctx, b.client, rawItemID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot resolve item ID", err), nil
			}

			root, err := b.buildReportRoot(ctx, itemID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot load tree", err), nil
			}

			descendants := workflowy.CountDescendants(root, 0.0)
			nodesWithTimestamps := workflowy.CollectNodesWithTimestamps(descendants)
			ranked := workflowy.RankByCreated(nodesWithTimestamps, topN)

			output := &reports.CreatedReportOutput{
				Ranked: ranked,
				TopN:   topN,
			}

			return mcptypes.NewToolResultJSON(output)
		},
	}
}

func (b ToolBuilder) buildReportModifiedTool() mcpserver.ServerTool {
	return mcpserver.ServerTool{
		Tool: mcptypes.NewTool(
			ToolReportModified,
			mcptypes.WithDescription("Rank nodes by modification date (oldest first)"),
			mcptypes.WithString("item_id",
				mcptypes.Description("Workflowy item ID (None for root)"),
				mcptypes.DefaultString("None"),
			),
			mcptypes.WithNumber("top_n",
				mcptypes.Description("Number of top results to include (0 for all)"),
				mcptypes.DefaultNumber(20),
			),
			mcptypes.WithBoolean("preserve_tags",
				mcptypes.Description("Preserve HTML tags in output"),
				mcptypes.DefaultBool(false),
			),
		),
		Handler: func(ctx context.Context, req mcptypes.CallToolRequest) (*mcptypes.CallToolResult, error) {
			rawItemID := req.GetString("item_id", "None")
			topN := req.GetInt("top_n", 20)

			itemID, err := workflowy.ResolveNodeID(ctx, b.client, rawItemID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot resolve item ID", err), nil
			}

			root, err := b.buildReportRoot(ctx, itemID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot load tree", err), nil
			}

			descendants := workflowy.CountDescendants(root, 0.0)
			nodesWithTimestamps := workflowy.CollectNodesWithTimestamps(descendants)
			ranked := workflowy.RankByModified(nodesWithTimestamps, topN)

			output := &reports.ModifiedReportOutput{
				Ranked: ranked,
				TopN:   topN,
			}

			return mcptypes.NewToolResultJSON(output)
		},
	}
}

func (b ToolBuilder) buildReplaceTool() mcpserver.ServerTool {
	return mcpserver.ServerTool{
		Tool: mcptypes.NewTool(
			ToolReplace,
			mcptypes.WithDescription("Search and replace text in node names using regex"),
			mcptypes.WithString("pattern",
				mcptypes.Description("Regular expression pattern to match"),
				mcptypes.Required(),
			),
			mcptypes.WithString("substitution",
				mcptypes.Description("Replacement string (supports groups)"),
				mcptypes.Required(),
			),
			mcptypes.WithString("parent_id",
				mcptypes.Description("Limit replacement to subtree under this node ID"),
				mcptypes.DefaultString("None"),
			),
			mcptypes.WithNumber("depth",
				mcptypes.Description("Maximum depth to traverse (-1 for unlimited)"),
				mcptypes.DefaultNumber(-1),
			),
			mcptypes.WithBoolean("ignore_case",
				mcptypes.Description("Case-insensitive matching"),
				mcptypes.DefaultBool(false),
			),
			mcptypes.WithBoolean("dry_run",
				mcptypes.Description("Show what would be replaced without applying"),
				mcptypes.DefaultBool(true),
			),
		),
		Handler: func(ctx context.Context, req mcptypes.CallToolRequest) (*mcptypes.CallToolResult, error) {
			pattern := strings.TrimSpace(req.GetString("pattern", ""))
			if pattern == "" {
				return mcptypes.NewToolResultError("pattern is required"), nil
			}

			substitution := req.GetString("substitution", "")
			if substitution == "" {
				return mcptypes.NewToolResultError("substitution is required"), nil
			}

			if req.GetBool("ignore_case", false) {
				pattern = "(?i)" + pattern
			}

			re, err := regexp.Compile(pattern)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("invalid regular expression", err), nil
			}

			rawParentID := req.GetString("parent_id", "None")
			depth := req.GetInt("depth", -1)
			dryRun := req.GetBool("dry_run", true)

			parentID, err := workflowy.ResolveNodeID(ctx, b.client, rawParentID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot resolve parent ID", err), nil
			}

			items, err := b.loadExportTree(ctx)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot load tree", err), nil
			}

			searchRoot := items
			if parentID != "None" {
				rootItem := workflowy.FindItemByID(items, parentID)
				if rootItem == nil {
					return mcptypes.NewToolResultErrorf("parent item not found: %s", parentID), nil
				}
				searchRoot = []*workflowy.Item{rootItem}
			}

			opts := replace.Options{
				Pattern:     re,
				Replacement: substitution,
				Interactive: false,
				DryRun:      dryRun,
				Depth:       depth,
			}

			results := make([]replace.Result, 0)
			replace.CollectReplacements(searchRoot, opts, 0, &results)

			if len(results) == 0 {
				return mcptypes.NewToolResultJSON(map[string]any{"results": results})
			}

			if !opts.DryRun {
				for i := range results {
					result := &results[i]
					updateReq := &workflowy.UpdateNodeRequest{
						Name: &result.NewName,
					}
					if _, err := b.client.UpdateNode(ctx, result.ID, updateReq); err != nil {
						result.Skipped = true
						result.SkipReason = fmt.Sprintf("update failed: %v", err)
						continue
					}
					result.Applied = true
				}
			}

			return mcptypes.NewToolResultJSON(map[string]any{"results": results})
		},
	}
}

func (b ToolBuilder) buildTransformTool() mcpserver.ServerTool {
	return mcpserver.ServerTool{
		Tool: mcptypes.NewTool(
			ToolTransform,
			mcptypes.WithDescription("Transform node names and/or notes using built-in or shell transformations. Built-in: "+strings.Join(transform.ListBuiltins(), ", ")),
			mcptypes.WithString("item_id",
				mcptypes.Description("Node ID to transform (includes descendants)"),
				mcptypes.Required(),
			),
			mcptypes.WithString("transform_name",
				mcptypes.Description("Built-in transform name: "+strings.Join(transform.ListBuiltins(), ", ")),
			),
			mcptypes.WithString("exec",
				mcptypes.Description("Shell command template (use {} for input text). Alternative to transform_name."),
			),
			mcptypes.WithNumber("depth",
				mcptypes.Description("Maximum depth to traverse (-1 for unlimited)"),
				mcptypes.DefaultNumber(-1),
			),
			mcptypes.WithBoolean("name",
				mcptypes.Description("Transform node names (default true if neither name nor note specified)"),
				mcptypes.DefaultBool(false),
			),
			mcptypes.WithBoolean("note",
				mcptypes.Description("Transform node notes"),
				mcptypes.DefaultBool(false),
			),
			mcptypes.WithBoolean("dry_run",
				mcptypes.Description("Show what would be transformed without applying"),
				mcptypes.DefaultBool(true),
			),
		),
		Handler: func(ctx context.Context, req mcptypes.CallToolRequest) (*mcptypes.CallToolResult, error) {
			rawItemID := strings.TrimSpace(req.GetString("item_id", ""))
			if rawItemID == "" {
				return mcptypes.NewToolResultError("item_id is required"), nil
			}

			t, err := transform.ResolveTransformer(
				strings.TrimSpace(req.GetString("transform_name", "")),
				strings.TrimSpace(req.GetString("exec", "")),
			)
			if err != nil {
				return mcptypes.NewToolResultError(err.Error()), nil
			}

			itemID, err := workflowy.ResolveNodeID(ctx, b.client, rawItemID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot resolve item ID", err), nil
			}

			items, err := b.loadExportTree(ctx)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot load tree", err), nil
			}

			searchRoot := items
			if itemID != "None" {
				rootItem := workflowy.FindItemByID(items, itemID)
				if rootItem == nil {
					return mcptypes.NewToolResultErrorf("item not found: %s", itemID), nil
				}
				searchRoot = []*workflowy.Item{rootItem}
			}

			opts := transform.Options{
				Transformer: t,
				Fields:      transform.DetermineFields(req.GetBool("name", false), req.GetBool("note", false)),
				DryRun:      req.GetBool("dry_run", true),
				Interactive: false,
				Depth:       req.GetInt("depth", -1),
			}

			results := make([]transform.Result, 0)
			transform.CollectTransformations(searchRoot, opts, 0, &results)

			if !opts.DryRun {
				transform.ApplyResults(ctx, b.client, results)
			}

			return mcptypes.NewToolResultJSON(map[string]any{"results": results})
		},
	}
}

// fetchItems mirrors the CLI logic: depth >=4 or -1 uses export API; otherwise GET API.
func (b ToolBuilder) fetchItems(ctx context.Context, itemID string, depth int) (interface{}, error) {
	useMethod := "get"
	if depth == -1 || depth >= 4 {
		useMethod = "export"
	}

	switch useMethod {
	case "export":
		tree, err := b.loadExportTree(ctx)
		if err != nil {
			return nil, err
		}

		if itemID != "None" {
			found := workflowy.FindItemInTree(tree, itemID, depth)
			if found == nil {
				return nil, fmt.Errorf("item %s not found", itemID)
			}
			return found, nil
		}

		if depth >= 0 {
			workflowy.LimitItemsDepth(tree, depth)
		}
		return &workflowy.ListChildrenResponse{Items: tree}, nil

	case "get":
		if depth < 0 {
			return nil, fmt.Errorf("depth must be non-negative for get method")
		}

		if itemID == "None" {
			resp, err := b.client.ListChildrenRecursiveWithDepth(ctx, itemID, depth)
			if err != nil {
				return nil, err
			}
			return resp, nil
		}

		item, err := b.client.GetItem(ctx, itemID)
		if err != nil {
			return nil, err
		}

		if depth > 0 {
			childrenResp, err := b.client.ListChildrenRecursiveWithDepth(ctx, itemID, depth)
			if err != nil {
				return nil, err
			}
			item.Children = childrenResp.Items
		}
		return item, nil

	default:
		return nil, fmt.Errorf("unknown method %s", useMethod)
	}
}

func (b ToolBuilder) loadExportTree(ctx context.Context) ([]*workflowy.Item, error) {
	resp, err := b.client.ExportNodesWithCache(ctx, false)
	if err != nil {
		return nil, err
	}
	root := workflowy.BuildTreeFromExport(resp.Nodes)
	return root.Children, nil
}

func (b ToolBuilder) buildReportRoot(ctx context.Context, itemID string) (*workflowy.Item, error) {
	items, err := b.loadExportTree(ctx)
	if err != nil {
		return nil, err
	}

	if itemID == "None" {
		return &workflowy.Item{
			ID:       "root",
			Name:     "Root",
			Children: items,
		}, nil
	}

	target := workflowy.FindItemByID(items, itemID)
	if target == nil {
		return nil, fmt.Errorf("item not found: %s", itemID)
	}
	return target, nil
}

func validatePosition(position string) error {
	if position != "" && position != "top" && position != "bottom" {
		return fmt.Errorf("position must be 'top' or 'bottom'")
	}
	return nil
}
