package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v3"
)

func TestFetchItems_NilClient_ExplicitGetMethod_ReturnsError(t *testing.T) {
	cmd := &cli.Command{
		Flags: getMethodFlags(),
		Action: func(ctx context.Context, c *cli.Command) error {
			_, err := fetchItems(c, ctx, nil, "None", 2)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "cannot use method 'get' without using the API")
			return nil
		},
	}
	err := cmd.Run(context.Background(), []string{"test", "--method=get"})
	assert.NoError(t, err)
}

func TestFetchItems_NilClient_ExplicitExportMethod_ReturnsError(t *testing.T) {
	cmd := &cli.Command{
		Flags: getMethodFlags(),
		Action: func(ctx context.Context, c *cli.Command) error {
			_, err := fetchItems(c, ctx, nil, "None", 2)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "cannot use method 'export' without using the API")
			return nil
		},
	}
	err := cmd.Run(context.Background(), []string{"test", "--method=export"})
	assert.NoError(t, err)
}

func TestFetchItems_NilClient_ExplicitBackupMethod_AttemptsBackup(t *testing.T) {
	cmd := &cli.Command{
		Flags: getMethodFlags(),
		Action: func(ctx context.Context, c *cli.Command) error {
			_, err := fetchItems(c, ctx, nil, "None", 2)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "cannot read backup file")
			return nil
		},
	}
	err := cmd.Run(context.Background(), []string{"test", "--method=backup", "--backup-file=/nonexistent/backup.json"})
	assert.NoError(t, err)
}

func TestFetchItems_NilClient_AutoMethod_AttemptsBackup(t *testing.T) {
	cmd := &cli.Command{
		Flags: getMethodFlags(),
		Action: func(ctx context.Context, c *cli.Command) error {
			_, err := fetchItems(c, ctx, nil, "None", 2)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "cannot read backup file")
			return nil
		},
	}
	err := cmd.Run(context.Background(), []string{"test", "--backup-file=/nonexistent/backup.json"})
	assert.NoError(t, err)
}

func TestFetchItems_IncludeAncestors_WithGetMethod_ReturnsError(t *testing.T) {
	cmd := &cli.Command{
		Flags: getFetchFlags(),
		Action: func(ctx context.Context, c *cli.Command) error {
			_, err := fetchItems(c, ctx, nil, "some-id", 2)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "cannot use ancestor options with --method=get")
			return nil
		},
	}
	err := cmd.Run(context.Background(), []string{"test", "--method=get", "--include-ancestors"})
	assert.NoError(t, err)
}
