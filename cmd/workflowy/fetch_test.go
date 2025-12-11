package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v3"
)

func TestFetchItems_NilClient_ExplicitGetMethod_ReturnsError(t *testing.T) {
	cmd := &cli.Command{
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "method", Value: "get"},
			&cli.StringFlag{Name: "backup-file"},
			&cli.BoolFlag{Name: "force-refresh"},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			_, err := fetchItems(c, ctx, nil, "None", 2)
			assert.Error(t, err)
			return nil
		},
	}
	err := cmd.Run(context.Background(), []string{"test"})
	assert.NoError(t, err)
}

func TestFetchItems_NilClient_ExplicitExportMethod_ReturnsError(t *testing.T) {
	cmd := &cli.Command{
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "method", Value: "export"},
			&cli.StringFlag{Name: "backup-file"},
			&cli.BoolFlag{Name: "force-refresh"},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			_, err := fetchItems(c, ctx, nil, "None", 2)
			assert.Error(t, err)
			return nil
		},
	}
	err := cmd.Run(context.Background(), []string{"test"})
	assert.NoError(t, err)
}

func TestFetchItems_NilClient_ExplicitBackupMethod_AttemptsBackup(t *testing.T) {
	cmd := &cli.Command{
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "method", Value: "backup"},
			&cli.StringFlag{Name: "backup-file", Value: "/nonexistent/backup.json"},
			&cli.BoolFlag{Name: "force-refresh"},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			_, err := fetchItems(c, ctx, nil, "None", 2)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "cannot read backup file")
			return nil
		},
	}
	err := cmd.Run(context.Background(), []string{"test"})
	assert.NoError(t, err)
}

func TestFetchItems_NilClient_AutoMethod_AttemptsBackup(t *testing.T) {
	cmd := &cli.Command{
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "method"},
			&cli.StringFlag{Name: "backup-file", Value: "/nonexistent/backup.json"},
			&cli.BoolFlag{Name: "force-refresh"},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			_, err := fetchItems(c, ctx, nil, "None", 2)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "cannot read backup file")
			return nil
		},
	}
	err := cmd.Run(context.Background(), []string{"test"})
	assert.NoError(t, err)
}
