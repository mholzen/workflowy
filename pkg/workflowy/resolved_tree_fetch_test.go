package workflowy

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testItem(id string, children ...*Item) *Item {
	return &Item{ID: id, Name: id, Data: map[string]interface{}{}, Children: children}
}

func testMirror(id, originID string, children ...*Item) *Item {
	return &Item{
		ID:       id,
		Name:     id,
		Data:     map[string]interface{}{"mirror": map[string]interface{}{"origin_id": originID}},
		Children: children,
	}
}

func resolvedFetchOptions(depth int) ResolveOptions {
	return ResolveOptions{ResolveMirrors: true, Depth: depth, Operation: "get"}
}

func TestResolvedTreeFetchUsesResolvedChildRules(t *testing.T) {
	originGrandchild := testItem("origin-grandchild")
	originChild := testItem("origin-child", originGrandchild)
	origin := testItem("origin", originChild)
	sourceChild := testItem("source-child")
	mirror := testMirror("mirror", origin.ID, sourceChild)
	nullChild := testItem("null-child")
	nullMirror := &Item{ID: "null-mirror", Name: "null-mirror", Data: map[string]interface{}{
		"mirror": map[string]interface{}{"origin_id": nil},
	}, Children: []*Item{nullChild}}
	malformedChild := testItem("malformed-child")
	malformedMirror := &Item{ID: "malformed-mirror", Name: "malformed-mirror", Data: map[string]interface{}{
		"mirror": map[string]interface{}{"origin_id": 12},
	}, Children: []*Item{malformedChild}}
	missingChild := testItem("missing-child")
	missingMirror := testMirror("missing-mirror", "absent-origin", missingChild)

	tree := NewResolvedTree([]*Item{origin, mirror, nullMirror, malformedMirror, missingMirror}, "test export")

	enabled, err := tree.Fetch("None", "None", resolvedFetchOptions(1))
	require.NoError(t, err)
	require.Len(t, enabled.Items, 5)
	assert.Equal(t, []string{"origin-child"}, itemIDs(enabled.Items[0].Children))
	assert.Equal(t, []string{"origin-child"}, itemIDs(enabled.Items[1].Children), "origin children replace source children")
	assert.Equal(t, []string{"null-child"}, itemIDs(enabled.Items[2].Children))
	assert.Equal(t, []string{"malformed-child"}, itemIDs(enabled.Items[3].Children))
	assert.Equal(t, []string{"missing-child"}, itemIDs(enabled.Items[4].Children))
	assert.Equal(t, 1, enabled.Summary.Resolved)
	assert.Equal(t, 1, enabled.Summary.MissingOrigin)
	assert.Equal(t, 1, enabled.Summary.MalformedMetadata)

	disabled, err := tree.Fetch("None", "None", ResolveOptions{Depth: 1, Operation: "get"})
	require.NoError(t, err)
	require.Len(t, disabled.Items, 5)
	assert.Equal(t, []string{"origin-child"}, itemIDs(disabled.Items[0].Children))
	for _, item := range disabled.Items[1:] {
		assert.Empty(t, item.Children, "%s should be a leaf", item.ID)
	}

	_, err = tree.Fetch("None", sourceChild.ID, resolvedFetchOptions(-1))
	require.EqualError(t, err, `Cannot find Workflowy node "source-child" within resolved read scope "None" from test export`)
}

