package search

import (
	"testing"
)

func TestParseOrderBy(t *testing.T) {
	tests := []struct {
		input     string
		field     string
		ascending bool
		wantErr   bool
	}{
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
