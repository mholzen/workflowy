package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/mholzen/workflowy/pkg/workflowy"
	"github.com/urfave/cli/v3"
)

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

func applyAncestorOptions(ancestors []*workflowy.Item, opts ancestorOptions) ([]*workflowy.Item, error) {
	if opts.toAncestorID != "" {
		return workflowy.SliceAncestorsTo(ancestors, opts.toAncestorID)
	}
	return workflowy.TruncateAncestors(ancestors, opts.ancestorDepth), nil
}
