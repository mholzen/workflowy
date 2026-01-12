package transform

import (
	"slices"
	"testing"

	"github.com/mholzen/workflowy/pkg/workflowy"
)

func TestSplit(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		separator string
		skipEmpty bool
		want      []string
	}{
		{
			name:      "split by comma",
			text:      "apple,banana,cherry",
			separator: ",",
			skipEmpty: true,
			want:      []string{"apple", "banana", "cherry"},
		},
		{
			name:      "split by comma with spaces",
			text:      "apple, banana, cherry",
			separator: ", ",
			skipEmpty: true,
			want:      []string{"apple", "banana", "cherry"},
		},
		{
			name:      "split by newline",
			text:      "line1\nline2\nline3",
			separator: "\n",
			skipEmpty: true,
			want:      []string{"line1", "line2", "line3"},
		},
		{
			name:      "skip empty segments",
			text:      "apple,,banana,,,cherry",
			separator: ",",
			skipEmpty: true,
			want:      []string{"apple", "banana", "cherry"},
		},
		{
			name:      "include empty segments",
			text:      "apple,,banana",
			separator: ",",
			skipEmpty: false,
			want:      []string{"apple", "", "banana"},
		},
		{
			name:      "trim whitespace when skipping empty",
			text:      "  apple  ,  banana  ,  cherry  ",
			separator: ",",
			skipEmpty: true,
			want:      []string{"apple", "banana", "cherry"},
		},
		{
			name:      "single element no split",
			text:      "apple",
			separator: ",",
			skipEmpty: true,
			want:      []string{"apple"},
		},
		{
			name:      "empty string",
			text:      "",
			separator: ",",
			skipEmpty: true,
			want:      []string{},
		},
		{
			name:      "only separators skipped",
			text:      ",,,",
			separator: ",",
			skipEmpty: true,
			want:      []string{},
		},
		{
			name:      "multi-char separator",
			text:      "apple---banana---cherry",
			separator: "---",
			skipEmpty: true,
			want:      []string{"apple", "banana", "cherry"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Split(tt.text, tt.separator, tt.skipEmpty)
			if !slices.Equal(got, tt.want) {
				t.Errorf("Split() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCollectSplits(t *testing.T) {
	tests := []struct {
		name      string
		items     []*workflowy.Item
		separator string
		field     Field
		skipEmpty bool
		maxDepth  int
		wantCount int
		wantParts [][]string
	}{
		{
			name: "collect single item split",
			items: []*workflowy.Item{
				{ID: "abc123", Name: "apple,banana,cherry"},
			},
			separator: ",",
			field:     FieldName,
			skipEmpty: true,
			maxDepth:  -1,
			wantCount: 1,
			wantParts: [][]string{{"apple", "banana", "cherry"}},
		},
		{
			name: "skip item with no separator",
			items: []*workflowy.Item{
				{ID: "abc123", Name: "apple"},
			},
			separator: ",",
			field:     FieldName,
			skipEmpty: true,
			maxDepth:  -1,
			wantCount: 0,
			wantParts: nil,
		},
		{
			name: "collect from note field",
			items: []*workflowy.Item{
				{ID: "abc123", Name: "title", Note: strPtr("item1;item2;item3")},
			},
			separator: ";",
			field:     FieldNote,
			skipEmpty: true,
			maxDepth:  -1,
			wantCount: 1,
			wantParts: [][]string{{"item1", "item2", "item3"}},
		},
		{
			name: "collect from children",
			items: []*workflowy.Item{
				{
					ID:   "parent",
					Name: "parent node",
					Children: []*workflowy.Item{
						{ID: "child1", Name: "a,b,c"},
						{ID: "child2", Name: "x,y"},
					},
				},
			},
			separator: ",",
			field:     FieldName,
			skipEmpty: true,
			maxDepth:  -1,
			wantCount: 2,
			wantParts: [][]string{{"a", "b", "c"}, {"x", "y"}},
		},
		{
			name: "respect depth limit",
			items: []*workflowy.Item{
				{
					ID:   "parent",
					Name: "a,b",
					Children: []*workflowy.Item{
						{ID: "child", Name: "x,y,z"},
					},
				},
			},
			separator: ",",
			field:     FieldName,
			skipEmpty: true,
			maxDepth:  0,
			wantCount: 1,
			wantParts: [][]string{{"a", "b"}},
		},
		{
			name: "multiple items at root",
			items: []*workflowy.Item{
				{ID: "item1", Name: "one,two"},
				{ID: "item2", Name: "three,four,five"},
			},
			separator: ",",
			field:     FieldName,
			skipEmpty: true,
			maxDepth:  -1,
			wantCount: 2,
			wantParts: [][]string{{"one", "two"}, {"three", "four", "five"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var results []SplitResult
			CollectSplits(tt.items, tt.separator, tt.field, tt.skipEmpty, 0, tt.maxDepth, &results)

			if len(results) != tt.wantCount {
				t.Errorf("CollectSplits() returned %d results, want %d", len(results), tt.wantCount)
				return
			}

			for i, result := range results {
				if i >= len(tt.wantParts) {
					break
				}
				if !slices.Equal(result.Parts, tt.wantParts[i]) {
					t.Errorf("result[%d].Parts = %v, want %v", i, result.Parts, tt.wantParts[i])
				}
			}
		})
	}
}

func TestSplitResult_String(t *testing.T) {
	tests := []struct {
		name   string
		result SplitResult
		want   string
	}{
		{
			name: "applied result",
			result: SplitResult{
				ParentID: "abc123",
				Original: "a,b,c",
				Parts:    []string{"a", "b", "c"},
				Applied:  true,
			},
			want: `abc123: "a,b,c" → 3 children`,
		},
		{
			name: "dry run result",
			result: SplitResult{
				ParentID: "abc123",
				Original: "a,b,c",
				Parts:    []string{"a", "b", "c"},
				Applied:  false,
			},
			want: `abc123: "a,b,c" → (dry-run) 3 children`,
		},
		{
			name: "skipped result",
			result: SplitResult{
				ParentID:   "abc123",
				Original:   "a,b,c",
				Parts:      []string{"a", "b", "c"},
				Skipped:    true,
				SkipReason: "user declined",
			},
			want: `abc123: "a,b,c" (skipped: user declined)`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.result.String()
			if got != tt.want {
				t.Errorf("SplitResult.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func strPtr(s string) *string {
	return &s
}
