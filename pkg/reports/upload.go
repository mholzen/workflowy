package reports

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/mholzen/workflowy/pkg/workflowy"
)

// UploadOptions configures where and how to upload a report
type UploadOptions struct {
	ParentID string // Where to create the report (default: "None" = root)
	Position string // "top" or "bottom" (optional)
}

// UploadReport uploads a report to WorkFlowy
func UploadReport(ctx context.Context, client *workflowy.WorkflowyClient, report ReportOutput, opts UploadOptions) (string, error) {
	// Convert report to nodes
	slog.Info("converting report to nodes", "title", report.Title())
	root, err := report.ToNodes()
	if err != nil {
		return "", fmt.Errorf("error converting report to nodes: %w", err)
	}

	// Set default parent if not specified
	if opts.ParentID == "" {
		opts.ParentID = "None"
	}

	// Upload the tree
	slog.Info("uploading report tree", "parent_id", opts.ParentID)
	nodeID, err := uploadTree(ctx, client, root, opts.ParentID, opts.Position)
	if err != nil {
		return "", fmt.Errorf("error uploading report: %w", err)
	}

	slog.Info("report uploaded successfully", "node_id", nodeID)
	return nodeID, nil
}

// uploadTree recursively uploads a tree of items to WorkFlowy
func uploadTree(ctx context.Context, client *workflowy.WorkflowyClient, item *workflowy.Item, parentID string, position string) (string, error) {
	// Create the current node
	req := &workflowy.CreateNodeRequest{
		ParentID: parentID,
		Name:     item.Name,
		Note:     item.Note,
	}

	// Add position if specified
	if position != "" {
		req.Position = &position
	}

	slog.Debug("creating node", "name", item.Name, "parent_id", parentID)
	resp, err := client.CreateNode(ctx, req)
	if err != nil {
		return "", fmt.Errorf("error creating node '%s': %w", item.Name, err)
	}

	newNodeID := resp.ItemID
	slog.Debug("node created", "node_id", newNodeID, "name", item.Name)

	// Recursively create children
	// Create them in reverse order at the top to preserve the original order
	for i := len(item.Children) - 1; i >= 0; i-- {
		child := item.Children[i]
		top := "top"
		_, err := uploadTree(ctx, client, child, newNodeID, top)
		if err != nil {
			return "", err // Partial upload - leave as is
		}
	}

	return newNodeID, nil
}
