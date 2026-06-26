package mcp

import (
	"context"
	"errors"
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
	ToolChildren       = "workflowy_children"
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
	method      string // "get", "export", "backup", or "" for auto-select based on depth
	backupFile  string // Path to backup file (for backup method)
}

// NewToolBuilder creates a builder bound to the provided Workflowy client.
// If writeRootID is set, write operations are restricted to that node and its descendants.
// If readRootID is set, all operations are restricted to that node and its descendants.
// If method is set, it forces all operations to use that method (get, export, or backup).
func NewToolBuilder(client workflowy.Client, writeRootID, readRootID, method, backupFile string) ToolBuilder {
	return ToolBuilder{client: client, writeRootID: writeRootID, readRootID: readRootID, method: method, backupFile: backupFile}
}

// isRestricted returns true if write restrictions are in effect.
func (b ToolBuilder) isRestricted() bool {
	return workflowy.IsWriteRestricted(b.writeRootID)
}

// isReadRestricted returns true if read restrictions are in effect.
func (b ToolBuilder) isReadRestricted() bool {
	return workflowy.IsRestricted(b.readRootID)
}

// validateAccessWithRetry loads the tree, runs the validator, and retries with a cache
// refresh if the node is not found. The label (e.g. "target", "parent") and rootLabel
// (e.g. "read_root", "write_root") are used for log messages.
func (b ToolBuilder) validateAccessWithRetry(
	ctx context.Context,
	resolvedID, rootID, operation, label, rootLabel, method string,
	validate func(items []*workflowy.Item) error,
) error {
	items, err := b.loadTree(ctx, method)
	if err != nil {
		return fmt.Errorf("cannot load tree for %s validation: %w", rootLabel, err)
	}
	err = validate(items)
	if err == nil {
		return nil
	}

	var notFound *workflowy.NodeNotFoundError
	if errors.As(err, &notFound) {
		items, refreshErr := b.loadTreeWithRefresh(ctx, method, true)
		if refreshErr != nil {
			slog.Warn(label+" not found, cache refresh failed",
				"operation", operation, label, resolvedID, "error", refreshErr)
			return fmt.Errorf("%s denied: %s %s not found in cache (refresh failed: %w)",
				operation, label, resolvedID, refreshErr)
		}
		err = validate(items)
		if errors.As(err, &notFound) {
			slog.Warn(label+" not found after cache refresh, may be too new",
				"operation", operation, label, resolvedID, rootLabel, rootID)
			return fmt.Errorf("%s denied: %s %s not found in cache; if newly created, retry in ~60s",
				operation, label, resolvedID)
		}
	}

	if err != nil {
		slog.Warn(rootLabel+" access denied", "operation", operation, label, resolvedID,
			rootLabel, rootID, "error", err)
	}
	return err
}

