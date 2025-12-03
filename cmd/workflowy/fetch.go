package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/mholzen/workflowy/pkg/workflowy"
	"github.com/urfave/cli/v3"
)

func fetchItems(cmd *cli.Command, apiCtx context.Context, itemID string, depth int) (interface{}, error) {
	client := createClient(cmd.String("api-key-file"))

	method := cmd.String("method")
	backupFile := cmd.String("backup-file")

	if method != "" && method != "get" && method != "export" && method != "backup" {
		return nil, fmt.Errorf("method must be 'get', 'export', or 'backup'")
	}

	var useMethod string
	if method != "" {
		useMethod = method
	} else {
		if depth == -1 || depth >= 4 {
			useMethod = "export"
		} else {
			useMethod = "get"
		}
	}

	slog.Debug("access method determined", "method", useMethod, "depth", depth)

	var result interface{}

	switch useMethod {
	case "backup":
		var items []*workflowy.Item
		var err error

		if backupFile != "" {
			slog.Debug("using backup file", "file", backupFile)
			items, err = workflowy.ReadBackupFile(backupFile)
		} else {
			slog.Debug("using latest backup file")
			items, err = workflowy.ReadLatestBackup()
		}
		if err != nil {
			return nil, fmt.Errorf("error reading backup file: %w", err)
		}

		if itemID != "None" {
			found := findItemInTree(items, itemID, depth)
			if found == nil {
				return nil, fmt.Errorf("item %s not found in backup", itemID)
			}
			result = found
		} else {
			result = &workflowy.ListChildrenResponse{Items: items}
		}

	case "export":
		slog.Debug("using export API", "depth", depth)
		forceRefresh := cmd.Bool("force-refresh")
		response, err := client.ExportNodesWithCache(apiCtx, forceRefresh)
		if err != nil {
			return nil, fmt.Errorf("error exporting nodes: %w", err)
		}

		slog.Debug("reconstructing tree from export data")
		root := workflowy.BuildTreeFromExport(response.Nodes)

		if itemID != "None" {
			found := findItemInTree(root.Children, itemID, depth)
			if found == nil {
				return nil, fmt.Errorf("item %s not found", itemID)
			}
			result = found
		} else {
			result = &workflowy.ListChildrenResponse{Items: root.Children}
		}

	case "get":
		slog.Debug("using GET API", "depth", depth)
		if depth < 0 {
			return nil, fmt.Errorf("depth must be non-negative when using GET API (use --method=export for depth=-1)")
		}

		if itemID == "None" {
			slog.Debug("fetching root items", "depth", depth)
			response, err := client.ListChildrenRecursiveWithDepth(apiCtx, itemID, depth)
			if err != nil {
				return nil, fmt.Errorf("error fetching root items: %w", err)
			}
			result = response
		} else {
			slog.Debug("fetching item", "item_id", itemID, "depth", depth)
			item, err := client.GetItem(apiCtx, itemID)
			if err != nil {
				return nil, fmt.Errorf("error getting item: %w", err)
			}

			if depth > 0 {
				childrenResp, err := client.ListChildrenRecursiveWithDepth(apiCtx, itemID, depth)
				if err != nil {
					return nil, fmt.Errorf("error fetching children: %w", err)
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

func findItemInTree(items []*workflowy.Item, targetID string, maxDepth int) *workflowy.Item {
	for _, item := range items {
		if item.ID == targetID {
			if maxDepth >= 0 {
				limitItemDepth(item, maxDepth)
			}
			return item
		}
		if found := findItemInTree(item.Children, targetID, maxDepth); found != nil {
			return found
		}
	}
	return nil
}

func limitItemDepth(item *workflowy.Item, maxDepth int) {
	if maxDepth == 0 {
		item.Children = nil
		return
	}
	for _, child := range item.Children {
		limitItemDepth(child, maxDepth-1)
	}
}

func flattenTree(data interface{}) *workflowy.ListChildrenResponse {
	var items []*workflowy.Item

	switch v := data.(type) {
	case *workflowy.Item:
		items = flattenItem(v)
	case *workflowy.ListChildrenResponse:
		for _, item := range v.Items {
			items = append(items, flattenItem(item)...)
		}
	}

	return &workflowy.ListChildrenResponse{Items: items}
}

func flattenItem(item *workflowy.Item) []*workflowy.Item {
	result := []*workflowy.Item{item}

	for _, child := range item.Children {
		result = append(result, flattenItem(child)...)
	}

	item.Children = nil
	return result
}

