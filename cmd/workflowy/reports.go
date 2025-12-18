package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"regexp"

	"github.com/mholzen/workflowy/pkg/reports"
	"github.com/mholzen/workflowy/pkg/workflowy"
	"github.com/urfave/cli/v3"
)

var htmlTagStripper = regexp.MustCompile(`<[^>]*>`)

func uploadReport(ctx context.Context, cmd *cli.Command, client workflowy.Client, report reports.ReportOutput) error {
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

func outputReport(ctx context.Context, cmd *cli.Command, client workflowy.Client, report reports.ReportOutput, output io.Writer) error {
	if cmd.Bool("upload") {
		return uploadReport(ctx, cmd, client, report)
	}

	format := cmd.String("format")
	if format == "json" {
		item, err := report.ToNodes()
		if err != nil {
			return err
		}
		printJSONToWriter(output, item)
	} else {
		preserveTags := cmd.Bool("preserve-tags")
		return printReportToWriter(output, report, preserveTags)
	}

	return nil
}

func loadTree(ctx context.Context, cmd *cli.Command, client workflowy.Client) ([]*workflowy.Item, error) {
	return loadTreeWithBackupProvider(ctx, cmd, client, workflowy.DefaultBackupProvider)
}

func loadTreeWithBackupProvider(ctx context.Context, cmd *cli.Command, client workflowy.Client, backupProvider workflowy.BackupProvider) ([]*workflowy.Item, error) {
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
		return loadFromBackupProvider(backupFile, backupProvider)
	}

	forceRefresh := cmd.Bool("force-refresh")

	slog.Debug("using export API", "force_refresh", forceRefresh)
	response, err := client.ExportNodesWithCache(ctx, forceRefresh)
	if err != nil {
		if method == "" {
			slog.Warn("export failed, falling back to backup", "error", err)
			return loadFromBackupProvider(backupFile, backupProvider)
		}
		return nil, fmt.Errorf("cannot export nodes: %w", err)
	}

	slog.Debug("reconstructing tree from export data")
	root := workflowy.BuildTreeFromExport(response.Nodes)
	items = root.Children

	return items, nil
}

func loadFromBackupProvider(backupFile string, provider workflowy.BackupProvider) ([]*workflowy.Item, error) {
	if backupFile != "" {
		slog.Debug("using backup file", "file", backupFile)
	} else {
		slog.Debug("using latest backup file")
	}

	var items []*workflowy.Item
	var err error
	if backupFile != "" {
		items, err = provider.ReadBackupFile(backupFile)
	} else {
		items, err = provider.ReadLatestBackup()
	}
	if err != nil {
		return nil, fmt.Errorf("cannot read backup file: %w", err)
	}
	return items, nil
}

func loadAndCountDescendants(ctx context.Context, cmd *cli.Command, client workflowy.Client) (workflowy.Descendants, error) {
	return loadAndCountDescendantsWithBackupProvider(ctx, cmd, client, workflowy.DefaultBackupProvider)
}

func loadAndCountDescendantsWithBackupProvider(ctx context.Context, cmd *cli.Command, client workflowy.Client, backupProvider workflowy.BackupProvider) (workflowy.Descendants, error) {
	items, err := loadTreeWithBackupProvider(ctx, cmd, client, backupProvider)
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

func stripHTMLTags(text string) string {
	return htmlTagStripper.ReplaceAllString(text, "")
}

func printReportToWriter(w io.Writer, report reports.ReportOutput, preserveTags bool) error {
	item, err := report.ToNodes()
	if err != nil {
		return err
	}

	title := item.Name
	if !preserveTags {
		title = stripHTMLTags(title)
	}
	fmt.Fprintf(w, "# %s\n\n", title)

	for _, child := range item.Children {
		printReportItem(w, child, 0, preserveTags)
	}

	return nil
}

func printReportItem(w io.Writer, item *workflowy.Item, depth int, preserveTags bool) {
	indent := ""
	if depth > 0 {
		indent = fmt.Sprintf("%*s", depth*2, "")
	}

	name := item.Name
	if !preserveTags {
		name = stripHTMLTags(name)
	}
	fmt.Fprintf(w, "%s- %s\n", indent, name)

	if len(item.Children) > 0 && item.Children[0].ID == "" {
		for _, child := range item.Children {
			printReportItem(w, child, depth+1, preserveTags)
		}
	}
}

type ReportDeps struct {
	BackupProvider workflowy.BackupProvider
	Output         io.Writer
}

func DefaultReportDeps() ReportDeps {
	return ReportDeps{
		BackupProvider: workflowy.DefaultBackupProvider,
		Output:         os.Stdout,
	}
}

func countReportAction(deps ReportDeps) func(ctx context.Context, cmd *cli.Command, client workflowy.Client) error {
	return func(ctx context.Context, cmd *cli.Command, client workflowy.Client) error {
		items, err := loadTreeWithBackupProvider(ctx, cmd, client, deps.BackupProvider)
		if err != nil {
			return err
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
				return fmt.Errorf("item with ID %s not found", itemID)
			}
		}

		threshold := cmd.Float64("threshold")
		slog.Debug("counting descendants", "threshold", threshold)
		descendants := workflowy.CountDescendants(rootItem, threshold)

		report := &reports.CountReportOutput{
			RootItem:    rootItem,
			Descendants: descendants,
			Threshold:   threshold,
		}

		return outputReport(ctx, cmd, client, report, deps.Output)
	}
}
