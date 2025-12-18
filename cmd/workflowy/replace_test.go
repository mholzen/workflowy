package main

import (
	"regexp"
	"testing"

	"github.com/mholzen/workflowy/pkg/workflowy"
	"github.com/stretchr/testify/assert"
)

func TestCollectReplacements_SimplePattern(t *testing.T) {
	items := []*workflowy.Item{
		{ID: "1", Name: "hello world"},
		{ID: "2", Name: "hello there"},
		{ID: "3", Name: "goodbye world"},
	}

	re := regexp.MustCompile("hello")
	opts := ReplaceOptions{
		Pattern:     re,
		Replacement: "hi",
		Depth:       -1,
	}

	var results []ReplaceResult
	collectReplacements(items, opts, 0, &results)

	assert.Len(t, results, 2)
	assert.Equal(t, "1", results[0].ID)
	assert.Equal(t, "hello world", results[0].OldName)
	assert.Equal(t, "hi world", results[0].NewName)
	assert.Equal(t, "2", results[1].ID)
	assert.Equal(t, "hello there", results[1].OldName)
	assert.Equal(t, "hi there", results[1].NewName)
}

func TestCollectReplacements_WithCaptureGroups(t *testing.T) {
	items := []*workflowy.Item{
		{ID: "1", Name: "task-123"},
		{ID: "2", Name: "task-456"},
		{ID: "3", Name: "other item"},
	}

	re := regexp.MustCompile(`(\w+)-(\d+)`)
	opts := ReplaceOptions{
		Pattern:     re,
		Replacement: "${2}_${1}",
		Depth:       -1,
	}

	var results []ReplaceResult
	collectReplacements(items, opts, 0, &results)

	assert.Len(t, results, 2)
	assert.Equal(t, "task-123", results[0].OldName)
	assert.Equal(t, "123_task", results[0].NewName)
	assert.Equal(t, "task-456", results[1].OldName)
	assert.Equal(t, "456_task", results[1].NewName)
}

func TestCollectReplacements_WithCaptureGroupsSpaceSeparated(t *testing.T) {
	items := []*workflowy.Item{
		{ID: "1", Name: "hello world"},
	}

	re := regexp.MustCompile(`(\w+) (\w+)`)
	opts := ReplaceOptions{
		Pattern:     re,
		Replacement: "$2 $1",
		Depth:       -1,
	}

	var results []ReplaceResult
	collectReplacements(items, opts, 0, &results)

	assert.Len(t, results, 1)
	assert.Equal(t, "hello world", results[0].OldName)
	assert.Equal(t, "world hello", results[0].NewName)
}

func TestCollectReplacements_CaseInsensitive(t *testing.T) {
	items := []*workflowy.Item{
		{ID: "1", Name: "TODO: do this"},
		{ID: "2", Name: "todo: and this"},
		{ID: "3", Name: "Todo: also this"},
	}

	re := regexp.MustCompile("(?i)todo")
	opts := ReplaceOptions{
		Pattern:     re,
		Replacement: "DONE",
		Depth:       -1,
	}

	var results []ReplaceResult
	collectReplacements(items, opts, 0, &results)

	assert.Len(t, results, 3)
	assert.Equal(t, "DONE: do this", results[0].NewName)
	assert.Equal(t, "DONE: and this", results[1].NewName)
	assert.Equal(t, "DONE: also this", results[2].NewName)
}

func TestCollectReplacements_DepthLimited(t *testing.T) {
	items := []*workflowy.Item{
		{
			ID:   "1",
			Name: "level0 hello",
			Children: []*workflowy.Item{
				{
					ID:   "2",
					Name: "level1 hello",
					Children: []*workflowy.Item{
						{ID: "3", Name: "level2 hello"},
					},
				},
			},
		},
	}

	re := regexp.MustCompile("hello")
	opts := ReplaceOptions{
		Pattern:     re,
		Replacement: "hi",
		Depth:       1,
	}

	var results []ReplaceResult
	collectReplacements(items, opts, 0, &results)

	assert.Len(t, results, 2)
	assert.Equal(t, "1", results[0].ID)
	assert.Equal(t, "2", results[1].ID)
}

