package workflowy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveNodeIDFromTree(t *testing.T) {
	items := []*Item{
		{ID: "11111111-1111-1111-1111-aaaaaaaaaaaa", Name: "First"},
		{ID: "22222222-2222-2222-2222-bbbbbbbbbbbb", Name: "Second"},
	}

	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "none", raw: "None", want: "None"},
		{name: "full UUID", raw: "11111111-1111-1111-1111-aaaaaaaaaaaa", want: "11111111-1111-1111-1111-aaaaaaaaaaaa"},
		{name: "Workflowy URL", raw: "https://workflowy.com/#/11111111-1111-1111-1111-aaaaaaaaaaaa", want: "11111111-1111-1111-1111-aaaaaaaaaaaa"},
		{name: "short ID", raw: "bbbbbbbbbbbb", want: "22222222-2222-2222-2222-bbbbbbbbbbbb"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ResolveNodeIDFromTree(items, test.raw)
			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestResolveNodeIDFromTreeReportsMissingAndAmbiguousIDs(t *testing.T) {
	items := []*Item{
		{ID: "11111111-1111-1111-1111-aaaaaaaaaaaa"},
		{ID: "22222222-2222-2222-2222-aaaaaaaaaaaa"},
	}

	_, err := ResolveNodeIDFromTree(items, "33333333-3333-3333-3333-cccccccccccc")
	require.EqualError(t, err, `Cannot resolve Workflowy node "33333333-3333-3333-3333-cccccccccccc" from tree: node was not found`)

	_, err = ResolveNodeIDFromTree(items, "aaaaaaaaaaaa")
	require.EqualError(t, err, `Cannot resolve Workflowy node "aaaaaaaaaaaa" from tree: short ID matches 2 nodes`)

	_, err = ResolveNodeIDFromTree(items, "inbox")
	require.EqualError(t, err, `Cannot resolve Workflowy node "inbox" from tree: expected a full UUID or 12-character short ID`)
}
