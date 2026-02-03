package search

import (
	"testing"

	"github.com/mholzen/workflowy/pkg/workflowy"
)

func int64Ptr(v int64) *int64 { return &v }

func TestSearchItems_ExcludesCompleted(t *testing.T) {
	items := []*workflowy.Item{
		{
			ID: "a", Name: "match",
			Children: []*workflowy.Item{
				{
					ID: "b", Name: "match", CompletedAt: int64Ptr(100),
					Children: []*workflowy.Item{
						{ID: "c", Name: "match"},
					},
				},
				{ID: "d", Name: "match"},
			},
		},
	}

	results := SearchItems(items, "match", false, false, false)
	ids := make(map[string]bool)
	for _, r := range results {
		ids[r.ID] = true
	}

	if !ids["a"] || !ids["d"] {
		t.Errorf("expected a and d in results, got %v", ids)
	}
	if ids["b"] || ids["c"] {
		t.Errorf("expected b and c excluded, got %v", ids)
	}
}

func TestSearchItems_IncludeCompleted(t *testing.T) {
	items := []*workflowy.Item{
		{
			ID: "a", Name: "match",
			Children: []*workflowy.Item{
				{
					ID: "b", Name: "match", CompletedAt: int64Ptr(100),
					Children: []*workflowy.Item{
						{ID: "c", Name: "match"},
					},
				},
			},
		},
	}

	results := SearchItems(items, "match", false, false, true)
	if len(results) != 3 {
		t.Errorf("expected 3 results with includeCompleted, got %d", len(results))
	}
}
