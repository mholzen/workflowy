package workflowy

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testChildren() []*Item {
	completedAt := int64(123)
	return []*Item{
		{ID: "c", Name: "Charlie", Priority: 2, Data: map[string]interface{}{"layoutMode": "todo"}, CompletedAt: &completedAt},
		{ID: "a", Name: "Alpha", Priority: 1, Children: []*Item{{ID: "nested", Name: "Nested"}}},
		{ID: "b", Name: "Bravo", Priority: 1},
		{ID: "d", Name: "Delta", Priority: 4},
	}
}

func TestNewChildrenPageSortsAndPaginates(t *testing.T) {
	page, err := NewChildrenPage(testChildren(), ChildrenPageOptions{
		Limit:   2,
		Offset:  1,
		Compact: true,
	})
	require.NoError(t, err)

	items := page.Items.([]CompactChild)
	require.Len(t, items, 2)
	assert.Equal(t, "b", items[0].ID)
	assert.Equal(t, "c", items[1].ID)
	assert.Equal(t, 4, page.Total)
	assert.True(t, page.HasMore)
	require.NotNil(t, page.NextOffset)
	assert.Equal(t, 3, *page.NextOffset)
}

func TestNewChildrenPageLastAndEmptyPages(t *testing.T) {
	last, err := NewChildrenPage(testChildren(), ChildrenPageOptions{
		Limit:   2,
		Offset:  2,
		Compact: true,
	})
	require.NoError(t, err)
	assert.False(t, last.HasMore)
	assert.Nil(t, last.NextOffset)
	assert.Len(t, last.Items.([]CompactChild), 2)

	empty, err := NewChildrenPage(testChildren(), ChildrenPageOptions{
		Limit:   2,
		Offset:  10,
		Compact: true,
	})
	require.NoError(t, err)
	assert.False(t, empty.HasMore)
	assert.Empty(t, empty.Items.([]CompactChild))
}

func TestNewChildrenPageNameFilter(t *testing.T) {
	page, err := NewChildrenPage(testChildren(), ChildrenPageOptions{
		Limit:      10,
		Compact:    true,
		NameFilter: "^a",
		IgnoreCase: true,
	})
	require.NoError(t, err)

	items := page.Items.([]CompactChild)
	require.Len(t, items, 1)
	assert.Equal(t, "a", items[0].ID)
	assert.Equal(t, 1, page.Total)
}

func TestCompactChildrenIncludesSmallTriageFields(t *testing.T) {
	page, err := NewChildrenPage(testChildren(), ChildrenPageOptions{
		Limit:   10,
		Compact: true,
	})
	require.NoError(t, err)

	items := page.Items.([]CompactChild)
	require.Len(t, items, 4)
	assert.Equal(t, "a", items[0].ID)
	require.NotNil(t, items[0].HasChildren)
	assert.True(t, *items[0].HasChildren)
	assert.Equal(t, "todo", items[2].LayoutMode)
	assert.True(t, items[2].Completed)

	raw, err := json.Marshal(items[0])
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "priority")
	assert.NotContains(t, string(raw), "createdAt")
	assert.NotContains(t, string(raw), "url")
}

func TestFullChildrenOmitDescendants(t *testing.T) {
	page, err := NewChildrenPage(testChildren(), ChildrenPageOptions{
		Limit:   10,
		Compact: false,
	})
	require.NoError(t, err)

	items := page.Items.([]*Item)
	require.Len(t, items, 4)
	assert.Equal(t, "a", items[0].ID)
	assert.Nil(t, items[0].Children)
}
