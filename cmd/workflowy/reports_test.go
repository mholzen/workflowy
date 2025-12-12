package main

import (
	"context"
	"testing"

	"github.com/mholzen/workflowy/pkg/workflowy"
	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v3"
)

func TestLoadTree_NilClient_AutoFallbackToBackup(t *testing.T) {
	cmd := &cli.Command{
		Flags: getMethodFlags(),
		Action: func(ctx context.Context, c *cli.Command) error {
			_, err := loadTree(ctx, c, nil)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "cannot read backup file")
			return nil
		},
	}
	err := cmd.Run(context.Background(), []string{"test", "--backup-file=/nonexistent/backup.json"})
	assert.NoError(t, err)
}

func TestLoadTree_NilClient_ExplicitExportMethod_ReturnsError(t *testing.T) {
	cmd := &cli.Command{
		Flags: getMethodFlags(),
		Action: func(ctx context.Context, c *cli.Command) error {
			_, err := loadTree(ctx, c, nil)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "cannot use 'export' without an API client")
			return nil
		},
	}
	err := cmd.Run(context.Background(), []string{"test", "--method=export"})
	assert.NoError(t, err)
}

func TestLoadTree_NilClient_ExplicitBackupMethod_AttemptsBackup(t *testing.T) {
	cmd := &cli.Command{
		Flags: getMethodFlags(),
		Action: func(ctx context.Context, c *cli.Command) error {
			_, err := loadTree(ctx, c, nil)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "cannot read backup file")
			return nil
		},
	}
	err := cmd.Run(context.Background(), []string{"test", "--method=backup", "--backup-file=/nonexistent/backup.json"})
	assert.NoError(t, err)
}

func TestUploadReport_NilClient_ReturnsError(t *testing.T) {
	cmd := &cli.Command{
		Flags: getReportFlags(),
		Action: func(ctx context.Context, c *cli.Command) error {
			err := uploadReport(ctx, c, nil, &mockReport{})
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "cannot upload a report without an API client")
			return nil
		},
	}
	err := cmd.Run(context.Background(), []string{"test"})
	assert.NoError(t, err)
}

type mockReport struct{}

func (m *mockReport) ToNodes() (*workflowy.Item, error) {
	return &workflowy.Item{Name: "mock"}, nil
}

func (m *mockReport) Title() string {
	return "Mock Report"
}
