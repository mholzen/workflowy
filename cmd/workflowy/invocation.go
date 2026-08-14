package main

import (
	"context"
	"fmt"

	"github.com/mholzen/workflowy/pkg/workflowy"
	"github.com/urfave/cli/v3"
)

type commandInvocation struct {
	client         workflowy.Client
	backupProvider workflowy.BackupProvider
	backupItems    []*workflowy.Item
	backup         bool
}

func newCommandInvocation(
	cmd *cli.Command,
	client workflowy.Client,
	backupProvider workflowy.BackupProvider,
) (*commandInvocation, error) {
	invocation := &commandInvocation{client: client, backupProvider: backupProvider}
	method := cmd.String("method")
	if method != "" && method != "get" && method != "export" && method != "backup" {
		return nil, fmt.Errorf("Cannot select access method %q: expected \"get\", \"export\", or \"backup\"", method)
	}
	if client == nil && (method == "get" || method == "export") {
		return nil, fmt.Errorf("Cannot use method %q without using the API", method)
	}
	if method != "backup" && !(method == "" && client == nil) {
		return invocation, nil
	}

	items, err := loadFromBackupProvider(cmd.String("backup-file"), backupProvider)
	if err != nil {
		return nil, fmt.Errorf("Cannot prepare backup invocation: %w", err)
	}
	invocation.backupItems = items
	invocation.backup = true
	return invocation, nil
}

func (invocation *commandInvocation) usesBackup() bool {
	return invocation.backup
}

func (invocation *commandInvocation) resolveNodeID(ctx context.Context, rawID string) (string, error) {
	if invocation.usesBackup() {
		return workflowy.ResolveNodeIDFromTree(invocation.backupItems, rawID)
	}
	return workflowy.ResolveNodeID(ctx, invocation.client, rawID)
}

func (invocation *commandInvocation) newReadGuard(ctx context.Context, readRootID string) (*ReadGuard, error) {
	if invocation.usesBackup() {
		return newReadGuardFromTree(invocation.backupItems, readRootID)
	}
	return NewReadGuard(ctx, invocation.client, readRootID)
}

func (invocation *commandInvocation) newWriteGuard(ctx context.Context, writeRootID string) (*WriteGuard, error) {
	if invocation.usesBackup() {
		return newWriteGuardFromTree(invocation.backupItems, writeRootID)
	}
	return NewWriteGuard(ctx, invocation.client, writeRootID)
}

func (invocation *commandInvocation) loadTree(ctx context.Context, cmd *cli.Command) ([]*workflowy.Item, error) {
	if invocation.usesBackup() {
		return invocation.backupItems, nil
	}
	return loadTreeWithBackupProvider(ctx, cmd, invocation.client, invocation.backupProvider)
}

func (invocation *commandInvocation) fetchItems(
	ctx context.Context,
	cmd *cli.Command,
	itemID string,
	depth int,
) (interface{}, error) {
	if !invocation.usesBackup() {
		return fetchItems(cmd, ctx, invocation.client, itemID, depth)
	}

	ancestorOptions, err := resolveAncestorOptions(cmd)
	if err != nil {
		return nil, err
	}
	return fetchFromTree(invocation.backupItems, itemID, depth, ancestorOptions)
}
