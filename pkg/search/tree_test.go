package search

import (
	"strings"
	"testing"

	"github.com/mholzen/workflowy/pkg/workflowy"
)

func TestSearchItemsTree_BasicNesting(t *testing.T) {
	items := []*workflowy.Item{
		{
			ID: "a", Name: "A", CreatedAt: 1, ModifiedAt: 10,
			Children: []*workflowy.Item{
				{
					ID: "b", Name: "B", CreatedAt: 2, ModifiedAt: 20,
					Children: []*workflowy.Item{
						{ID: "c", Name: "C match", CreatedAt: 3, ModifiedAt: 30},
						{
							ID: "d", Name: "D", CreatedAt: 4, ModifiedAt: 40,
							Children: []*workflowy.Item{
								{ID: "e", Name: "E match", CreatedAt: 5, ModifiedAt: 50},
							},
						},
					},
				},
				{
					ID: "f", Name: "F", CreatedAt: 6, ModifiedAt: 60,
					Children: []*workflowy.Item{
						{ID: "g", Name: "G match", CreatedAt: 7, ModifiedAt: 70},
					},
				},
			},
		},
	}

	tree := SearchItemsTree(items, "match", false, false)

	if len(tree) != 1 {
		t.Fatalf("expected 1 root, got %d", len(tree))
	}
	root := tree[0]
	if root.ID != "a" {
		t.Errorf("expected root ID 'a', got %q", root.ID)
	}
	if root.IsMatch {
		t.Error("root should not be a match")
	}
	if len(root.Children) != 2 {
		t.Fatalf("expected 2 children of A, got %d", len(root.Children))
	}

	b := root.Children[0]
	if b.ID != "b" {
		t.Errorf("expected child ID 'b', got %q", b.ID)
	}
	if len(b.Children) != 2 {
		t.Fatalf("expected 2 children of B, got %d", len(b.Children))
	}

	c := b.Children[0]
	if !c.IsMatch || c.ID != "c" {
		t.Errorf("expected match node 'c', got id=%q isMatch=%v", c.ID, c.IsMatch)
	}

	d := b.Children[1]
	if d.IsMatch || d.ID != "d" {
		t.Errorf("expected intermediate node 'd', got id=%q isMatch=%v", d.ID, d.IsMatch)
	}
	if len(d.Children) != 1 || d.Children[0].ID != "e" {
		t.Error("expected D to have child E")
	}

	f := root.Children[1]
	if f.ID != "f" || len(f.Children) != 1 || f.Children[0].ID != "g" {
		t.Error("expected F with child G")
	}
}

func TestSearchItemsTree_RootMatch(t *testing.T) {
	items := []*workflowy.Item{
		{ID: "a", Name: "match here"},
	}

	tree := SearchItemsTree(items, "match", false, false)
	if len(tree) != 1 {
		t.Fatalf("expected 1 root, got %d", len(tree))
	}
	if !tree[0].IsMatch {
		t.Error("root should be a match")
	}
}

func TestSearchItemsTree_NoMatches(t *testing.T) {
	items := []*workflowy.Item{
		{ID: "a", Name: "nothing"},
	}

	tree := SearchItemsTree(items, "xyz", false, false)
	if len(tree) != 0 {
		t.Errorf("expected 0 roots, got %d", len(tree))
	}
}

func TestTreeNode_String(t *testing.T) {
	node := &TreeNode{
		ID:   "a",
		Name: "A",
		URL:  "https://workflowy.com/#/a",
		Children: []*TreeNode{
			{
				ID:              "b",
				Name:            "B match",
				URL:             "https://workflowy.com/#/b",
				IsMatch:         true,
				HighlightedName: "B **match**",
			},
		},
	}

	s := node.String()
	if !strings.Contains(s, "- [A](https://workflowy.com/#/a)") {
		t.Errorf("unexpected output: %s", s)
	}
	if !strings.Contains(s, "  - [B **match**](https://workflowy.com/#/b)") {
		t.Errorf("unexpected output: %s", s)
	}
}

func TestSortTreeNodes(t *testing.T) {
	nodes := []*TreeNode{
		{ID: "b", Name: "B", ModifiedAt: 10},
		{ID: "a", Name: "A", ModifiedAt: 20},
	}

	SortTreeNodes(nodes, OrderBy{Field: "match", Ascending: true})
	if nodes[0].ID != "a" {
		t.Errorf("expected 'a' first after sort by name ascending, got %q", nodes[0].ID)
	}

	SortTreeNodes(nodes, OrderBy{Field: "modified", Ascending: false})
	if nodes[0].ID != "a" {
		t.Errorf("expected 'a' first after sort by modified desc, got %q", nodes[0].ID)
	}
}
