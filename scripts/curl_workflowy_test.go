package scripts_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCurlWorkflowy_UsesDefaultAPIKeyAndPassesCurlFlags(t *testing.T) {
	root, err := filepath.Abs("..")
	require.NoError(t, err)

	home := t.TempDir()
	apiKeyDir := filepath.Join(home, ".workflowy")
	require.NoError(t, os.MkdirAll(apiKeyDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(apiKeyDir, "api.key"), []byte("test-api-key\n"), 0o600))

	binDir := filepath.Join(t.TempDir(), "bin")
	require.NoError(t, os.MkdirAll(binDir, 0o755))

	argsFile := filepath.Join(t.TempDir(), "curl-args.txt")
	curlStub := filepath.Join(binDir, "curl")
	stub := "#!/bin/sh\nprintf '%s\n' \"$@\" > \"$CURL_ARGS_FILE\"\nprintf 'stub response\\n'"
	require.NoError(t, os.WriteFile(curlStub, []byte(stub), 0o755))

	cmd := exec.Command(filepath.Join(root, "scripts", "curl-workflowy"), "/nodes/test-id", "-X", "POST", "-d", `{"name":"Test"}`)
	cmd.Env = append(os.Environ(),
		"HOME="+home,
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"CURL_ARGS_FILE="+argsFile,
	)

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
	assert.Equal(t, "stub response\n", string(output))

	argsData, err := os.ReadFile(argsFile)
	require.NoError(t, err)

	args := strings.Split(strings.TrimSpace(string(argsData)), "\n")
	assert.Equal(t, []string{
		"-sS",
		"-H",
		"Authorization: Bearer test-api-key",
		"-X",
		"POST",
		"-d",
		`{"name":"Test"}`,
		"https://workflowy.com/api/v1/nodes/test-id",
	}, args)
}