func TestResolvedTreeFetchHonorsDepthAndDoesNotMutateSource(t *testing.T) {
	grandchild := testItem("grandchild")
	child := testItem("child", grandchild)
	root := testItem("root", child)
	tree := NewResolvedTree([]*Item{root}, "test backup")

	before, err := json.Marshal(root)
	require.NoError(t, err)

	depthZero, err := tree.Fetch("None", root.ID, resolvedFetchOptions(0))
	require.NoError(t, err)
	assert.Empty(t, depthZero.Item.Children)

	finite, err := tree.Fetch("None", root.ID, resolvedFetchOptions(1))
	require.NoError(t, err)
	require.Len(t, finite.Item.Children, 1)
	assert.Empty(t, finite.Item.Children[0].Children)

	unlimited, err := tree.Fetch("None", root.ID, resolvedFetchOptions(-1))
	require.NoError(t, err)
	require.Len(t, unlimited.Item.Children[0].Children, 1)
	assert.NotSame(t, root, unlimited.Item)
	assert.NotSame(t, child, unlimited.Item.Children[0])

	FlattenTree(unlimited.Item)
	FilterEmptyItem(finite.Item)
	after, err := json.Marshal(root)
	require.NoError(t, err)
	assert.JSONEq(t, string(before), string(after))

	rootDepthZero, err := tree.Fetch("None", "None", resolvedFetchOptions(0))
	require.NoError(t, err)
	require.Len(t, rootDepthZero.Items, 1)
	assert.Empty(t, rootDepthZero.Items[0].Children)
}

func TestResolvedTreeFetchDoesNotTraverseUnrequestedCycles(t *testing.T) {
	target := testItem("target")
	cycleRoot := testItem("cycle-root")
	cycleRoot.Children = []*Item{testMirror("cycle-mirror", cycleRoot.ID)}
	tree := NewResolvedTree([]*Item{target, cycleRoot}, "test export")

	result, err := tree.Fetch("None", target.ID, resolvedFetchOptions(-1))
	require.NoError(t, err)
	assert.Zero(t, result.Summary.Cycles)

	rootResult, err := tree.Fetch("None", "None", resolvedFetchOptions(0))
	require.NoError(t, err)
	assert.Zero(t, rootResult.Summary.Cycles)
}

func TestResolvedTreeFetchPrefersReachableOriginalOccurrence(t *testing.T) {
	target := testItem("target")
	origin := testItem("origin", target)
	firstMirror := testMirror("first-mirror", origin.ID)
	secondMirror := testMirror("second-mirror", origin.ID)
	tree := NewResolvedTree([]*Item{firstMirror, secondMirror, origin}, "test export")

	result, err := tree.Fetch("None", target.ID, resolvedFetchOptions(0))
	require.NoError(t, err)
	assert.False(t, result.Occurrence.ViaMirror)
	assert.Equal(t, []string{origin.ID}, itemIDs(result.Occurrence.Ancestors))
}

func TestResolvedTreeFetchSelectsFirstMirrorOccurrenceWhenOriginalIsOutsideScope(t *testing.T) {
	target := testItem("target")
	origin := testItem("origin", target)
	firstMirror := testMirror("first-mirror", origin.ID)
	secondMirror := testMirror("second-mirror", origin.ID)
	scope := testItem("scope", firstMirror, secondMirror)
	tree := NewResolvedTree([]*Item{scope, origin}, "test backup")

	result, err := tree.Fetch(scope.ID, target.ID, resolvedFetchOptions(0))
	require.NoError(t, err)
	assert.True(t, result.Occurrence.ViaMirror)
	assert.Equal(t, []string{scope.ID, firstMirror.ID}, itemIDs(result.Occurrence.Ancestors))
}

func TestResolvedTreeFetchCarriesSelectionPathIntoMaterialization(t *testing.T) {
	backToOrigin := testMirror("back-to-origin", "origin")
	target := testItem("target", backToOrigin)
	origin := testItem("origin", target)
	entryMirror := testMirror("entry-mirror", origin.ID)
	scope := testItem("scope", entryMirror)
	tree := NewResolvedTree([]*Item{scope, origin}, "test export")

	result, err := tree.Fetch(scope.ID, target.ID, resolvedFetchOptions(-1))
	require.NoError(t, err)
	require.Len(t, result.Item.Children, 1)
	assert.Empty(t, result.Item.Children[0].Children)
	assert.Equal(t, 1, result.Summary.Resolved)
	assert.Equal(t, 1, result.Summary.Cycles)
}

func itemIDs(items []*Item) []string {
	ids := make([]string, len(items))
	for index, item := range items {
		ids[index] = item.ID
	}
	return ids
}
