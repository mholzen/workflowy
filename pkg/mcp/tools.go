package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	mcptypes "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/mholzen/workflowy/pkg/mirror"
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
	ToolMove           = "workflowy_move"
	ToolDelete         = "workflowy_delete"
	ToolComplete       = "workflowy_complete"
	ToolUncomplete     = "workflowy_uncomplete"
	ToolReportCount    = "workflowy_report_count"
	ToolReportChildren = "workflowy_report_children"
	ToolReportCreated  = "workflowy_report_created"
	ToolReportModified = "workflowy_report_modified"
	ToolReportMirrors  = "workflowy_report_mirrors"
	ToolReplace        = "workflowy_replace"
	ToolTransform      = "workflowy_transform"
)

// ToolBuilder wires Workflowy operations into MCP tool handlers.
type ToolBuilder struct {
	client      workflowy.Client
	writeRootID string
	readRootID  string
}

// NewToolBuilder creates a builder bound to the provided Workflowy client.
// If writeRootID is set, write operations are restricted to that node and its descendants.
// If readRootID is set, all operations are restricted to that node and its descendants.
func NewToolBuilder(client workflowy.Client, writeRootID, readRootID string) ToolBuilder {
	return ToolBuilder{client: client, writeRootID: writeRootID, readRootID: readRootID}
}

// isRestricted returns true if write restrictions are in effect.
func (b ToolBuilder) isRestricted() bool {
	return workflowy.IsWriteRestricted(b.writeRootID)
}

// isReadRestricted returns true if read restrictions are in effect.
func (b ToolBuilder) isReadRestricted() bool {
	return workflowy.IsRestricted(b.readRootID)
}

// validateReadTarget checks if the target is within the read-root scope.
func (b ToolBuilder) validateReadTarget(ctx context.Context, targetID, operation string) error {
	if !b.isReadRestricted() {
		return nil
	}
	items, err := b.loadExportTree(ctx)
	if err != nil {
		return fmt.Errorf("cannot load tree for read validation: %w", err)
	}
	return workflowy.ValidateReadAccess(items, b.readRootID, targetID, operation)
}

// defaultReadID returns the readRootID when itemID is "None" and read restrictions are in effect.
func (b ToolBuilder) defaultReadID(itemID string) string {
	if !b.isReadRestricted() {
		return itemID
	}
	if itemID == "None" || itemID == "" {
		return b.readRootID
	}
	return itemID
}

// readRestrictionNote returns a note about read restrictions if enabled.
func (b ToolBuilder) readRestrictionNote() string {
	if !b.isReadRestricted() {
		return ""
	}
	return fmt.Sprintf(" (restricted to %s and descendants)", b.readRootID)
}

// validateWriteTarget checks if the target is within the write-root scope.
func (b ToolBuilder) validateWriteTarget(ctx context.Context, targetID, operation string) error {
	if !b.isRestricted() {
		return nil
	}
	items, err := b.loadExportTree(ctx)
	if err != nil {
		return fmt.Errorf("cannot load tree for write validation: %w", err)
	}
	return workflowy.ValidateWriteAccess(items, b.writeRootID, targetID, operation)
}

// validateWriteParent checks if the parent is within the write-root scope.
func (b ToolBuilder) validateWriteParent(ctx context.Context, parentID, operation string) error {
	if !b.isRestricted() {
		return nil
	}
	if parentID == "None" || parentID == "" {
		return fmt.Errorf("%s denied: cannot use root as parent when write-root-id is set to %s", operation, b.writeRootID)
	}
	items, err := b.loadExportTree(ctx)
	if err != nil {
		return fmt.Errorf("cannot load tree for write validation: %w", err)
	}
	return workflowy.ValidateWriteAccess(items, b.writeRootID, parentID, operation)
}

// defaultParent returns the write-root-id if parentID is "None" and restrictions are in effect.
func (b ToolBuilder) defaultParent(parentID string) string {
	if !b.isRestricted() {
		return parentID
	}
	if parentID == "None" || parentID == "" {
		return b.writeRootID
	}
	return parentID
}

