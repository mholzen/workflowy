package search

import (
	"testing"

	"github.com/mholzen/workflowy/pkg/workflowy"
	"github.com/stretchr/testify/assert"
)

func TestParseOrderBy(t *testing.T) {
	tests := []struct {
		input     string
		field     string
		ascending bool
		wantErr   bool
	}{
		{"", "priority", true, false},
		{"priority", "priority", true, false},
		{"-priority", "priority", false, false},
		{"name", "name", true, false},
		{"match", "match", true, false},
		{"parent", "parent", true, false},
		{"path", "path", true, false},
		{"modified", "modified", false, false},
		{"created", "created", false, false},
		{"+modified", "modified", true, false},
		{"-match", "match", false, false},
		{"+created", "created", true, false},
		{"-path", "path", false, false},
		{"unknown", "", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			ob, err := ParseOrderBy(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ob.Field != tt.field {
				t.Errorf("field: got %q, want %q", ob.Field, tt.field)
			}
			if ob.Ascending != tt.ascending {
				t.Errorf("ascending: got %v, want %v", ob.Ascending, tt.ascending)
			}
		})
	}
}

func TestSortResults(t *testing.T) {
	results := []Result{
		{Name: "banana", ModifiedAt: 100, CreatedAt: 300},
		{Name: "apple", ModifiedAt: 300, CreatedAt: 100},
		{Name: "cherry", ModifiedAt: 200, CreatedAt: 200},
	}

	SortResults(results, OrderBy{Field: "match", Ascending: true})
	if results[0].Name != "apple" || results[1].Name != "banana" || results[2].Name != "cherry" {
		t.Errorf("match asc: got %s, %s, %s", results[0].Name, results[1].Name, results[2].Name)
	}

	SortResults(results, OrderBy{Field: "modified", Ascending: false})
	if results[0].ModifiedAt != 300 || results[1].ModifiedAt != 200 || results[2].ModifiedAt != 100 {
		t.Errorf("modified desc: got %d, %d, %d", results[0].ModifiedAt, results[1].ModifiedAt, results[2].ModifiedAt)
	}

	SortResults(results, OrderBy{Field: "created", Ascending: true})
	if results[0].CreatedAt != 100 || results[1].CreatedAt != 200 || results[2].CreatedAt != 300 {
		t.Errorf("created asc: got %d, %d, %d", results[0].CreatedAt, results[1].CreatedAt, results[2].CreatedAt)
	}
}

// Matches are gathered depth-first, so the order they arrive in is the outline
// order. Sorting them by their sibling index would interleave nodes from
// unrelated parents, so priority leaves a flat result set alone.
// order, in both directions: SortSearchRoots has already applied the direction
// to the outline by the time matches are collected. Reversing here instead
// would produce reverse depth-first order, placing a child ahead of its parent.
func TestSortResultsByPriorityLeavesCollectionOrderAlone(t *testing.T) {
	for _, ascending := range []bool{true, false} {
		results := []Result{{Name: "banana"}, {Name: "apple"}, {Name: "cherry"}}
		SortResults(results, OrderBy{Field: "priority", Ascending: ascending})
		if results[0].Name != "banana" || results[1].Name != "apple" || results[2].Name != "cherry" {
			t.Errorf("ascending=%v: got %s, %s, %s", ascending, results[0].Name, results[1].Name, results[2].Name)
		}
	}
}

func TestSortGroupedResultsByPriorityLeavesCollectionOrderAlone(t *testing.T) {
	for _, ascending := range []bool{true, false} {
		groups := []GroupedResult{{GroupLabel: "B"}, {GroupLabel: "A"}, {GroupLabel: "C"}}
		SortGroupedResults(groups, OrderBy{Field: "priority", Ascending: ascending})
		if groups[0].GroupLabel != "B" || groups[1].GroupLabel != "A" || groups[2].GroupLabel != "C" {
			t.Errorf("ascending=%v: got %s, %s, %s", ascending, groups[0].GroupLabel, groups[1].GroupLabel, groups[2].GroupLabel)
		}
	}
}

// SortSearchRoots is where the direction actually lands: on the outline, while
// each node is still next to its real siblings.
func TestSortSearchRootsOrdersSiblingsInPlace(t *testing.T) {
	roots := func() []*workflowy.Item {
		return []*workflowy.Item{
			{ID: "a", Priority: 0, Children: []*workflowy.Item{{ID: "a1", Priority: 0}, {ID: "a2", Priority: 1}}},
			{ID: "b", Priority: 1},
		}
	}
	ids := func(items []*workflowy.Item) []string {
		out := make([]string, len(items))
		for i, item := range items {
			out[i] = item.ID
		}
		return out
	}

	ascending := roots()
	SortSearchRoots(ascending, OrderBy{Field: "priority", Ascending: true})
	assert.Equal(t, []string{"a", "b"}, ids(ascending))
	assert.Equal(t, []string{"a1", "a2"}, ids(ascending[0].Children))

	descending := roots()
	SortSearchRoots(descending, OrderBy{Field: "priority", Ascending: false})
	assert.Equal(t, []string{"b", "a"}, ids(descending))
	assert.Equal(t, []string{"a2", "a1"}, ids(descending[1].Children))

	// A value sort ranks the results themselves, so the outline is left alone.
	untouched := roots()
	SortSearchRoots(untouched, OrderBy{Field: "name", Ascending: false})
	assert.Equal(t, []string{"a", "b"}, ids(untouched))
}

func TestSortGroupedResults(t *testing.T) {
	groups := []GroupedResult{
		{GroupLabel: "B", Children: []Result{{ModifiedAt: 100}}},
		{GroupLabel: "A", Children: []Result{{ModifiedAt: 300}}},
		{GroupLabel: "C", Children: []Result{{ModifiedAt: 200}}},
	}

	SortGroupedResults(groups, OrderBy{Field: "match", Ascending: true})
	if groups[0].GroupLabel != "A" || groups[1].GroupLabel != "B" || groups[2].GroupLabel != "C" {
		t.Errorf("label asc: got %s, %s, %s", groups[0].GroupLabel, groups[1].GroupLabel, groups[2].GroupLabel)
	}

	SortGroupedResults(groups, OrderBy{Field: "modified", Ascending: false})
	if groups[0].Children[0].ModifiedAt != 300 {
		t.Errorf("modified desc: expected first group to have max modified")
	}
}
