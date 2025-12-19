package main

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetupLogging_WritesToFile(t *testing.T) {
	previous := slog.Default()
	defer slog.SetDefault(previous)

	dir := t.TempDir()
	logPath := filepath.Join(dir, "workflowy.log")

	setupLogging("info", logPath)
	slog.Info("hello world")

	data, err := os.ReadFile(logPath)
	assert.NoError(t, err)
	assert.Contains(t, string(data), "INFO: hello world")
}
