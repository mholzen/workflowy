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

func TestNewPageReturnsEmptySlicePastEnd(t *testing.T) {
	page, err := NewPage(testPageItems(), DefaultPageLimit, 10)
	require.NoError(t, err)

	assert.Equal(t, DefaultPageLimit, page.Limit)
	assert.False(t, page.HasMore)
	assert.Nil(t, page.NextOffset)
	assert.Empty(t, page.Items.([]*Item))

	first, last := page.Window()
	assert.Zero(t, first)
	assert.Zero(t, last)
}

func TestNewPageValidatesBounds(t *testing.T) {
	// Zero is a request for an empty page, not a request for the default, so it
	// is rejected rather than silently widened to DefaultPageLimit.
	_, err := NewPage(testPageItems(), 0, 0)
	assert.EqualError(t, err, "limit must be at least 1")

	_, err = NewPage(testPageItems(), -1, 0)
	assert.EqualError(t, err, "limit must be at least 1")

	_, err = NewPage(testPageItems(), MaxPageLimit+1, 0)
	assert.EqualError(t, err, "limit must be at most 200")

	_, err = NewPage(testPageItems(), 10, -1)
	assert.EqualError(t, err, "offset must be non-negative")
}

func TestPageWindowReportsOneBasedRange(t *testing.T) {
	page, err := NewPage(testPageItems(), 2, 1)
	require.NoError(t, err)

	first, last := page.Window()
	assert.Equal(t, 2, first)
	assert.Equal(t, 3, last)
}

func TestNodeRefForDescribesOnlyASpecificNode(t *testing.T) {
	note := "a note"
	ref := NodeRefFor(&Item{ID: "parent", Name: "Parent", Note: &note, Children: testPageItems()})
	require.NotNil(t, ref)
	assert.Equal(t, NodeRef{ID: "parent", Name: "Parent", Note: "a note"}, *ref)

	// Root responses are a list of top-level nodes, so there is no node to name.
	assert.Nil(t, NodeRefFor(&ListChildrenResponse{Items: testPageItems()}))
	assert.Nil(t, NodeRefFor((*Item)(nil)))
}

func TestIsStructuralSortOnlyCoversPriority(t *testing.T) {
	for _, field := range []string{"priority"} {
		order, err := ParseSortOrder(field)
		require.NoError(t, err)
		assert.True(t, IsStructuralSort(order), field)
	}
	for _, field := range []string{"name", "created", "modified"} {
		order, err := ParseSortOrder(field)
		require.NoError(t, err)
		assert.False(t, IsStructuralSort(order), field)
	}
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

// Backup files record position by array order and leave priority at zero, so
// every sibling ties. A stable descending sort by value would leave the list
// exactly as it was, making -priority silently do nothing on that path.
func TestSortItemsReversesSiblingsForDescendingPriorityWhenAllTie(t *testing.T) {
	items := []*Item{{ID: "z"}, {ID: "a"}, {ID: "m"}}
	order, err := ParseSortOrder("-priority")
	require.NoError(t, err)

	SortItems(items, order, false)

	assert.Equal(t, []string{"m", "a", "z"}, itemIDs(items))
}

func TestSortItemsReversesSiblingsForDescendingPriorityWithRealValues(t *testing.T) {
	items := []*Item{
		{ID: "c", Priority: 2},
		{ID: "a", Priority: 0},
		{ID: "b", Priority: 1},
	}
	order, err := ParseSortOrder("-priority")
	require.NoError(t, err)

	SortItems(items, order, false)

	assert.Equal(t, []string{"c", "b", "a"}, itemIDs(items))
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

// The tree deliberately arrives with siblings out of priority order and with
// names that do not follow the outline, so an ordering mistake cannot pass by
// coincidence.
func unsortedTestTree() *Item {
	return &Item{
		ID: "root",
		Children: []*Item{
			{ID: "b", Name: "Charlie", Priority: 1, Children: []*Item{
				{ID: "b1", Name: "Delta", Priority: 0},
			}},
			{ID: "a", Name: "Alpha", Priority: 0, Children: []*Item{
				{ID: "a2", Name: "Bravo", Priority: 1},
				{ID: "a1", Name: "Zulu", Priority: 0},
			}},
		},
	}
}

func TestSortedFlatListByPriorityKeepsOutlineOrder(t *testing.T) {
	order, err := ParseSortOrder("priority")
	require.NoError(t, err)

	flat := SortedFlatList(unsortedTestTree(), order, true)

	// Each parent is immediately followed by its own children. Sorting the
	// flattened list by priority instead would group every first child together,
	// yielding b1, a, a1, b, a2.
	assert.Equal(t, []string{"root", "a", "a1", "a2", "b", "b1"}, itemIDs(flat.Items))
}

func TestSortedFlatListByValueRanksTheWholeList(t *testing.T) {
	order, err := ParseSortOrder("name")
	require.NoError(t, err)

	flat := SortedFlatList(unsortedTestTree(), order, true)

	// Alpha, Bravo, Charlie, Delta, Zulu, with the unnamed root sorting first.
	assert.Equal(t, []string{"root", "a", "a2", "b", "b1", "a1"}, itemIDs(flat.Items))
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
