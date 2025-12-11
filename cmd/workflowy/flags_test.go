package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
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
