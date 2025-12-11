package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/mholzen/workflowy/pkg/reports"
	"github.com/mholzen/workflowy/pkg/workflowy"
	"github.com/urfave/cli/v3"
)

func uploadReport(ctx context.Context, cmd *cli.Command, client *workflowy.WorkflowyClient, report reports.ReportOutput) error {
	if client == nil {
		return fmt.Errorf("cannot upload a report without an API client")
	}

	opts := reports.UploadOptions{
		ParentID: cmd.String("parent-id"),
		Position: cmd.String("position"),
	}

	nodeID, err := reports.UploadReport(ctx, client, report, opts)
	if err != nil {
		return err
	}

	fmt.Printf("Report uploaded successfully!\n")
	fmt.Printf("URL: https://workflowy.com/#/%s\n", nodeID)
	return nil
}

func loadTree(ctx context.Context, cmd *cli.Command, client *workflowy.WorkflowyClient) ([]*workflowy.Item, error) {
	var items []*workflowy.Item

	method := cmd.String("method")
	backupFile := cmd.String("backup-file")

	if method != "" && method != "export" && method != "backup" {
		return nil, fmt.Errorf("method must be 'export' or 'backup'")
	}

	useMethod := method
	if useMethod == "" {
		if client == nil {
			useMethod = "backup"
		} else {
			useMethod = "export"
		}
	}

	if client == nil && useMethod == "export" {
		return nil, fmt.Errorf("cannot use 'export' without an API client")
	}

	if useMethod == "backup" {
		return loadFromBackup(backupFile)
	}

	forceRefresh := cmd.Bool("force-refresh")

	slog.Debug("using export API", "force_refresh", forceRefresh)
	response, err := client.ExportNodesWithCache(ctx, forceRefresh)
	if err != nil {
		if method == "" {
			slog.Warn("export failed, falling back to backup", "error", err)
			return loadFromBackup(backupFile)
		}
		return nil, fmt.Errorf("cannot export nodes: %w", err)
	}

	slog.Debug("reconstructing tree from export data")
	root := workflowy.BuildTreeFromExport(response.Nodes)
	items = root.Children

	return items, nil
}

func loadFromBackup(backupFile string) ([]*workflowy.Item, error) {
	if backupFile != "" {
		slog.Debug("using backup file", "file", backupFile)
	} else {
		slog.Debug("using latest backup file")
	}

	var items []*workflowy.Item
	var err error
	if backupFile != "" {
		items, err = workflowy.ReadBackupFile(backupFile)
	} else {
		items, err = workflowy.ReadLatestBackup()
	}
	if err != nil {
		return nil, fmt.Errorf("cannot read backup file: %w", err)
	}
	return items, nil
}

func loadAndCountDescendants(ctx context.Context, cmd *cli.Command, client *workflowy.WorkflowyClient) (workflowy.Descendants, error) {
	items, err := loadTree(ctx, cmd, client)
	if err != nil {
		return nil, err
	}

	var rootItem *workflowy.Item
	itemID := cmd.String("item-id")
	if itemID == "None" && len(items) > 0 {
		rootItem = &workflowy.Item{
			ID:       "root",
			Name:     "Root",
			Children: items,
		}
	} else {
		rootItem = findItemByID(items, itemID)
		if rootItem == nil {
			return nil, fmt.Errorf("item with ID %s not found", itemID)
		}
	}

	threshold := cmd.Float64("threshold")
	return workflowy.CountDescendants(rootItem, threshold), nil
}

func findItemByID(items []*workflowy.Item, id string) *workflowy.Item {
	for _, item := range items {
		if item.ID == id {
			return item
		}
		if found := findItemByID(item.Children, id); found != nil {
			return found
		}
	}
	return nil
}

func printCountTree(node workflowy.Descendants, depth int) {
	indent := strings.Repeat("  ", depth)
	nodeValue := node.NodeValue()

	fmt.Printf("%s- %s (%.1f%%, %d descendants)\n",
		indent,
		(**nodeValue).String(),
		node.RatioToRoot*100,
		node.Count,
	)

	for child := range node.Children() {
		printCountTree(child.Node(), depth+1)
	}
}
