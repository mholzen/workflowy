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
