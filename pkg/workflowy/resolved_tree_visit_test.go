package workflowy

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNodeOccurrenceIdentityEqualityAndSnapshot(t *testing.T) {
	ancestor := testItem("ancestor")
	mirror := testMirror("mirror", "origin")
	occurrence := NodeOccurrence{Item: mirror, Ancestors: []*Item{ancestor}, ViaMirror: true}

	assert.Equal(t, "ancestor/mirror", occurrence.Identity())
	assert.Equal(t, "origin", occurrence.EqualityID())

	snapshot := occurrence.Snapshot()
	occurrence.Ancestors[0] = testItem("changed")
	assert.Equal(t, "ancestor/mirror", snapshot.Identity())
	assert.Equal(t, "plain", (NodeOccurrence{Item: testItem("plain")}).EqualityID())
}

func TestResolvedTreeVisitReportsPathsAndMirrorDescendants(t *testing.T) {
	grandchild := testItem("grandchild")
	child := testItem("child", grandchild)
	origin := testItem("origin", child)
	mirror := testMirror("mirror", origin.ID)
	tree := NewResolvedTree([]*Item{origin, mirror}, "test export")

	occurrences := make([]NodeOccurrence, 0)
	summary, err := tree.Visit("None", "None", resolvedFetchOptions(-1), func(occurrence NodeOccurrence) (OccurrenceVisitResult, error) {
		occurrences = append(occurrences, occurrence.Snapshot())
		return OccurrenceVisitResult{RetainedOccurrences: len(occurrences)}, nil
	})
	require.NoError(t, err)
	assert.Equal(t, 1, summary.Resolved)

	byIdentity := make(map[string]NodeOccurrence)
	for _, occurrence := range occurrences {
		byIdentity[occurrence.Identity()] = occurrence
	}
	assert.False(t, byIdentity["origin/child"].ViaMirror)
	assert.True(t, byIdentity["mirror/child"].ViaMirror)
	assert.True(t, byIdentity["mirror/child/grandchild"].ViaMirror)
	assert.Equal(t, []string{"mirror", "child"}, itemIDs(byIdentity["mirror/child/grandchild"].Ancestors))
}

func TestResolvedTreeVisitUsesPreferredTargetOccurrence(t *testing.T) {
	target := testItem("target", testItem("descendant"))
	origin := testItem("origin", target)
	firstMirror := testMirror("first-mirror", origin.ID)
	secondMirror := testMirror("second-mirror", origin.ID)
	tree := NewResolvedTree([]*Item{firstMirror, secondMirror, origin}, "test export")

	var identities []string
	_, err := tree.Visit("None", target.ID, resolvedFetchOptions(-1), func(occurrence NodeOccurrence) (OccurrenceVisitResult, error) {
		identities = append(identities, occurrence.Identity())
		return OccurrenceVisitResult{}, nil
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"origin/target", "origin/target/descendant"}, identities)

	scope := testItem("scope", firstMirror, secondMirror)
	scopedTree := NewResolvedTree([]*Item{scope, origin}, "test export")
	identities = nil
	_, err = scopedTree.Visit(scope.ID, target.ID, resolvedFetchOptions(0), func(occurrence NodeOccurrence) (OccurrenceVisitResult, error) {
		identities = append(identities, occurrence.Identity())
		return OccurrenceVisitResult{}, nil
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"scope/first-mirror/target"}, identities)
}