func TestCollectReplacements_UnlimitedDepth(t *testing.T) {
	items := []*workflowy.Item{
		{
			ID:   "1",
			Name: "level0 hello",
			Children: []*workflowy.Item{
				{
					ID:   "2",
					Name: "level1 hello",
					Children: []*workflowy.Item{
						{ID: "3", Name: "level2 hello"},
					},
				},
			},
		},
	}

	re := regexp.MustCompile("hello")
	opts := ReplaceOptions{
		Pattern:     re,
		Replacement: "hi",
		Depth:       -1,
	}

	var results []ReplaceResult
	collectReplacements(items, opts, 0, &results)

	assert.Len(t, results, 3)
}

func TestCollectReplacements_NoMatches(t *testing.T) {
	items := []*workflowy.Item{
		{ID: "1", Name: "hello world"},
		{ID: "2", Name: "hello there"},
	}

	re := regexp.MustCompile("goodbye")
	opts := ReplaceOptions{
		Pattern:     re,
		Replacement: "hi",
		Depth:       -1,
	}

	var results []ReplaceResult
	collectReplacements(items, opts, 0, &results)

	assert.Len(t, results, 0)
}

func TestCollectReplacements_SameNameAfterReplace(t *testing.T) {
	items := []*workflowy.Item{
		{ID: "1", Name: "hello"},
	}

	re := regexp.MustCompile("nomatch")
	opts := ReplaceOptions{
		Pattern:     re,
		Replacement: "hi",
		Depth:       -1,
	}

	var results []ReplaceResult
	collectReplacements(items, opts, 0, &results)

	assert.Len(t, results, 0)
}

func TestCollectReplacements_PartialMatch(t *testing.T) {
	items := []*workflowy.Item{
		{ID: "1", Name: "prefix-hello-suffix"},
	}

	re := regexp.MustCompile("hello")
	opts := ReplaceOptions{
		Pattern:     re,
		Replacement: "world",
		Depth:       -1,
	}

	var results []ReplaceResult
	collectReplacements(items, opts, 0, &results)

	assert.Len(t, results, 1)
	assert.Equal(t, "prefix-world-suffix", results[0].NewName)
}

func TestCollectReplacements_MultipleMatchesInSameName(t *testing.T) {
	items := []*workflowy.Item{
		{ID: "1", Name: "hello hello hello"},
	}

	re := regexp.MustCompile("hello")
	opts := ReplaceOptions{
		Pattern:     re,
		Replacement: "hi",
		Depth:       -1,
	}

	var results []ReplaceResult
	collectReplacements(items, opts, 0, &results)

	assert.Len(t, results, 1)
	assert.Equal(t, "hi hi hi", results[0].NewName)
}

func TestReplaceResult_String_Applied(t *testing.T) {
	result := ReplaceResult{
		ID:      "abc123",
		OldName: "hello world",
		NewName: "hi world",
		Applied: true,
	}

	str := result.String()
	assert.Equal(t, `abc123: "hello world" → "hi world"`, str)
}

func TestReplaceResult_String_DryRun(t *testing.T) {
	result := ReplaceResult{
		ID:      "abc123",
		OldName: "hello world",
		NewName: "hi world",
		Applied: false,
	}

	str := result.String()
	assert.Equal(t, `abc123: "hello world" → (dry-run) "hi world"`, str)
}

func TestReplaceResult_String_Skipped(t *testing.T) {
	result := ReplaceResult{
		ID:         "abc123",
		OldName:    "hello world",
		NewName:    "hi world",
		Skipped:    true,
		SkipReason: "user declined",
	}

	str := result.String()
	assert.Equal(t, `abc123: "hello world" (skipped: user declined)`, str)
}

func TestCollectReplacements_URL(t *testing.T) {
	items := []*workflowy.Item{
		{ID: "abc-123-def", Name: "hello"},
	}

	re := regexp.MustCompile("hello")
	opts := ReplaceOptions{
		Pattern:     re,
		Replacement: "hi",
		Depth:       -1,
	}

	var results []ReplaceResult
	collectReplacements(items, opts, 0, &results)

	assert.Len(t, results, 1)
	assert.Equal(t, "https://workflowy.com/#/abc-123-def", results[0].URL)
}
