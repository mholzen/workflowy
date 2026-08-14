package main

import (
	"bytes"
	"context"
	"testing"

	genericclient "github.com/mholzen/workflowy/pkg/client"
	"github.com/mholzen/workflowy/pkg/workflowy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingBackupProvider struct {
	filename string
	items    []*workflowy.Item
	calls    int
}

func (provider *recordingBackupProvider) ReadBackupFile(filename string) ([]*workflowy.Item, error) {
	provider.calls++
	provider.filename = filename
	return provider.items, nil
}

func (provider *recordingBackupProvider) ReadLatestBackup() ([]*workflowy.Item, error) {
	provider.calls++
	return provider.items, nil
}

func TestCountReportBackupUploadUsesSelectedAPI(t *testing.T) {
	t.Setenv("WORKFLOWY_API_KEY", "test-key")

	backupProvider := &recordingBackupProvider{items: []*workflowy.Item{
		{
			ID:   "00000000-0000-0000-0000-000000000001",
			Name: "Project",
			Children: []*workflowy.Item{
				{ID: "00000000-0000-0000-0000-000000000002", Name: "Task"},
			},
		},
	}}
	betaClient := &MockClient{}
	var selected workflowy.APIDeployment
	factory := func(deployment workflowy.APIDeployment, _ ...genericclient.Option) (workflowy.Client, error) {
		selected = deployment
		return betaClient, nil
	}

	command := getCountReportCommandWithDeps(
		ReportDeps{BackupProvider: backupProvider, Output: &bytes.Buffer{}},
		newClientProvider(factory, true),
	)
	err := command.Run(context.Background(), []string{
		"count",
		"--api=beta",
		"--method=backup",
		"--backup-file=fixture.json",
		"--upload",
		"--threshold=0",
	})

	require.NoError(t, err)
	assert.Equal(t, "fixture.json", backupProvider.filename)
	assert.Equal(t, 1, backupProvider.calls)
	assert.Equal(t, workflowy.BetaAPI, selected)
	assert.NotEmpty(t, betaClient.CreatedNodes)
}
