package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mholzen/workflowy/pkg/workflowy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

const (
	backupRootID = "11111111-1111-1111-1111-aaaaaaaaaaaa"
	backupItemID = "22222222-2222-2222-2222-bbbbbbbbbbbb"
)

type networkRejectingClient struct {
	MockClient
	networkCalls int
	mutations    int
}

func (client *networkRejectingClient) rejectNetworkCall() error {
	client.networkCalls++
	return fmt.Errorf("unexpected network call")
}

func (client *networkRejectingClient) ListTargets(context.Context) (*workflowy.ListTargetsResponse, error) {
	return nil, client.rejectNetworkCall()
}

func (client *networkRejectingClient) GetItem(context.Context, string) (*workflowy.Item, error) {
	return nil, client.rejectNetworkCall()
}

func (client *networkRejectingClient) ExportNodesWithCache(context.Context, bool) (*workflowy.ExportNodesResponse, error) {
	return nil, client.rejectNetworkCall()
}

func (client *networkRejectingClient) ListChildrenRecursiveWithOptions(context.Context, string, workflowy.RecursiveFetchOptions) (*workflowy.RecursiveFetchResult, error) {
	return nil, client.rejectNetworkCall()
}

func (client *networkRejectingClient) DeleteNode(context.Context, string) (*workflowy.UpdateNodeResponse, error) {
	client.mutations++
	return &workflowy.UpdateNodeResponse{}, nil
}

func (client *networkRejectingClient) CompleteNode(context.Context, string) (*workflowy.UpdateNodeResponse, error) {
	client.mutations++
	return &workflowy.UpdateNodeResponse{}, nil
}

func (client *networkRejectingClient) UncompleteNode(context.Context, string) (*workflowy.UpdateNodeResponse, error) {
	client.mutations++
	return &workflowy.UpdateNodeResponse{}, nil
}

func TestGetBackupResolvesIDsAndRestrictionsWithoutNetwork(t *testing.T) {
	backupFile := writeCommandBackup(t)
	client := &networkRejectingClient{}
	command := getGetCommandWithClientProvider(withMockClient(client))
	root := commandRoot(command)

	err := root.Run(context.Background(), []string{
		"workflowy", "get", backupItemID[len(backupItemID)-12:],
		"--method=backup", "--backup-file=" + backupFile,
		"--read-root-id=" + backupRootID[len(backupRootID)-12:],
		"--depth=0",
	})

	require.NoError(t, err)
	assert.Zero(t, client.networkCalls)
}

func TestGetDoesNotReplaceExplicitNetworkMethodWithBackup(t *testing.T) {
	backupFile := writeCommandBackup(t)
	for _, method := range []string{"get", "export"} {
		t.Run(method, func(t *testing.T) {
			command := getGetCommandWithClientProvider(withMockClient(nil))
			root := commandRoot(command)

			err := root.Run(context.Background(), []string{
				"workflowy", "get", "--method=" + method,
				"--backup-file=" + backupFile, "--depth=1",
			})

			require.ErrorContains(t, err, "without using the API")
		})
	}
}

func TestGetRejectsInvalidMethodBeforePreparingRestrictions(t *testing.T) {
	command := getGetCommandWithClientProvider(withMockClient(nil))
	root := commandRoot(command)

	err := root.Run(context.Background(), []string{
		"workflowy", "get", "--method=invalid",
		"--read-root-id=" + backupRootID,
	})

	require.EqualError(t, err, `Cannot select access method "invalid": expected "get", "export", or "backup"`)
}

