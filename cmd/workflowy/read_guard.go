package main

import (
	"context"
	"fmt"

	"github.com/mholzen/workflowy/pkg/workflowy"
)

// ReadGuard validates read (and write) operations against a read-root restriction.
type ReadGuard struct {
	client     workflowy.Client
	readRootID string
	tree       []*workflowy.Item
}

// NewReadGuard creates a guard that restricts all operations to descendants of readRootID.
// If readRootID is empty or "None", no restrictions are applied.
func NewReadGuard(ctx context.Context, client workflowy.Client, readRootID string) (*ReadGuard, error) {
	guard := &ReadGuard{
		client:     client,
		readRootID: readRootID,
	}

	if !workflowy.IsRestricted(readRootID) {
		return guard, nil
	}

	resolvedID, err := workflowy.ResolveNodeIDToUUID(ctx, client, readRootID)
	if err != nil {
		return nil, fmt.Errorf("read-root-id resolution failed: %w", err)
	}
	guard.readRootID = resolvedID

	resp, err := client.ExportNodesWithCache(ctx, false)
	if err != nil {
		return nil, fmt.Errorf("cannot load tree for read validation: %w", err)
	}
	root := workflowy.BuildTreeFromExport(resp.Nodes)
	guard.tree = root.Children

	if workflowy.FindItemByID(guard.tree, resolvedID) == nil {
		return nil, fmt.Errorf("read-root-id not found: %s", resolvedID)
	}

	return guard, nil
}

// IsRestricted returns true if read restrictions are in effect.
func (g *ReadGuard) IsRestricted() bool {
	return workflowy.IsRestricted(g.readRootID)
}

// ValidateTarget checks if targetID is within the read-root scope.
func (g *ReadGuard) ValidateTarget(targetID, operation string) error {
	if !g.IsRestricted() {
		return nil
	}
	return workflowy.ValidateReadAccess(g.tree, g.readRootID, targetID, operation)
}

// DefaultID returns the readRootID when itemID is "None" and restrictions are in effect,
// otherwise returns the original itemID unchanged.
func (g *ReadGuard) DefaultID(itemID string) string {
	if !g.IsRestricted() {
		return itemID
	}
	if itemID == "None" || itemID == "" {
		return g.readRootID
	}
	return itemID
}

// ReadRootID returns the resolved read-root-id, or empty string if not restricted.
func (g *ReadGuard) ReadRootID() string {
	if !g.IsRestricted() {
		return ""
	}
	return g.readRootID
}
