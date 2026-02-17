package workflowy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func makeTestTree() []*Item {
	return []*Item{
		{
			ID:   "a",
			Name: "A",
			Children: []*Item{
				{
					ID:   "b",
					Name: "B",
					Children: []*Item{
						{
							ID:       "c",
							Name:     "C",
							Children: []*Item{
								{ID: "d", Name: "D"},
							},
						},
					},
				},
				{ID: "e", Name: "E"},
			},
		},
		{ID: "f", Name: "F"},
	}
}

func TestFindItemWithAncestors_DeepNode(t *testing.T) {
	tree := makeTestTree()
	item, ancestors := FindItemWithAncestors(tree, "c")

	assert.NotNil(t, item)
	assert.Equal(t, "c", item.ID)
	assert.Len(t, ancestors, 2)
	assert.Equal(t, "a", ancestors[0].ID)
	assert.Equal(t, "b", ancestors[1].ID)
}

func TestFindItemWithAncestors_TopLevel(t *testing.T) {
	tree := makeTestTree()
	item, ancestors := FindItemWithAncestors(tree, "a")

	assert.NotNil(t, item)
	assert.Equal(t, "a", item.ID)
	assert.Empty(t, ancestors)
}

func TestFindItemWithAncestors_NotFound(t *testing.T) {
	tree := makeTestTree()
	item, ancestors := FindItemWithAncestors(tree, "nonexistent")

	assert.Nil(t, item)
	assert.Nil(t, ancestors)
}

func TestFindItemWithAncestors_LeafNode(t *testing.T) {
	tree := makeTestTree()
	item, ancestors := FindItemWithAncestors(tree, "d")

	assert.NotNil(t, item)
	assert.Equal(t, "d", item.ID)
	assert.Len(t, ancestors, 3)
	assert.Equal(t, "a", ancestors[0].ID)
	assert.Equal(t, "b", ancestors[1].ID)
	assert.Equal(t, "c", ancestors[2].ID)
}

func TestFindItemWithAncestors_SecondTopLevel(t *testing.T) {
	tree := makeTestTree()
	item, ancestors := FindItemWithAncestors(tree, "f")

	assert.NotNil(t, item)
	assert.Equal(t, "f", item.ID)
	assert.Empty(t, ancestors)
}

func TestBuildAncestorSpine_WithAncestors(t *testing.T) {
	tree := makeTestTree()
	item, ancestors := FindItemWithAncestors(tree, "c")

	spine := BuildAncestorSpine(item, ancestors, -1)

	// Spine root is a copy of "a"
	assert.Equal(t, "a", spine.ID)
	assert.Equal(t, "A", spine.Name)
	assert.Len(t, spine.Children, 1)

	// Next is copy of "b"
	b := spine.Children[0]
	assert.Equal(t, "b", b.ID)
	assert.Len(t, b.Children, 1)

	// Leaf is "c" with its original children
	c := b.Children[0]
	assert.Equal(t, "c", c.ID)
	assert.Len(t, c.Children, 1)
	assert.Equal(t, "d", c.Children[0].ID)
}

func TestBuildAncestorSpine_NoAncestors(t *testing.T) {
	tree := makeTestTree()
	item, ancestors := FindItemWithAncestors(tree, "a")

	spine := BuildAncestorSpine(item, ancestors, -1)

	// No ancestors, so the spine is the item itself
	assert.Equal(t, "a", spine.ID)
	assert.Len(t, spine.Children, 2) // original children preserved
}

func TestBuildAncestorSpine_DepthLimitsTargetChildren(t *testing.T) {
	tree := makeTestTree()
	item, ancestors := FindItemWithAncestors(tree, "c")

	spine := BuildAncestorSpine(item, ancestors, 0)

	// Walk to target
	c := spine.Children[0].Children[0]
	assert.Equal(t, "c", c.ID)
	// depth=0 means no children on target
	assert.Nil(t, c.Children)
}

func TestBuildAncestorSpine_AncestorsAreCopies(t *testing.T) {
	tree := makeTestTree()
	item, ancestors := FindItemWithAncestors(tree, "c")

	spine := BuildAncestorSpine(item, ancestors, -1)

	// Original "a" still has 2 children (b, e) -- spine copy has 1
	original := tree[0]
	assert.Len(t, original.Children, 2)
	assert.Len(t, spine.Children, 1)
}

func TestBuildAncestorSpine_PreservesMetadata(t *testing.T) {
	note := "test note"
	items := []*Item{
		{
			ID: "parent", Name: "Parent", Note: &note,
			CreatedAt: 1000, ModifiedAt: 2000,
			Children: []*Item{
				{ID: "child", Name: "Child"},
			},
		},
	}

	item, ancestors := FindItemWithAncestors(items, "child")
	spine := BuildAncestorSpine(item, ancestors, -1)

	assert.Equal(t, "parent", spine.ID)
	assert.Equal(t, "Parent", spine.Name)
	assert.Equal(t, &note, spine.Note)
	assert.Equal(t, int64(1000), spine.CreatedAt)
	assert.Equal(t, int64(2000), spine.ModifiedAt)
}

func TestTruncateAncestors_KeepLastN(t *testing.T) {
	ancestors := []*Item{
		{ID: "a", Name: "A"},
		{ID: "b", Name: "B"},
		{ID: "c", Name: "C"},
	}

	result := TruncateAncestors(ancestors, 2)
	assert.Len(t, result, 2)
	assert.Equal(t, "b", result[0].ID)
	assert.Equal(t, "c", result[1].ID)
}

func TestTruncateAncestors_DepthExceedsLength(t *testing.T) {
	ancestors := []*Item{
		{ID: "a", Name: "A"},
		{ID: "b", Name: "B"},
	}

	result := TruncateAncestors(ancestors, 5)
	assert.Len(t, result, 2)
}

func TestTruncateAncestors_DepthMinusOne(t *testing.T) {
	ancestors := []*Item{
		{ID: "a", Name: "A"},
		{ID: "b", Name: "B"},
		{ID: "c", Name: "C"},
	}

	result := TruncateAncestors(ancestors, -1)
	assert.Len(t, result, 3)
}

func TestTruncateAncestors_DepthZero(t *testing.T) {
	ancestors := []*Item{
		{ID: "a", Name: "A"},
	}

	result := TruncateAncestors(ancestors, 0)
	assert.Nil(t, result)
}

func TestTruncateAncestors_EmptyAncestors(t *testing.T) {
	result := TruncateAncestors(nil, 3)
	assert.Nil(t, result)
}

func TestTruncateAncestors_DepthOne(t *testing.T) {
	ancestors := []*Item{
		{ID: "a", Name: "A"},
		{ID: "b", Name: "B"},
		{ID: "c", Name: "C"},
	}

	result := TruncateAncestors(ancestors, 1)
	assert.Len(t, result, 1)
	assert.Equal(t, "c", result[0].ID)
}