// validateReadTarget checks if the target is within the read-root scope.
func (b ToolBuilder) validateReadTarget(ctx context.Context, targetID, operation, method string) error {
	if !b.isReadRestricted() {
		return nil
	}
	resolvedID, err := workflowy.ResolveNodeIDToUUID(ctx, b.client, targetID)
	if err != nil {
		slog.Warn("read access denied: target outside read-root scope",
			"operation", operation, "target", targetID, "read_root", b.readRootID)
		return fmt.Errorf("%s denied: %s is not within read-root %s", operation, targetID, b.readRootID)
	}
	return b.validateAccessWithRetry(ctx, resolvedID, b.readRootID, operation, "target", "read_root", method,
		func(items []*workflowy.Item) error {
			return workflowy.ValidateReadAccess(items, b.readRootID, resolvedID, operation)
		},
	)
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
func (b ToolBuilder) validateWriteTarget(ctx context.Context, targetID, operation, method string) error {
	if !b.isRestricted() {
		return nil
	}
	resolvedID, err := workflowy.ResolveNodeIDToUUID(ctx, b.client, targetID)
	if err != nil {
		slog.Warn("write access denied: target outside write-root scope",
			"operation", operation, "target", targetID, "write_root", b.writeRootID)
		return fmt.Errorf("%s denied: %s is not within write-root %s", operation, targetID, b.writeRootID)
	}
	return b.validateAccessWithRetry(ctx, resolvedID, b.writeRootID, operation, "target", "write_root", method,
		func(items []*workflowy.Item) error {
			return workflowy.ValidateWriteAccess(items, b.writeRootID, resolvedID, operation)
		},
	)
}

// validateWriteParent checks if the parent is within the write-root scope.
func (b ToolBuilder) validateWriteParent(ctx context.Context, parentID, operation, method string) error {
	if !b.isRestricted() {
		return nil
	}
	if parentID == "None" || parentID == "" {
		err := fmt.Errorf("%s denied: cannot use root as parent when write-root-id is set to %s", operation, b.writeRootID)
		slog.Warn("write parent access denied", "operation", operation, "parent", parentID,
			"write_root", b.writeRootID, "error", err)
		return err
	}
	resolvedID, err := workflowy.ResolveNodeIDToUUID(ctx, b.client, parentID)
	if err != nil {
		slog.Warn("write parent access denied: target outside write-root scope",
			"operation", operation, "parent", parentID, "write_root", b.writeRootID)
		return fmt.Errorf("%s denied: %s is not within write-root %s", operation, parentID, b.writeRootID)
	}
	return b.validateAccessWithRetry(ctx, resolvedID, b.writeRootID, operation, "parent", "write_root", method,
		func(items []*workflowy.Item) error {
			return workflowy.ValidateWriteAccess(items, b.writeRootID, resolvedID, operation)
		},
	)
}

// defaultCreateParent returns the appropriate parent for create operations.
// Priority: explicit parent > write-root-id > read-root-id > "None"
func (b ToolBuilder) defaultCreateParent(parentID string) string {
	if parentID != "None" && parentID != "" {
		return parentID
	}
	if b.isRestricted() {
		return b.writeRootID
	}
	if b.isReadRestricted() {
		return b.readRootID
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
		ToolChildren:       b.buildChildrenTool,
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
			mcptypes.WithString("method",
				mcptypes.Description("Access method: get, export, or backup (default: auto based on depth)"),
			),
			mcptypes.WithBoolean("include_ancestors",
				mcptypes.Description("Wrap result in ancestor path from root to target node (requires export or backup method)"),
				mcptypes.DefaultBool(false),
			),
			mcptypes.WithNumber("ancestor_depth",
				mcptypes.Description("Include N levels of ancestors (-1 for all, 0 for none; requires export or backup method)"),
				mcptypes.DefaultNumber(0),
			),
			mcptypes.WithString("to_ancestor",
				mcptypes.Description("Include ancestors up to and including this node ID (requires export or backup method)"),
			),
		),
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
			mcptypes.WithString("method",
				mcptypes.Description("Access method: get, export, or backup (default: auto based on depth)"),
			),
		),
		Handler: func(ctx context.Context, req mcptypes.CallToolRequest) (*mcptypes.CallToolResult, error) {
			rawItemID := b.defaultReadID(req.GetString("id", "None"))
			depth := req.GetInt("depth", 2)
			includeEmpty := req.GetBool("include_empty_names", false)
			method := req.GetString("method", "")

			itemID, err := workflowy.ResolveNodeID(ctx, b.client, rawItemID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot resolve ID", err), nil
			}

			if err := b.validateReadTarget(ctx, itemID, "list", method); err != nil {
				return mcptypes.NewToolResultError(err.Error()), nil
			}

			data, err := b.fetchItems(ctx, itemID, depth, method)
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

func (b ToolBuilder) buildChildrenTool() mcpserver.ServerTool {
	return mcpserver.ServerTool{
		Tool: mcptypes.NewTool(
			ToolChildren,
			mcptypes.WithDescription("List direct children with compact paginated output"+b.readRestrictionNote()),
			mcptypes.WithReadOnlyHintAnnotation(true),
			mcptypes.WithDestructiveHintAnnotation(false),
			mcptypes.WithIdempotentHintAnnotation(true),
			mcptypes.WithString("id",
				mcptypes.Description("Parent ID (default: root)"),
				mcptypes.DefaultString("None"),
			),
			mcptypes.WithNumber("limit",
				mcptypes.Description("Maximum number of direct children to return (default 50, max 200)"),
				mcptypes.DefaultNumber(workflowy.DefaultChildrenLimit),
			),
			mcptypes.WithNumber("offset",
				mcptypes.Description("Number of matching direct children to skip before returning results"),
				mcptypes.DefaultNumber(0),
			),
			mcptypes.WithBoolean("compact",
				mcptypes.Description("Return compact child objects with id, name, layoutMode, and completed"),
				mcptypes.DefaultBool(true),
			),
			mcptypes.WithString("name_filter",
				mcptypes.Description("Optional regular expression matched against direct child names before pagination"),
			),
			mcptypes.WithBoolean("ignore_case",
				mcptypes.Description("Apply name_filter case-insensitively"),
				mcptypes.DefaultBool(false),
			),
			mcptypes.WithString("method",
				mcptypes.Description("Access method: get, export, or backup (default: get direct-children API)"),
			),
		),
		Handler: func(ctx context.Context, req mcptypes.CallToolRequest) (*mcptypes.CallToolResult, error) {
			rawItemID := b.defaultReadID(req.GetString("id", "None"))
			method := req.GetString("method", "")

			itemID, err := workflowy.ResolveNodeID(ctx, b.client, rawItemID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot resolve ID", err), nil
			}

			if err := b.validateReadTarget(ctx, itemID, "children", method); err != nil {
				return mcptypes.NewToolResultError(err.Error()), nil
			}

			children, err := b.fetchDirectChildren(ctx, itemID, method)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot list children", err), nil
			}

			page, err := workflowy.NewChildrenPage(children, workflowy.ChildrenPageOptions{
				Limit:      req.GetInt("limit", workflowy.DefaultChildrenLimit),
				Offset:     req.GetInt("offset", 0),
				Compact:    req.GetBool("compact", true),
				NameFilter: req.GetString("name_filter", ""),
				IgnoreCase: req.GetBool("ignore_case", false),
			})
			if err != nil {
				return mcptypes.NewToolResultError(err.Error()), nil
			}

			return mcptypes.NewToolResultJSON(page)
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
			mcptypes.WithBoolean("include_completed",
				mcptypes.Description("Include completed nodes in search results (excluded by default)"),
				mcptypes.DefaultBool(false),
			),
			mcptypes.WithString("group_by",
				mcptypes.Description("Group results by: parent, path, tree, modified.<unit>, created.<unit> (unit: year, month, day, or Go time format)"),
			),
			mcptypes.WithNumber("path_max_length",
				mcptypes.Description("Max characters per path segment name when using group_by=path"),
			),
			mcptypes.WithString("order_by",
				mcptypes.Description("Sort results by: match, parent, path, modified, created (prefix +/- for asc/desc)"),
			),
			mcptypes.WithString("method",
				mcptypes.Description("Access method: get, export, or backup (default: export)"),
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
			includeCompleted := req.GetBool("include_completed", false)
			method := req.GetString("method", "")

			itemID, err := workflowy.ResolveNodeID(ctx, b.client, rawItemID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot resolve ID", err), nil
			}

			if err := b.validateReadTarget(ctx, itemID, "search", method); err != nil {
				return mcptypes.NewToolResultError(err.Error()), nil
			}

			items, err := b.loadTree(ctx, method)
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

			var orderBy *search.OrderBy
			if ob := req.GetString("order_by", ""); ob != "" {
				parsed, err := search.ParseOrderBy(ob)
				if err != nil {
					return mcptypes.NewToolResultError(err.Error()), nil
				}
				orderBy = &parsed
			}

			groupBy := req.GetString("group_by", "")
			if groupBy != "" {
				if groupBy == "tree" {
					tree := search.SearchItemsTree(searchRoot, pattern, useRegexp, ignoreCase, includeCompleted)
					if orderBy != nil {
						search.SortTreeNodes(tree, *orderBy)
					}
					return mcptypes.NewToolResultJSON(map[string]any{"results": tree})
				}

				pathMaxLen := req.GetInt("path_max_length", 20)
				strategy, err := search.ParseGroupBy(groupBy, pathMaxLen)
				if err != nil {
					return mcptypes.NewToolResultError(err.Error()), nil
				}
				grouped := search.SearchItemsGroupedBy(searchRoot, pattern, useRegexp, ignoreCase, includeCompleted, strategy)
				if orderBy != nil {
					search.SortGroupedResults(grouped, *orderBy)
				}
				return mcptypes.NewToolResultJSON(map[string]any{"results": grouped})
			}

			results := search.SearchItems(searchRoot, pattern, useRegexp, ignoreCase, includeCompleted)
			if orderBy != nil {
				search.SortResults(results, *orderBy)
			}
			return mcptypes.NewToolResultJSON(map[string]any{"results": results})
		},
	}
}

func (b ToolBuilder) buildTargetsTool() mcpserver.ServerTool {
	return mcpserver.ServerTool{
		Tool: mcptypes.NewTool(
			ToolTargets,
			mcptypes.WithDescription("List available Workflowy targets (shortcuts and system targets)"),
			mcptypes.WithString("method",
				mcptypes.Description("Access method for resolving root names: get, export, or backup"),
			),
		),
		Handler: func(ctx context.Context, req mcptypes.CallToolRequest) (*mcptypes.CallToolResult, error) {
			method := req.GetString("method", "")

			response, err := b.client.ListTargets(ctx)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot list targets", err), nil
			}

			result := map[string]any{"targets": response.Targets}

			if b.isRestricted() || b.isReadRestricted() {
				items, err := b.loadTree(ctx, method)

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
			mcptypes.WithString("method",
				mcptypes.Description("Access method for validation: get, export, or backup (writes always use API)"),
			),
		),
		Handler: func(ctx context.Context, req mcptypes.CallToolRequest) (*mcptypes.CallToolResult, error) {
			name := strings.TrimSpace(req.GetString("name", ""))
			if name == "" {
				return mcptypes.NewToolResultError("name is required"), nil
			}

			layoutMode := strings.TrimSpace(req.GetString("layout_mode", ""))
			note := strings.TrimSpace(req.GetString("note", ""))
			method := req.GetString("method", "")

			// Default parent to write-root-id or read-root-id if not specified
			rawParentID := b.defaultCreateParent(req.GetString("parent_id", "None"))

			parentID, err := workflowy.ResolveNodeID(ctx, b.client, rawParentID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot resolve parent ID", err), nil
			}

			if err := b.validateReadTarget(ctx, parentID, "create", method); err != nil {
				return mcptypes.NewToolResultError(err.Error()), nil
			}
			if err := b.validateWriteParent(ctx, parentID, "create", method); err != nil {
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
			mcptypes.WithString("method",
				mcptypes.Description("Access method for validation: get, export, or backup (writes always use API)"),
			),
		),
		Handler: func(ctx context.Context, req mcptypes.CallToolRequest) (*mcptypes.CallToolResult, error) {
			rawItemID := strings.TrimSpace(req.GetString("id", ""))
			if rawItemID == "" {
				return mcptypes.NewToolResultError("id is required"), nil
			}
			method := req.GetString("method", "")

			itemID, err := workflowy.ResolveNodeID(ctx, b.client, rawItemID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot resolve ID", err), nil
			}

			if err := b.validateReadTarget(ctx, itemID, "update", method); err != nil {
				return mcptypes.NewToolResultError(err.Error()), nil
			}
			if err := b.validateWriteTarget(ctx, itemID, "update", method); err != nil {
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
			mcptypes.WithString("method",
				mcptypes.Description("Access method for validation: get, export, or backup (writes always use API)"),
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
			method := req.GetString("method", "")

			itemID, err := workflowy.ResolveNodeID(ctx, b.client, rawItemID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot resolve ID", err), nil
			}

			parentID, err := workflowy.ResolveNodeID(ctx, b.client, rawParentID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot resolve parent ID", err), nil
			}

			if err := b.validateReadTarget(ctx, itemID, "move", method); err != nil {
				return mcptypes.NewToolResultError(err.Error()), nil
			}
			if err := b.validateReadTarget(ctx, parentID, "move destination", method); err != nil {
				return mcptypes.NewToolResultError(err.Error()), nil
			}
			if err := b.validateWriteTarget(ctx, itemID, "move", method); err != nil {
				return mcptypes.NewToolResultError(err.Error()), nil
			}
			if err := b.validateWriteParent(ctx, parentID, "move", method); err != nil {
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
			mcptypes.WithString("method",
				mcptypes.Description("Access method for validation: get, export, or backup (writes always use API)"),
			),
		),
		Handler: func(ctx context.Context, req mcptypes.CallToolRequest) (*mcptypes.CallToolResult, error) {
			rawItemID := strings.TrimSpace(req.GetString("id", ""))
			if rawItemID == "" {
				return mcptypes.NewToolResultError("id is required"), nil
			}
			method := req.GetString("method", "")

			itemID, err := workflowy.ResolveNodeID(ctx, b.client, rawItemID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot resolve ID", err), nil
			}

			if err := b.validateReadTarget(ctx, itemID, "delete", method); err != nil {
				return mcptypes.NewToolResultError(err.Error()), nil
			}
			if err := b.validateWriteTarget(ctx, itemID, "delete", method); err != nil {
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
			mcptypes.WithString("method",
				mcptypes.Description("Access method for validation: get, export, or backup (writes always use API)"),
			),
		),
		Handler: func(ctx context.Context, req mcptypes.CallToolRequest) (*mcptypes.CallToolResult, error) {
			rawItemID := strings.TrimSpace(req.GetString("id", ""))
			if rawItemID == "" {
				return mcptypes.NewToolResultError("id is required"), nil
			}
			method := req.GetString("method", "")

			itemID, err := workflowy.ResolveNodeID(ctx, b.client, rawItemID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot resolve ID", err), nil
			}

			if err := b.validateReadTarget(ctx, itemID, "complete", method); err != nil {
				return mcptypes.NewToolResultError(err.Error()), nil
			}
			if err := b.validateWriteTarget(ctx, itemID, "complete", method); err != nil {
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
			mcptypes.WithString("method",
				mcptypes.Description("Access method for validation: get, export, or backup (writes always use API)"),
			),
		),
		Handler: func(ctx context.Context, req mcptypes.CallToolRequest) (*mcptypes.CallToolResult, error) {
			rawItemID := strings.TrimSpace(req.GetString("id", ""))
			if rawItemID == "" {
				return mcptypes.NewToolResultError("id is required"), nil
			}
			method := req.GetString("method", "")

			itemID, err := workflowy.ResolveNodeID(ctx, b.client, rawItemID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot resolve ID", err), nil
			}

			if err := b.validateReadTarget(ctx, itemID, "uncomplete", method); err != nil {
				return mcptypes.NewToolResultError(err.Error()), nil
			}
			if err := b.validateWriteTarget(ctx, itemID, "uncomplete", method); err != nil {
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
			mcptypes.WithString("method",
				mcptypes.Description("Access method: get, export, or backup (default: export)"),
			),
		),
		Handler: func(ctx context.Context, req mcptypes.CallToolRequest) (*mcptypes.CallToolResult, error) {
			rawItemID := b.defaultReadID(req.GetString("id", "None"))
			threshold := req.GetFloat("threshold", 0.01)
			method := req.GetString("method", "")

			itemID, err := workflowy.ResolveNodeID(ctx, b.client, rawItemID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot resolve ID", err), nil
			}

			if err := b.validateReadTarget(ctx, itemID, "report_count", method); err != nil {
				return mcptypes.NewToolResultError(err.Error()), nil
			}

			root, err := b.buildReportRoot(ctx, itemID, method)
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
			mcptypes.WithString("method",
				mcptypes.Description("Access method: get, export, or backup (default: export)"),
			),
		),
		Handler: func(ctx context.Context, req mcptypes.CallToolRequest) (*mcptypes.CallToolResult, error) {
			rawItemID := b.defaultReadID(req.GetString("id", "None"))
			topN := req.GetInt("top_n", 20)
			method := req.GetString("method", "")

			itemID, err := workflowy.ResolveNodeID(ctx, b.client, rawItemID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot resolve ID", err), nil
			}

			if err := b.validateReadTarget(ctx, itemID, "report_children", method); err != nil {
				return mcptypes.NewToolResultError(err.Error()), nil
			}

			root, err := b.buildReportRoot(ctx, itemID, method)
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
			mcptypes.WithString("method",
				mcptypes.Description("Access method: get, export, or backup (default: export)"),
			),
		),
		Handler: func(ctx context.Context, req mcptypes.CallToolRequest) (*mcptypes.CallToolResult, error) {
			rawItemID := b.defaultReadID(req.GetString("id", "None"))
			topN := req.GetInt("top_n", 20)
			method := req.GetString("method", "")

			itemID, err := workflowy.ResolveNodeID(ctx, b.client, rawItemID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot resolve ID", err), nil
			}

			if err := b.validateReadTarget(ctx, itemID, "report_created", method); err != nil {
				return mcptypes.NewToolResultError(err.Error()), nil
			}

			root, err := b.buildReportRoot(ctx, itemID, method)
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
			mcptypes.WithString("method",
				mcptypes.Description("Access method: get, export, or backup (default: export)"),
			),
		),
		Handler: func(ctx context.Context, req mcptypes.CallToolRequest) (*mcptypes.CallToolResult, error) {
			rawItemID := b.defaultReadID(req.GetString("id", "None"))
			topN := req.GetInt("top_n", 20)
			method := req.GetString("method", "")

			itemID, err := workflowy.ResolveNodeID(ctx, b.client, rawItemID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot resolve ID", err), nil
			}

			if err := b.validateReadTarget(ctx, itemID, "report_modified", method); err != nil {
				return mcptypes.NewToolResultError(err.Error()), nil
			}

			root, err := b.buildReportRoot(ctx, itemID, method)
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
			mcptypes.WithString("method",
				mcptypes.Description("Access method: get, export, or backup (writes always use API)"),
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
			method := req.GetString("method", "")

			parentID, err := workflowy.ResolveNodeID(ctx, b.client, rawParentID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot resolve parent ID", err), nil
			}

			if err := b.validateReadTarget(ctx, parentID, "replace", method); err != nil {
				return mcptypes.NewToolResultError(err.Error()), nil
			}
			if err := b.validateWriteTarget(ctx, parentID, "replace", method); err != nil {
				return mcptypes.NewToolResultError(err.Error()), nil
			}

			items, err := b.loadTree(ctx, method)
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
			mcptypes.WithDescription("Transform node names and/or notes. Built-in: "+strings.Join(transform.ListBuiltins(), ", ")+", split, group"+b.writeRestrictionNote()),
			mcptypes.WithString("id",
				mcptypes.Description("ID to transform (includes descendants)"),
				mcptypes.Required(),
			),
			mcptypes.WithString("transform_name",
				mcptypes.Description("Transform name: "+strings.Join(transform.ListBuiltins(), ", ")+", 'split', or 'group'"),
			),
			mcptypes.WithString("exec",
				mcptypes.Description("Shell command template (use {} for input text). Use instead of transform_name."),
			),
			mcptypes.WithString("separator",
				mcptypes.Description("Separator for split transform. Use \\n for newline, \\t for tab."),
				mcptypes.DefaultString(","),
			),
			mcptypes.WithBoolean("regex",
				mcptypes.Description("Treat separator as a regular expression pattern"),
				mcptypes.DefaultBool(false),
			),
			mcptypes.WithBoolean("list",
				mcptypes.Description("Split by markdown list markers (-, *, +, 1., 2), etc.)"),
				mcptypes.DefaultBool(false),
			),
			mcptypes.WithString("group_by",
				mcptypes.Description("For group transform: modified, created, modified.<unit>, created.<unit> (unit: year, month, day)"),
				mcptypes.DefaultString("created.day"),
			),
			mcptypes.WithString("order",
				mcptypes.Description("For group transform: +modified, -modified, +created, -created (default: newest first)"),
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
			mcptypes.WithString("method",
				mcptypes.Description("Access method: get, export, or backup (writes always use API)"),
			),
		),
		Handler: func(ctx context.Context, req mcptypes.CallToolRequest) (*mcptypes.CallToolResult, error) {
			rawItemID := strings.TrimSpace(req.GetString("id", ""))
			if rawItemID == "" {
				return mcptypes.NewToolResultError("id is required"), nil
			}
			method := req.GetString("method", "")

			itemID, err := workflowy.ResolveNodeID(ctx, b.client, rawItemID)
			if err != nil {
				return mcptypes.NewToolResultErrorFromErr("cannot resolve ID", err), nil
			}

			if err := b.validateReadTarget(ctx, itemID, "transform", method); err != nil {
				return mcptypes.NewToolResultError(err.Error()), nil
			}
			if err := b.validateWriteTarget(ctx, itemID, "transform", method); err != nil {
				return mcptypes.NewToolResultError(err.Error()), nil
			}

			items, err := b.loadTree(ctx, method)
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
				useRegex := req.GetBool("regex", false)
				useList := req.GetBool("list", false)
				return b.handleSplitTransform(ctx, req, searchRoot, separator, useRegex, useList)
			}

			// Handle group transform
			if transformName == "group" {
				groupBy := req.GetString("group_by", "created.day")
				order := req.GetString("order", "")
				return b.handleGroupTransform(ctx, req, searchRoot, groupBy, order)
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

func (b ToolBuilder) handleSplitTransform(ctx context.Context, req mcptypes.CallToolRequest, searchRoot []*workflowy.Item, separator string, useRegex, useList bool) (*mcptypes.CallToolResult, error) {
	fields := transform.DetermineFields(req.GetBool("name", false), req.GetBool("note", false))
	dryRun := req.GetBool("dry_run", true)
	depth := req.GetInt("depth", -1)

	var results []transform.SplitResult
	if useList {
		transform.CollectSplitsRegex(searchRoot, transform.MarkdownListPattern, fields, true, 0, depth, &results)
	} else if useRegex {
		pattern, err := regexp.Compile(separator)
		if err != nil {
			return mcptypes.NewToolResultErrorf("invalid regex pattern: %v", err), nil
		}
		transform.CollectSplitsRegex(searchRoot, pattern, fields, true, 0, depth, &results)
	} else {
		separator = transform.UnescapeSeparator(separator)
		transform.CollectSplits(searchRoot, separator, fields, true, 0, depth, &results)
	}

	if !dryRun {
		transform.ApplySplitResults(ctx, b.client, results)
	}

	return mcptypes.NewToolResultJSON(map[string]any{"results": results})
}

func (b ToolBuilder) handleGroupTransform(ctx context.Context, req mcptypes.CallToolRequest, searchRoot []*workflowy.Item, groupBy, order string) (*mcptypes.CallToolResult, error) {
	field, dateFormat, granularity, err := transform.ParseGroupBy(groupBy)
	if err != nil {
		return mcptypes.NewToolResultError(err.Error()), nil
	}

	ascending, err := transform.ParseOrder(order, field)
	if err != nil {
		return mcptypes.NewToolResultError(err.Error()), nil
	}

	dryRun := req.GetBool("dry_run", true)

	opts := transform.GroupOptions{
		Field:       field,
		Format:      dateFormat,
		Granularity: granularity,
		Ascending:   ascending,
		DryRun:      dryRun,
	}

	results := transform.CollectGroupResults(searchRoot, opts)

	if !dryRun {
		if err := transform.ApplyGroupResults(ctx, b.client, results, granularity); err != nil {
			return mcptypes.NewToolResultErrorFromErr("failed to apply group transform", err), nil
		}
	}

	return mcptypes.NewToolResultJSON(map[string]any{"results": results})
}

// fetchItemWithAncestors loads the full tree and returns the target wrapped in its ancestor spine.
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

func (b ToolBuilder) fetchDirectChildren(ctx context.Context, itemID string, method string) ([]*workflowy.Item, error) {
	useMethod := b.resolveMethod(method)
	if useMethod == "" {
		useMethod = "get"
	}

	switch useMethod {
	case "get":
		resp, err := b.client.ListChildren(ctx, itemID)
		if err != nil {
			return nil, err
		}
		return resp.Items, nil

	case "export":
		tree, err := b.loadExportTree(ctx, false)
		if err != nil {
			return nil, err
		}
		if itemID == "None" {
			return tree, nil
		}
		found := workflowy.FindItemByID(tree, itemID)
		if found == nil {
			return nil, fmt.Errorf("item %s not found", itemID)
		}
		return found.Children, nil

	case "backup":
		tree, err := b.loadBackupTree()
		if err != nil {
			return nil, err
		}
		if itemID == "None" {
			return tree, nil
		}
		found := workflowy.FindItemByID(tree, itemID)
		if found == nil {
			return nil, fmt.Errorf("item %s not found in backup", itemID)
		}
		return found.Children, nil

	default:
		return nil, fmt.Errorf("unknown method %s", useMethod)
	}
}

// fetchItems mirrors the CLI logic: depth >=4 or -1 uses export API; otherwise GET API.
// If a method is provided (per-request or builder default), it overrides the auto-selection.
func (b ToolBuilder) fetchItems(ctx context.Context, itemID string, depth int, method string) (interface{}, error) {
	useMethod := b.resolveMethod(method)
	if useMethod == "" {
		// Auto-select based on depth
		useMethod = "get"
		if depth == -1 || depth >= 4 {
			useMethod = "export"
		}
	}

	switch useMethod {
	case "export":
		tree, err := b.loadExportTree(ctx, false)
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

	case "backup":
		tree, err := b.loadBackupTree()
		if err != nil {
			return nil, err
		}

		if itemID != "None" {
			found := workflowy.FindItemInTree(tree, itemID, depth)
			if found == nil {
				return nil, fmt.Errorf("item %s not found in backup", itemID)
			}
			return found, nil
		}

		if depth >= 0 {
			workflowy.LimitItemsDepth(tree, depth)
		}
		return &workflowy.ListChildrenResponse{Items: tree}, nil

	default:
		return nil, fmt.Errorf("unknown method %s", useMethod)
	}
}

func (b ToolBuilder) loadExportTree(ctx context.Context, force bool) ([]*workflowy.Item, error) {
	resp, err := b.client.ExportNodesWithCache(ctx, force)
	if err != nil {
		return nil, err
	}
	root := workflowy.BuildTreeFromExport(resp.Nodes)
	return root.Children, nil
}

func (b ToolBuilder) loadBackupTree() ([]*workflowy.Item, error) {
	if b.backupFile != "" {
		return workflowy.ReadBackupFile(b.backupFile)
	}
	return workflowy.ReadLatestBackup()
}

// resolveMethod returns the per-request method if provided, otherwise falls back to the builder's default.
func (b ToolBuilder) resolveMethod(reqMethod string) string {
	if reqMethod != "" {
		return reqMethod
	}
	return b.method
}

// loadTree loads the tree using the specified method (or builder default if empty).
func (b ToolBuilder) loadTree(ctx context.Context, method string) ([]*workflowy.Item, error) {
	useMethod := b.resolveMethod(method)
	if useMethod == "backup" {
		return b.loadBackupTree()
	}
	return b.loadExportTree(ctx, false)
}

// loadTreeWithRefresh loads the tree, with option to force refresh (only for export method).
func (b ToolBuilder) loadTreeWithRefresh(ctx context.Context, method string, force bool) ([]*workflowy.Item, error) {
	useMethod := b.resolveMethod(method)
	if useMethod == "backup" {
		return b.loadBackupTree()
	}
	return b.loadExportTree(ctx, force)
}

func (b ToolBuilder) buildReportRoot(ctx context.Context, itemID, method string) (*workflowy.Item, error) {
	items, err := b.loadTree(ctx, method)
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
