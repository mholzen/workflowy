package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mholzen/workflowy/pkg/workflowy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

func TestCommandsExposeAPIFlag(t *testing.T) {
	commands := make(map[string]*cli.Command)
	for _, command := range getCommands() {
		commands[command.Name] = command
		for _, subcommand := range command.Commands {
			commands[command.Name+" "+subcommand.Name] = subcommand
		}
	}

	requiredPaths := []string{
		"get",
		"list",
		"create",
		"update",
		"move",
		"delete",
		"complete",
		"uncomplete",
		"targets",
		"search",
		"replace",
		"transform",
		"id",
		"report count",
		"report children",
		"report created",
		"report modified",
		"report mirrors",
	}

	for _, path := range requiredPaths {
		t.Run(path, func(t *testing.T) {
			command := commands[path]
			require.NotNil(t, command)

			apiFlag := findStringFlag(command.Flags, "api")
			require.NotNil(t, apiFlag, "%s must expose --api", path)
			assert.Equal(t, string(workflowy.ProductionAPI), apiFlag.Value)
		})
	}
}

func TestCommandsWithoutAPIFlag(t *testing.T) {
	commands := make(map[string]*cli.Command)
	for _, command := range getCommands() {
		commands[command.Name] = command
	}

	assert.Nil(t, findStringFlag(commands["mcp"].Flags, "api"), "mcp is added atomically with MCP config")
	assert.Nil(t, findStringFlag(commands["version"].Flags, "api"))
}

func findStringFlag(flags []cli.Flag, name string) *cli.StringFlag {
	for _, flag := range flags {
		stringFlag, ok := flag.(*cli.StringFlag)
		if ok && stringFlag.Name == name {
			return stringFlag
		}
	}
	return nil
}

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