// writeRestrictionNote returns a note about write restrictions if enabled.
func (b ToolBuilder) writeRestrictionNote() string {
	if !b.isRestricted() {
		return ""
	}
	return fmt.Sprintf(" (writes restricted to %s and descendants)", b.writeRootID)
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
		ToolMove:           b.buildMoveTool,
		ToolDelete:         b.buildDeleteTool,
		ToolComplete:       b.buildCompleteTool,
		ToolUncomplete:     b.buildUncompleteTool,
		ToolReportCount:    b.buildReportCountTool,
		ToolReportChildren: b.buildReportChildrenTool,
		ToolReportCreated:  b.buildReportCreatedTool,
		ToolReportModified: b.buildReportModifiedTool,
		ToolReportMirrors:  b.buildReportMirrorsTool,
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
			mcptypes.WithDescription("Get node and descendants"+b.readRestrictionNote()),
			mcptypes.WithString("id",
				mcptypes.Description("ID (default: root)"),
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
			rawItemID := b.defaultReadID(req.GetString("id", "None"))
			depth := req.GetInt("depth", 2)
			includeEmpty := req.GetBool("include_empty_names", false)

			itemID, err := workflowy.ResolveNodeID(ctx, b.client, rawItemID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot resolve ID", err), nil
			}

			if err := b.validateReadTarget(ctx, itemID, "get"); err != nil {
				return mcptypes.NewToolResultError(err.Error()), nil
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
			mcptypes.WithDescription("List descendants as flat list"+b.readRestrictionNote()),
			mcptypes.WithString("id",
				mcptypes.Description("ID (default: root)"),
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
			rawItemID := b.defaultReadID(req.GetString("id", "None"))
			depth := req.GetInt("depth", 2)
			includeEmpty := req.GetBool("include_empty_names", false)

			itemID, err := workflowy.ResolveNodeID(ctx, b.client, rawItemID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot resolve ID", err), nil
			}

			if err := b.validateReadTarget(ctx, itemID, "list"); err != nil {
				return mcptypes.NewToolResultError(err.Error()), nil
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
			mcptypes.WithDescription("Search node names by text or regular expression"+b.readRestrictionNote()),
			mcptypes.WithString("pattern",
				mcptypes.Description("Search text or regular expression"),
				mcptypes.Required(),
			),
			mcptypes.WithString("id",
				mcptypes.Description("ID to search within (default: root)"),
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

			rawItemID := b.defaultReadID(req.GetString("id", "None"))
			useRegexp := req.GetBool("regexp", false)
			ignoreCase := req.GetBool("ignore_case", false)

			itemID, err := workflowy.ResolveNodeID(ctx, b.client, rawItemID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot resolve ID", err), nil
			}

			if err := b.validateReadTarget(ctx, itemID, "search"); err != nil {
				return mcptypes.NewToolResultError(err.Error()), nil
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

			result := map[string]any{"targets": response.Targets}

			if b.isRestricted() || b.isReadRestricted() {
				items, err := b.loadExportTree(ctx)

				if b.isRestricted() {
					writeRoot := map[string]string{"id": b.writeRootID}
					if err == nil {
						if item := workflowy.FindItemByID(items, b.writeRootID); item != nil {
							writeRoot["name"] = item.Name
						}
					}
					result["write_root"] = writeRoot
				}

				if b.isReadRestricted() {
					readRoot := map[string]string{"id": b.readRootID}
					if err == nil {
						if item := workflowy.FindItemByID(items, b.readRootID); item != nil {
							readRoot["name"] = item.Name
						}
					}
					result["read_root"] = readRoot
				}
			}

			return mcptypes.NewToolResultJSON(result)
		},
	}
}

func (b ToolBuilder) buildIDTool() mcpserver.ServerTool {
	return mcpserver.ServerTool{
		Tool: mcptypes.NewTool(
			ToolID,
			mcptypes.WithDescription("Resolve a short ID or target key to full UUID"),
			mcptypes.WithString("id",
				mcptypes.Description("ID to resolve to full UUID"),
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
			mcptypes.WithDescription("Create a new node"+b.writeRestrictionNote()),
			mcptypes.WithString("name",
				mcptypes.Description("Node name"),
				mcptypes.Required(),
			),
			mcptypes.WithString("parent_id",
				mcptypes.Description("Parent ID: UUID or target key (default: root)"),
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

			layoutMode := strings.TrimSpace(req.GetString("layout_mode", ""))
			note := strings.TrimSpace(req.GetString("note", ""))

			// Default parent to write-root-id if not specified and restrictions are in effect
			rawParentID := b.defaultParent(req.GetString("parent_id", "None"))

			parentID, err := workflowy.ResolveNodeID(ctx, b.client, rawParentID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot resolve parent ID", err), nil
			}

			if err := b.validateReadTarget(ctx, parentID, "create"); err != nil {
				return mcptypes.NewToolResultError(err.Error()), nil
			}
			if err := b.validateWriteParent(ctx, parentID, "create"); err != nil {
				return mcptypes.NewToolResultError(err.Error()), nil
			}

			request := &workflowy.CreateNodeRequest{
				ParentID: parentID,
				Name:     name,
			}
			if err := request.SetPosition(strings.TrimSpace(req.GetString("position", ""))); err != nil {
				return mcptypes.NewToolResultError(err.Error()), nil
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
			mcptypes.WithDescription("Update an existing node"+b.writeRestrictionNote()),
			mcptypes.WithString("id",
				mcptypes.Description("ID to update"),
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
			rawItemID := strings.TrimSpace(req.GetString("id", ""))
			if rawItemID == "" {
				return mcptypes.NewToolResultError("id is required"), nil
			}

			itemID, err := workflowy.ResolveNodeID(ctx, b.client, rawItemID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot resolve ID", err), nil
			}

			if err := b.validateReadTarget(ctx, itemID, "update"); err != nil {
				return mcptypes.NewToolResultError(err.Error()), nil
			}
			if err := b.validateWriteTarget(ctx, itemID, "update"); err != nil {
				return mcptypes.NewToolResultError(err.Error()), nil
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

func (b ToolBuilder) buildMoveTool() mcpserver.ServerTool {
	return mcpserver.ServerTool{
		Tool: mcptypes.NewTool(
			ToolMove,
			mcptypes.WithDescription("Move a node to a new parent"+b.writeRestrictionNote()),
			mcptypes.WithString("id",
				mcptypes.Description("ID to move"),
				mcptypes.Required(),
			),
			mcptypes.WithString("parent_id",
				mcptypes.Description("Destination parent: UUID, target key (home, inbox), or 'None' for top-level"),
				mcptypes.Required(),
			),
			mcptypes.WithString("position",
				mcptypes.Description("Position in new parent: top or bottom (default: top)"),
			),
		),
		Handler: func(ctx context.Context, req mcptypes.CallToolRequest) (*mcptypes.CallToolResult, error) {
			rawItemID := strings.TrimSpace(req.GetString("id", ""))
			if rawItemID == "" {
				return mcptypes.NewToolResultError("id is required"), nil
			}

			rawParentID := strings.TrimSpace(req.GetString("parent_id", ""))
			if rawParentID == "" {
				return mcptypes.NewToolResultError("parent_id is required"), nil
			}

			itemID, err := workflowy.ResolveNodeID(ctx, b.client, rawItemID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot resolve ID", err), nil
			}

			parentID, err := workflowy.ResolveNodeID(ctx, b.client, rawParentID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot resolve parent ID", err), nil
			}

			if err := b.validateReadTarget(ctx, itemID, "move"); err != nil {
				return mcptypes.NewToolResultError(err.Error()), nil
			}
			if err := b.validateReadTarget(ctx, parentID, "move destination"); err != nil {
				return mcptypes.NewToolResultError(err.Error()), nil
			}
			if err := b.validateWriteTarget(ctx, itemID, "move"); err != nil {
				return mcptypes.NewToolResultError(err.Error()), nil
			}
			if err := b.validateWriteParent(ctx, parentID, "move"); err != nil {
				return mcptypes.NewToolResultError(err.Error()), nil
			}

			request := &workflowy.MoveNodeRequest{
				ParentID: parentID,
			}
			if err := request.SetPosition(strings.TrimSpace(req.GetString("position", ""))); err != nil {
				return mcptypes.NewToolResultError(err.Error()), nil
			}

			response, err := b.client.MoveNode(ctx, itemID, request)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot move node", err), nil
			}

			return mcptypes.NewToolResultJSON(response)
		},
	}
}

func (b ToolBuilder) buildDeleteTool() mcpserver.ServerTool {
	return mcpserver.ServerTool{
		Tool: mcptypes.NewTool(
			ToolDelete,
			mcptypes.WithDescription("Delete a node"+b.writeRestrictionNote()),
			mcptypes.WithString("id",
				mcptypes.Description("ID to delete"),
				mcptypes.Required(),
			),
		),
		Handler: func(ctx context.Context, req mcptypes.CallToolRequest) (*mcptypes.CallToolResult, error) {
			rawItemID := strings.TrimSpace(req.GetString("id", ""))
			if rawItemID == "" {
				return mcptypes.NewToolResultError("id is required"), nil
			}

			itemID, err := workflowy.ResolveNodeID(ctx, b.client, rawItemID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot resolve ID", err), nil
			}

			if err := b.validateReadTarget(ctx, itemID, "delete"); err != nil {
				return mcptypes.NewToolResultError(err.Error()), nil
			}
			if err := b.validateWriteTarget(ctx, itemID, "delete"); err != nil {
				return mcptypes.NewToolResultError(err.Error()), nil
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
			mcptypes.WithDescription("Mark a node as complete"+b.writeRestrictionNote()),
			mcptypes.WithString("id",
				mcptypes.Description("ID to complete"),
				mcptypes.Required(),
			),
		),
		Handler: func(ctx context.Context, req mcptypes.CallToolRequest) (*mcptypes.CallToolResult, error) {
			rawItemID := strings.TrimSpace(req.GetString("id", ""))
			if rawItemID == "" {
				return mcptypes.NewToolResultError("id is required"), nil
			}

			itemID, err := workflowy.ResolveNodeID(ctx, b.client, rawItemID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot resolve ID", err), nil
			}

			if err := b.validateReadTarget(ctx, itemID, "complete"); err != nil {
				return mcptypes.NewToolResultError(err.Error()), nil
			}
			if err := b.validateWriteTarget(ctx, itemID, "complete"); err != nil {
				return mcptypes.NewToolResultError(err.Error()), nil
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
			mcptypes.WithDescription("Mark a node as uncomplete"+b.writeRestrictionNote()),
			mcptypes.WithString("id",
				mcptypes.Description("ID to uncomplete"),
				mcptypes.Required(),
			),
		),
		Handler: func(ctx context.Context, req mcptypes.CallToolRequest) (*mcptypes.CallToolResult, error) {
			rawItemID := strings.TrimSpace(req.GetString("id", ""))
			if rawItemID == "" {
				return mcptypes.NewToolResultError("id is required"), nil
			}

			itemID, err := workflowy.ResolveNodeID(ctx, b.client, rawItemID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot resolve ID", err), nil
			}

			if err := b.validateReadTarget(ctx, itemID, "uncomplete"); err != nil {
				return mcptypes.NewToolResultError(err.Error()), nil
			}
			if err := b.validateWriteTarget(ctx, itemID, "uncomplete"); err != nil {
				return mcptypes.NewToolResultError(err.Error()), nil
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
			mcptypes.WithDescription("Generate descendant count report"+b.readRestrictionNote()),
			mcptypes.WithString("id",
				mcptypes.Description("ID (default: root)"),
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
			rawItemID := b.defaultReadID(req.GetString("id", "None"))
			threshold := req.GetFloat("threshold", 0.01)

			itemID, err := workflowy.ResolveNodeID(ctx, b.client, rawItemID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot resolve ID", err), nil
			}

			if err := b.validateReadTarget(ctx, itemID, "report_count"); err != nil {
				return mcptypes.NewToolResultError(err.Error()), nil
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
			mcptypes.WithDescription("Rank nodes by immediate children count"+b.readRestrictionNote()),
			mcptypes.WithString("id",
				mcptypes.Description("ID (default: root)"),
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
			rawItemID := b.defaultReadID(req.GetString("id", "None"))
			topN := req.GetInt("top_n", 20)

			itemID, err := workflowy.ResolveNodeID(ctx, b.client, rawItemID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot resolve ID", err), nil
			}

			if err := b.validateReadTarget(ctx, itemID, "report_children"); err != nil {
				return mcptypes.NewToolResultError(err.Error()), nil
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
			mcptypes.WithDescription("Rank nodes by creation date (oldest first)"+b.readRestrictionNote()),
			mcptypes.WithString("id",
				mcptypes.Description("ID (default: root)"),
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
			rawItemID := b.defaultReadID(req.GetString("id", "None"))
			topN := req.GetInt("top_n", 20)

			itemID, err := workflowy.ResolveNodeID(ctx, b.client, rawItemID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot resolve ID", err), nil
			}

			if err := b.validateReadTarget(ctx, itemID, "report_created"); err != nil {
				return mcptypes.NewToolResultError(err.Error()), nil
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
			mcptypes.WithDescription("Rank nodes by modification date (oldest first)"+b.readRestrictionNote()),
			mcptypes.WithString("id",
				mcptypes.Description("ID (default: root)"),
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
			rawItemID := b.defaultReadID(req.GetString("id", "None"))
			topN := req.GetInt("top_n", 20)

			itemID, err := workflowy.ResolveNodeID(ctx, b.client, rawItemID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot resolve ID", err), nil
			}

			if err := b.validateReadTarget(ctx, itemID, "report_modified"); err != nil {
				return mcptypes.NewToolResultError(err.Error()), nil
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

func (b ToolBuilder) buildReportMirrorsTool() mcpserver.ServerTool {
	return mcpserver.ServerTool{
		Tool: mcptypes.NewTool(
			ToolReportMirrors,
			mcptypes.WithDescription("Rank nodes by mirror count (most mirrored first). Uses backup file as mirror data is only available there."),
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
			topN := req.GetInt("top_n", 20)

			items, err := workflowy.ReadLatestBackup()
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot load backup file (mirror data requires backup)", err), nil
			}

			infos := mirror.CollectMirrorInfos(items)
			ranked := mirror.RankByMirrorCount(infos, topN)

			output := &reports.MirrorCountReportOutput{
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
			mcptypes.WithDescription("Search and replace text in node names using regex"+b.writeRestrictionNote()),
			mcptypes.WithString("pattern",
				mcptypes.Description("Regular expression pattern to match"),
				mcptypes.Required(),
			),
			mcptypes.WithString("substitution",
				mcptypes.Description("Replacement string (supports groups)"),
				mcptypes.Required(),
			),
			mcptypes.WithString("parent_id",
				mcptypes.Description("Parent ID to limit replacement scope: UUID or target key (default: root)"),
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

			if err := b.validateReadTarget(ctx, parentID, "replace"); err != nil {
				return mcptypes.NewToolResultError(err.Error()), nil
			}
			if err := b.validateWriteTarget(ctx, parentID, "replace"); err != nil {
				return mcptypes.NewToolResultError(err.Error()), nil
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
			mcptypes.WithDescription("Transform node names and/or notes. Built-in: "+strings.Join(transform.ListBuiltins(), ", ")+", split"+b.writeRestrictionNote()),
			mcptypes.WithString("id",
				mcptypes.Description("ID to transform (includes descendants)"),
				mcptypes.Required(),
			),
			mcptypes.WithString("transform_name",
				mcptypes.Description("Transform name: "+strings.Join(transform.ListBuiltins(), ", ")+", or 'split'"),
			),
			mcptypes.WithString("exec",
				mcptypes.Description("Shell command template (use {} for input text). Use instead of transform_name."),
			),
			mcptypes.WithString("separator",
				mcptypes.Description("Separator for split transform. Use \\n for newline, \\t for tab."),
				mcptypes.DefaultString(","),
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
			mcptypes.WithBoolean("as_child",
				mcptypes.Description("Insert result as child of source node instead of replacing"),
				mcptypes.DefaultBool(false),
			),
		),
		Handler: func(ctx context.Context, req mcptypes.CallToolRequest) (*mcptypes.CallToolResult, error) {
			rawItemID := strings.TrimSpace(req.GetString("id", ""))
			if rawItemID == "" {
				return mcptypes.NewToolResultError("id is required"), nil
			}

			itemID, err := workflowy.ResolveNodeID(ctx, b.client, rawItemID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot resolve ID", err), nil
			}

			if err := b.validateReadTarget(ctx, itemID, "transform"); err != nil {
				return mcptypes.NewToolResultError(err.Error()), nil
			}
			if err := b.validateWriteTarget(ctx, itemID, "transform"); err != nil {
				return mcptypes.NewToolResultError(err.Error()), nil
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

			transformName := strings.TrimSpace(req.GetString("transform_name", ""))
			execCmd := strings.TrimSpace(req.GetString("exec", ""))

			// Handle split transform
			if transformName == "split" {
				separator := req.GetString("separator", ",")
				return b.handleSplitTransform(ctx, req, searchRoot, separator)
			}

			// Handle exec (no transform_name required)
			if execCmd != "" {
				if transformName != "" {
					return mcptypes.NewToolResultError("cannot use both transform_name and exec"), nil
				}
			} else if transformName == "" {
				return mcptypes.NewToolResultError("transform_name required (use a built-in, 'split', or exec)"), nil
			}

			t, err := transform.ResolveTransformer(transformName, execCmd)
			if err != nil {
				return mcptypes.NewToolResultError(err.Error()), nil
			}

			asChild := req.GetBool("as_child", false)
			opts := transform.Options{
				Transformer: t,
				Fields:      transform.DetermineFields(req.GetBool("name", false), req.GetBool("note", false)),
				DryRun:      req.GetBool("dry_run", true),
				Interactive: false,
				Depth:       req.GetInt("depth", -1),
				AsChild:     asChild,
			}

			results := make([]transform.Result, 0)
			transform.CollectTransformations(searchRoot, opts, 0, &results)

			if !opts.DryRun {
				transform.ApplyResultsWithOptions(ctx, b.client, results, asChild)
			}

			return mcptypes.NewToolResultJSON(map[string]any{"results": results})
		},
	}
}

func (b ToolBuilder) handleSplitTransform(ctx context.Context, req mcptypes.CallToolRequest, searchRoot []*workflowy.Item, separator string) (*mcptypes.CallToolResult, error) {
	separator = transform.UnescapeSeparator(separator)
	fields := transform.DetermineFields(req.GetBool("name", false), req.GetBool("note", false))
	dryRun := req.GetBool("dry_run", true)
	depth := req.GetInt("depth", -1)

	var results []transform.SplitResult
	transform.CollectSplits(searchRoot, separator, fields, true, 0, depth, &results)

	if !dryRun {
		transform.ApplySplitResults(ctx, b.client, results)
	}

	return mcptypes.NewToolResultJSON(map[string]any{"results": results})
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