func TestCountReportBackupResolvesIDsAndRestrictionsWithoutNetwork(t *testing.T) {
	client := &networkRejectingClient{}
	provider := &recordingBackupProvider{items: commandBackupItems()}
	command := getCountReportCommandWithDeps(
		ReportDeps{BackupProvider: provider, Output: &bytes.Buffer{}},
		withMockClient(client),
	)
	root := commandRoot(command)

	err := root.Run(context.Background(), []string{
		"workflowy", "count",
		"--method=backup", "--id=" + backupItemID[len(backupItemID)-12:],
		"--read-root-id=" + backupRootID[len(backupRootID)-12:], "--threshold=0",
	})

	require.NoError(t, err)
	assert.Zero(t, client.networkCalls)
	assert.Equal(t, 1, provider.calls)
}

func TestReplaceBackupDryRunUsesBackupForValidationWithoutNetwork(t *testing.T) {
	backupFile := writeCommandBackup(t)
	client := &networkRejectingClient{}
	command := getReplaceCommandWithClientProvider(withMockClient(client))
	root := commandRoot(command)

	err := root.Run(context.Background(), []string{
		"workflowy", "replace", "Task", "Done",
		"--method=backup", "--backup-file=" + backupFile, "--dry-run",
		"--parent-id=" + backupRootID[len(backupRootID)-12:],
		"--write-root-id=" + backupRootID[len(backupRootID)-12:],
	})

	require.NoError(t, err)
	assert.Zero(t, client.networkCalls)
}

func TestTransformBackupDryRunUsesBackupForValidationWithoutNetwork(t *testing.T) {
	backupFile := writeCommandBackup(t)
	client := &networkRejectingClient{}
	command := getTransformCommandWithClientProvider(withMockClient(client))
	root := commandRoot(command)

	err := root.Run(context.Background(), []string{
		"workflowy", "transform", backupItemID[len(backupItemID)-12:], "uppercase",
		"--method=backup", "--backup-file=" + backupFile, "--dry-run", "--depth=0",
		"--write-root-id=" + backupRootID[len(backupRootID)-12:],
	})

	require.NoError(t, err)
	assert.Zero(t, client.networkCalls)
}

func TestBackupMutationCommandsValidateFromBackupBeforeSelectedAPIWrite(t *testing.T) {
	backupFile := writeCommandBackup(t)
	tests := []struct {
		name  string
		build func(ClientProvider) *cli.Command
	}{
		{name: "delete", build: getDeleteCommandWithClientProvider},
		{name: "complete", build: func(provider ClientProvider) *cli.Command {
			return getCompletionCommandWithClientProvider("complete", "test", "testing", provider)
		}},
		{name: "uncomplete", build: func(provider ClientProvider) *cli.Command {
			return getCompletionCommandWithClientProvider("uncomplete", "test", "testing", provider)
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := &networkRejectingClient{}
			command := test.build(withMockClient(client))
			root := commandRoot(command)

			err := root.Run(context.Background(), []string{
				"workflowy", test.name, backupItemID[len(backupItemID)-12:],
				"--method=backup", "--backup-file=" + backupFile,
				"--write-root-id=" + backupRootID[len(backupRootID)-12:],
			})

			require.NoError(t, err)
			assert.Zero(t, client.networkCalls)
			assert.Equal(t, 1, client.mutations)
		})
	}
}

func commandRoot(command *cli.Command) *cli.Command {
	return &cli.Command{
		Name: "workflowy",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "format", Value: "json"},
			getReadRootIdFlag(),
			getWriteRootIdFlag(),
		},
		Commands: []*cli.Command{command},
	}
}

func writeCommandBackup(t *testing.T) string {
	t.Helper()
	filename := filepath.Join(t.TempDir(), "workflowy.backup")
	data := `[{"id":"` + backupRootID + `","nm":"Project","ch":[{"id":"` + backupItemID + `","nm":"Task"}]}]`
	require.NoError(t, os.WriteFile(filename, []byte(data), 0o600))
	return filename
}

func commandBackupItems() []*workflowy.Item {
	return []*workflowy.Item{{
		ID: backupRootID, Name: "Project",
		Children: []*workflowy.Item{{ID: backupItemID, Name: "Task"}},
	}}
}
