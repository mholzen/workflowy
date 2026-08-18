package main

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/mholzen/workflowy/pkg/workflowy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

func paginatedTestItems() []*workflowy.Item {
	return []*workflowy.Item{
		{ID: "a", Name: "Alpha"},
		{ID: "b", Name: "Bravo"},
		{ID: "c", Name: "Charlie"},
	}
}

// runPaginated renders items through the real flag plumbing and returns what
// each stream received.
func runPaginated(t *testing.T, items []*workflowy.Item, format string, args []string) (stdout, stderr string) {
	t.Helper()

	originalOut, originalErr := os.Stdout, os.Stderr
	outReader, outWriter, err := os.Pipe()
	require.NoError(t, err)
	errReader, errWriter, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout, os.Stderr = outWriter, errWriter
	defer func() { os.Stdout, os.Stderr = originalOut, originalErr }()

	cmd := &cli.Command{
		Flags: getPaginationFlags(),
		Action: func(_ context.Context, command *cli.Command) error {
			return printPaginated(command, items, format, nil)
		},
	}
	runErr := cmd.Run(context.Background(), args)

	require.NoError(t, outWriter.Close())
	require.NoError(t, errWriter.Close())
	os.Stdout, os.Stderr = originalOut, originalErr

	outBytes, err := io.ReadAll(outReader)
	require.NoError(t, err)
	errBytes, err := io.ReadAll(errReader)
	require.NoError(t, err)
	require.NoError(t, runErr)

	return string(outBytes), string(errBytes)
}

// Paginating used to hand printOutput a Page, which no format knew how to
// render, so every window silently fell back to JSON.
func TestPrintPaginatedHonoursTheRequestedFormat(t *testing.T) {
	stdout, stderr := runPaginated(t, paginatedTestItems(), "list", []string{"test", "--limit=2"})

	assert.Equal(t, "- Alpha\n- Bravo\n", stdout)
	assert.NotContains(t, stdout, `"items"`)
	assert.Equal(t, "# 1-2 of 3 (next offset: 2)\n", stderr)
}

func TestPrintPaginatedKeepsTheEnvelopeForJSON(t *testing.T) {
	stdout, stderr := runPaginated(t, paginatedTestItems(), "json", []string{"test", "--limit=2"})

	assert.Contains(t, stdout, `"items"`)
	assert.Contains(t, stdout, `"has_more": true`)
	assert.Contains(t, stdout, `"next_offset": 2`)
	assert.Empty(t, stderr, "the envelope already carries the window")
}

func TestPrintPaginatedReportsAnExhaustedWindow(t *testing.T) {
	_, stderr := runPaginated(t, paginatedTestItems(), "list", []string{"test", "--offset=9"})

	assert.Equal(t, "# no results at offset 9 of 3\n", stderr)
}

func TestPageSummaryOmitsNextOffsetOnTheLastPage(t *testing.T) {
	page, err := workflowy.NewPage(paginatedTestItems(), 2, 2)
	require.NoError(t, err)

	assert.Equal(t, "# 3-3 of 3", pageSummary(page))
}

func TestPageContentWrapsItemsForTheOutlineFormats(t *testing.T) {
	page, err := workflowy.NewPage(paginatedTestItems(), 2, 0)
	require.NoError(t, err)

	list, ok := pageContent(page).(*workflowy.ListChildrenResponse)
	require.True(t, ok, "printOutput renders items as a list response, not a bare slice")
	assert.Len(t, list.Items, 2)
}
