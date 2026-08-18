package workflowy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testPageItems() []*Item {
	return []*Item{
		{ID: "c", Name: "Charlie", Priority: 2, CreatedAt: 30, ModifiedAt: 10},
		{ID: "a", Name: "Alpha", Priority: 1, CreatedAt: 10, ModifiedAt: 30},
		{ID: "b", Name: "Bravo", Priority: 1, CreatedAt: 20, ModifiedAt: 20},
		{ID: "d", Name: "Delta", Priority: 4, CreatedAt: 40, ModifiedAt: 40},
	}
}

func TestNewPageReturnsRequestedWindow(t *testing.T) {
	page, err := NewPage(testPageItems(), 2, 1)
	require.NoError(t, err)

	items := page.Items.([]*Item)
	require.Len(t, items, 2)
	assert.Equal(t, "a", items[0].ID)
	assert.Equal(t, "b", items[1].ID)
	assert.Equal(t, 4, page.Total)
	assert.True(t, page.HasMore)
	require.NotNil(t, page.NextOffset)
	assert.Equal(t, 3, *page.NextOffset)
}

func TestNewPageUsesDefaultLimitAndReturnsEmptySlicePastEnd(t *testing.T) {
	page, err := NewPage(testPageItems(), 0, 10)
	require.NoError(t, err)

	assert.Equal(t, DefaultPageLimit, page.Limit)
	assert.False(t, page.HasMore)
	assert.Nil(t, page.NextOffset)
	assert.Empty(t, page.Items.([]*Item))
}

func TestNewPageValidatesBounds(t *testing.T) {
	_, err := NewPage(testPageItems(), -1, 0)
	assert.EqualError(t, err, "limit must be non-negative")

	_, err = NewPage(testPageItems(), MaxPageLimit+1, 0)
	assert.EqualError(t, err, "limit must be at most 200")

	_, err = NewPage(testPageItems(), 10, -1)
	assert.EqualError(t, err, "offset must be non-negative")
}

func TestParseSortOrderDefaultsToPriority(t *testing.T) {
	order, err := ParseSortOrder("")
	require.NoError(t, err)
	assert.Equal(t, SortOrder{Field: "priority", Ascending: true}, order)
}

func TestSortItemsSupportsFieldsAndDirection(t *testing.T) {
	items := testPageItems()
	order, err := ParseSortOrder("priority")
	require.NoError(t, err)
	SortItems(items, order, false)
	assert.Equal(t, []string{"a", "b", "c", "d"}, itemIDs(items))

	order, err = ParseSortOrder("-name")
	require.NoError(t, err)
	SortItems(items, order, false)
	assert.Equal(t, []string{"d", "c", "b", "a"}, itemIDs(items))

	order, err = ParseSortOrder("modified")
	require.NoError(t, err)
	SortItems(items, order, false)
	assert.Equal(t, []string{"d", "a", "b", "c"}, itemIDs(items))
}

func TestSortItemsPreservesOutlineOrderWhenPrioritiesTie(t *testing.T) {
	items := []*Item{
		{ID: "z", Priority: 0},
		{ID: "a", Priority: 0},
		{ID: "m", Priority: 0},
	}
	order, err := ParseSortOrder("priority")
	require.NoError(t, err)
	SortItems(items, order, false)
	assert.Equal(t, []string{"z", "a", "m"}, itemIDs(items))
}

func TestTopLevelItemsUsesChildrenForSpecificNode(t *testing.T) {
	children := testPageItems()
	assert.Equal(t, children, TopLevelItems(&Item{ID: "parent", Children: children}))
	assert.Equal(t, children, TopLevelItems(&ListChildrenResponse{Items: children}))
}

func itemIDs(items []*Item) []string {
	ids := make([]string, len(items))
	for i, item := range items {
		ids[i] = item.ID
	}
	return ids
}
