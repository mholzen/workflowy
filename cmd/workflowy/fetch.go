package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/mholzen/workflowy/pkg/workflowy"
	"github.com/urfave/cli/v3"
)

func fetchItems(cmd *cli.Command, apiCtx context.Context, client workflowy.Client, itemID string, depth int) (interface{}, error) {
	method := cmd.String("method")
	backupFile := cmd.String("backup-file")

	if method != "" && method != "get" && method != "export" && method != "backup" {
		return nil, fmt.Errorf("method must be 'get', 'export', or 'backup'")
	}

	var useMethod string
	if method != "" {
		useMethod = method
	} else if client == nil {
		useMethod = "backup"
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

	slog.Debug("access method determined", "method", useMethod, "depth", depth)

	var result interface{}

	switch useMethod {
	case "backup":
		return fetchFromBackup(backupFile, itemID, depth)

	case "export":
		slog.Debug("using export API", "depth", depth)
		forceRefresh := cmd.Bool("force-refresh")
		response, err := client.ExportNodesWithCache(apiCtx, forceRefresh)
		if err != nil {
			if method == "" {
				slog.Warn("export failed, falling back to backup", "error", err)
				return fetchFromBackup(backupFile, itemID, depth)
			}
			return nil, fmt.Errorf("cannot export nodes: %w", err)
		}

		slog.Debug("reconstructing tree from export data")
		root := workflowy.BuildTreeFromExport(response.Nodes)

		if itemID != "None" {
			found := workflowy.FindItemInTree(root.Children, itemID, depth)
			if found == nil {
				return nil, fmt.Errorf("item %s not found", itemID)
			}
			result = found
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
					return fetchFromBackup(backupFile, itemID, depth)
				}
				return nil, fmt.Errorf("cannot fetch root items: %w", err)
			}
		} else {
			slog.Debug("fetching item", "item_id", itemID, "depth", depth)
			item, err := client.GetItem(apiCtx, itemID)
			if err != nil {
				if method == "" {
					slog.Warn("get API failed, falling back to backup", "error", err)
					return fetchFromBackup(backupFile, itemID, depth)
				}
				return nil, fmt.Errorf("cannot get item: %w", err)
			}

			if depth > 0 {
				childrenResp, err := client.ListChildrenRecursiveWithDepth(apiCtx, itemID, depth)
				if err != nil {
					if method == "" {
						slog.Warn("get API failed fetching children, falling back to backup", "error", err)
						return fetchFromBackup(backupFile, itemID, depth)
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

func fetchFromBackup(backupFile string, itemID string, depth int) (interface{}, error) {
	items, err := loadFromBackupProvider(backupFile, workflowy.DefaultBackupProvider)
	if err != nil {
		return nil, err
	}

	if itemID != "None" {
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

func flattenTree(data interface{}) *workflowy.ListChildrenResponse {
	return workflowy.FlattenTree(data)
}
