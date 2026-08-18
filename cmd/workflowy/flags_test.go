package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

func TestGetWriteFlags_IncludesAPIKeyFile(t *testing.T) {
	flags := getWriteFlags()

	var found bool
	for _, f := range flags {
		if sf, ok := f.(*cli.StringFlag); ok && sf.Name == "api-key-file" {
			found = true
			homeDir, _ := os.UserHomeDir()
			expectedDefault := filepath.Join(homeDir, ".workflowy", "api.key")
			assert.Equal(t, expectedDefault, sf.Value, "api-key-file should have correct default value")
		}
	}
	assert.True(t, found, "getWriteFlags should include api-key-file flag")
}

func TestUpdateCommand_HasAPIKeyFileFlag(t *testing.T) {
	var apiKeyFile string

	cmd := &cli.Command{
		Flags: getWriteFlags(),
		Action: func(ctx context.Context, c *cli.Command) error {
			apiKeyFile = c.String("api-key-file")
			return nil
		},
	}

	err := cmd.Run(context.Background(), []string{"test"})
	assert.NoError(t, err)

	homeDir, _ := os.UserHomeDir()
	expectedDefault := filepath.Join(homeDir, ".workflowy", "api.key")
	assert.Equal(t, expectedDefault, apiKeyFile, "api-key-file should have default value from getWriteFlags")
}

func TestGetMethodFlags_IncludesAPIKeyFile(t *testing.T) {
	flags := getMethodFlags()

	var found bool
	for _, f := range flags {
		if sf, ok := f.(*cli.StringFlag); ok && sf.Name == "api-key-file" {
			found = true
			homeDir, _ := os.UserHomeDir()
			expectedDefault := filepath.Join(homeDir, ".workflowy", "api.key")
			assert.Equal(t, expectedDefault, sf.Value, "api-key-file should have correct default value")
		}
	}
	assert.True(t, found, "getMethodFlags should include api-key-file flag")
}

func TestPaginationRequestedOnlyForExplicitFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{name: "omitted", args: []string{"test"}, want: false},
		{name: "limit", args: []string{"test", "--limit=50"}, want: true},
		{name: "explicit zero offset", args: []string{"test", "--offset=0"}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got bool
			cmd := &cli.Command{
				Flags: getPaginationFlags(),
				Action: func(_ context.Context, command *cli.Command) error {
					got = paginationRequested(command)
					return nil
				},
			}
			require.NoError(t, cmd.Run(context.Background(), tt.args))
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFetchParamsRejectPaginationWithAncestorOptions(t *testing.T) {
	cmd := &cli.Command{
		Flags: append(getFetchFlags(), &cli.StringFlag{Name: "format", Value: "list"}),
		Action: func(_ context.Context, command *cli.Command) error {
			_, err := getAndValidateFetchParams(command)
			assert.EqualError(t, err, "cannot combine pagination with ancestor options")
			return nil
		},
	}
	require.NoError(t, cmd.Run(context.Background(), []string{"test", "--limit=10", "--include-ancestors"}))
}
