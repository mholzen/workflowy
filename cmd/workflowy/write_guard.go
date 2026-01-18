package main

import (
	"context"
	"fmt"

	"github.com/mholzen/workflowy/pkg/workflowy"
)

// WriteGuard validates write operations against a root restriction
type WriteGuard struct {
	client      workflowy.Client
	writeRootID string
	tree        []*workflowy.Item
}

// NewWriteGuard creates a guard that restricts writes to descendants of writeRootID.
// If writeRootID is empty or "None", no restrictions are applied.
func NewWriteGuard(ctx context.Context, client workflowy.Client, writeRootID string) (*WriteGuard, error) {
	guard := &WriteGuard{
		client:      client,
		writeRootID: writeRootID,
	}

	if !workflowy.IsWriteRestricted(writeRootID) {
		return guard, nil
	}

	// Resolve write-root-id to UUID (supports short IDs, target keys)
	resolvedID, err := workflowy.ResolveNodeIDToUUID(ctx, client, writeRootID)
	if err != nil {
		return nil, fmt.Errorf("write-root-id resolution failed: %w", err)
	}
	guard.writeRootID = resolvedID

	// Load tree for descendant checking
	resp, err := client.ExportNodesWithCache(ctx, false)
	if err != nil {
		return nil, fmt.Errorf("cannot load tree for write validation: %w", err)
	}
	root := workflowy.BuildTreeFromExport(resp.Nodes)
	guard.tree = root.Children

	// Verify write-root exists
	if workflowy.FindItemByID(guard.tree, resolvedID) == nil {
		return nil, fmt.Errorf("write-root-id not found: %s", resolvedID)
	}

	return guard, nil
}

// IsRestricted returns true if write restrictions are in effect.
func (g *WriteGuard) IsRestricted() bool {
	return workflowy.IsWriteRestricted(g.writeRootID)
}

// ValidateTarget checks if targetID is within the write-root scope
func (g *WriteGuard) ValidateTarget(targetID, operation string) error {
	if !g.IsRestricted() {
		return nil
	}
	return workflowy.ValidateWriteAccess(g.tree, g.writeRootID, targetID, operation)
}

// ValidateParent checks if parentID is within the write-root scope (for create/move)
func (g *WriteGuard) ValidateParent(parentID, operation string) error {
	if !g.IsRestricted() {
		return nil
	}
	// For "None" parent (root level), deny if we have restrictions
	if parentID == "None" || parentID == "" {
		return fmt.Errorf("%s denied: cannot use root as parent when write-root-id is set to %s", operation, g.writeRootID)
	}
	return workflowy.ValidateWriteAccess(g.tree, g.writeRootID, parentID, operation)
}

// DefaultParent returns the write-root-id if parentID is "None" and restrictions are in effect,
// otherwise returns the original parentID unchanged.
func (g *WriteGuard) DefaultParent(parentID string) string {
	if !g.IsRestricted() {
		return parentID
	}
	if parentID == "None" || parentID == "" {
		return g.writeRootID
	}
	return parentID
}

// WriteRootID returns the resolved write-root-id, or empty string if not restricted.
func (g *WriteGuard) WriteRootID() string {
	if !g.IsRestricted() {
		return ""
	}
	return g.writeRootID
}
