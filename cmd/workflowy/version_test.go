package main

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRootCommand_VersionFlagMatchesVersionCommand(t *testing.T) {
	cmd := newRootCommand()

	cmd.Writer = &bytes.Buffer{}
	cmd.ErrWriter = &bytes.Buffer{}

	version = "1.2.3"
	commit = "abc123"
	date = "2026-04-02T00:00:00Z"

	err := cmd.Run(context.Background(), []string{"workflowy", "--version"})
	assert.NoError(t, err)

	versionFlagOutput := cmd.Writer.(*bytes.Buffer).String()

	cmd = newRootCommand()
	cmd.Writer = &bytes.Buffer{}
	cmd.ErrWriter = &bytes.Buffer{}

	version = "1.2.3"
	commit = "abc123"
	date = "2026-04-02T00:00:00Z"

	err = cmd.Run(context.Background(), []string{"workflowy", "version"})
	assert.NoError(t, err)

	assert.Equal(t, cmd.Writer.(*bytes.Buffer).String(), versionFlagOutput)
}

func TestRootCommand_VersionFlagIsTopLevelOnly(t *testing.T) {
	cmd := newRootCommand()
	cmd.Writer = &bytes.Buffer{}
	cmd.ErrWriter = &bytes.Buffer{}

	err := cmd.Run(context.Background(), []string{"workflowy", "get", "--version"})
	assert.Error(t, err)
	assert.Contains(t, cmd.ErrWriter.(*bytes.Buffer).String(), "flag provided but not defined: -version")
}
