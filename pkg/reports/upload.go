package reports

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/mholzen/workflowy/pkg/workflowy"
)

type UploadOptions struct {
	ParentID string // Where to create the report (default: "None" = root)
	Position string // "top" or "bottom" (optional)
}

func UploadReport(ctx context.Context, client workflowy.Client, report ReportOutput, opts UploadOptions) (string, error) {
	slog.Info("converting report to nodes", "title", report.Title())
	root, err := report.ToNodes()
	if err != nil {
		return "", fmt.Errorf("cannot convert report to nodes: %w", err)
	}

	if opts.ParentID == "" {
		opts.ParentID = "None"
	}

	slog.Info("uploading report tree", "parent_id", opts.ParentID)
	nodeID, err := uploadTree(ctx, client, root, opts.ParentID, opts.Position)
	if err != nil {
		return "", fmt.Errorf("cannot upload report: %w", err)
	}

	slog.Info("report uploaded successfully", "node_id", nodeID)
	return nodeID, nil
}

func uploadTree(ctx context.Context, client workflowy.Client, item *workflowy.Item, parentID string, position string) (string, error) {
	req := &workflowy.CreateNodeRequest{
		ParentID: parentID,
		Name:     item.Name,
		Note:     item.Note,
	}

	if position != "" {
		req.Position = &position
	}

	slog.Debug("creating node", "name", item.Name, "parent_id", parentID)
	resp, err := client.CreateNode(ctx, req)
	if err != nil {
		return "", fmt.Errorf("cannot create node '%s': %w", item.Name, err)
	}

	newNodeID := resp.ItemID
	slog.Debug("node created", "node_id", newNodeID, "name", item.Name)

	for i := len(item.Children) - 1; i >= 0; i-- {
		child := item.Children[i]
		top := "top"
		_, err := uploadTree(ctx, client, child, newNodeID, top)
		if err != nil {
			return "", err
		}
	}

	return newNodeID, nil
}