func TestResolvedTreeVisitStopsCyclesPerBranch(t *testing.T) {
	selfMirror := testMirror("self-mirror", "origin")
	origin := testItem("origin", selfMirror)
	firstEntry := testMirror("first-entry", origin.ID)
	secondEntry := testMirror("second-entry", origin.ID)
	tree := NewResolvedTree([]*Item{origin, firstEntry, secondEntry}, "test backup")

	var identities []string
	summary, err := tree.Visit("None", "None", resolvedFetchOptions(-1), func(occurrence NodeOccurrence) (OccurrenceVisitResult, error) {
		identities = append(identities, occurrence.Identity())
		return OccurrenceVisitResult{}, nil
	})
	require.NoError(t, err)
	assert.Equal(t, 3, summary.Cycles)
	assert.Contains(t, identities, "origin/self-mirror")
	assert.Contains(t, identities, "first-entry/self-mirror")
	assert.Contains(t, identities, "second-entry/self-mirror")
	assert.NotContains(t, identities, "origin/self-mirror/self-mirror")
}

func TestResolvedTreeVisitDoesNotGloballySuppressIndependentOccurrences(t *testing.T) {
	child := testItem("child")
	origin := testItem("origin", child)
	tree := NewResolvedTree([]*Item{testMirror("one", origin.ID), testMirror("two", origin.ID), origin}, "test export")

	visits := 0
	_, err := tree.Visit("None", "None", resolvedFetchOptions(-1), func(occurrence NodeOccurrence) (OccurrenceVisitResult, error) {
		if occurrence.Item.ID == child.ID {
			visits++
		}
		return OccurrenceVisitResult{}, nil
	})
	require.NoError(t, err)
	assert.Equal(t, 3, visits)
}

func TestResolvedTreeVisitAllowsLaterPathWithDifferentCycleBoundary(t *testing.T) {
	backToRoot := testMirror("back-to-root", "root")
	intermediate := testItem("intermediate", backToRoot)
	root := testItem("root", intermediate)
	entry := testMirror("entry", intermediate.ID)
	tree := NewResolvedTree([]*Item{root, entry}, "test export")

	var identities []string
	summary, err := tree.Visit("None", "None", resolvedFetchOptions(-1), func(occurrence NodeOccurrence) (OccurrenceVisitResult, error) {
		identities = append(identities, occurrence.Identity())
		return OccurrenceVisitResult{}, nil
	})
	require.NoError(t, err)
	assert.Equal(t, 2, summary.Cycles)
	assert.Contains(t, identities, "root/intermediate/back-to-root")
	assert.NotContains(t, identities, "root/intermediate/back-to-root/intermediate")
	assert.Contains(t, identities, "entry/back-to-root/intermediate")
	assert.Contains(t, identities, "entry/back-to-root/intermediate/back-to-root")
}

func TestResolvedTreeVisitStopsNestedMirrorCycle(t *testing.T) {
	backToFirst := testMirror("back-to-first", "first")
	second := testItem("second", backToFirst)
	toSecond := testMirror("to-second", second.ID)
	first := testItem("first", toSecond)
	tree := NewResolvedTree([]*Item{first, second}, "test backup")

	var identities []string
	summary, err := tree.Visit("first", "None", resolvedFetchOptions(-1), func(occurrence NodeOccurrence) (OccurrenceVisitResult, error) {
		identities = append(identities, occurrence.Identity())
		return OccurrenceVisitResult{}, nil
	})
	require.NoError(t, err)
	assert.Equal(t, 1, summary.Resolved)
	assert.Equal(t, 1, summary.Cycles)
	assert.Equal(t, []string{"first", "first/to-second", "first/to-second/back-to-first"}, identities)
}

func TestResolvedTreeVisitStopsOnVisitorErrorWithContext(t *testing.T) {
	tree := NewResolvedTree([]*Item{testItem("root", testItem("child"))}, "test export")
	want := errors.New("matcher failed")

	_, err := tree.Visit("None", "None", resolvedFetchOptions(-1), func(occurrence NodeOccurrence) (OccurrenceVisitResult, error) {
		if occurrence.Item.ID == "child" {
			return OccurrenceVisitResult{}, want
		}
		return OccurrenceVisitResult{}, nil
	})

	require.ErrorIs(t, err, want)
	assert.EqualError(t, err, `Cannot visit Workflowy node "child" during get from test export: matcher failed`)
}
