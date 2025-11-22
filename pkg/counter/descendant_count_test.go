package counter

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_IsTreeProvider(t *testing.T) {
	x := NewDescendantTreeCount(1)
	var _ TreeProvider[*DescendantTreeCount[int]] = x
}

func Test_DescendantTreeCount(t *testing.T) {
	root := &testTreeNode{
		val: 1, children: []*testTreeNode{
			{
				val: 2, children: []*testTreeNode{
					{val: 3},
					{val: 4, children: []*testTreeNode{
						{val: 5},
						{val: 6},
					}},
					{val: 7},
				}},
		}}

	result := CountDescendantTree(root)

	require.Equal(t, 7, result.Count)
	require.Equal(t, 1, len(result.children))
}

func Test_DescendantTreeCount2(t *testing.T) {
	root := &testTreeNode{
		val: 1, children: []*testTreeNode{
			{val: 2},
			{val: 3},
			{val: 4, children: []*testTreeNode{
				{val: 5},
			}},
		},
	}

	result := CountDescendantTree(root)
	require.Equal(t, 5, result.Count)
	require.Equal(t, 3, len(result.children))

	CalculateRatioToRoot(result)
	result = FilterDescendantTree(result, 0.3)

	require.Equal(t, 5, result.Count)
	require.Equal(t, 1, len(result.children))
}

func Test_SortDescendantTree(t *testing.T) {
	root := &testTreeNode{
		val: 1, children: []*testTreeNode{
			{val: 2, children: []*testTreeNode{
				{val: 3},
				{val: 4},
			}},
			{val: 5, children: []*testTreeNode{
				{val: 6},
				{val: 7},
				{val: 8},
			}},
			{val: 9},
		},
	}

	counted := CountDescendantTree(root)
	sorted := SortDescendantTree(counted)

	require.Equal(t, 9, sorted.Count)
	require.Equal(t, 3, len(sorted.children))

	require.Equal(t, 4, sorted.children[0].Count)
	require.Equal(t, 3, sorted.children[1].Count)
	require.Equal(t, 1, sorted.children[2].Count)

	require.True(t, sorted.children[0].Count >= sorted.children[1].Count)
	require.True(t, sorted.children[1].Count >= sorted.children[2].Count)
}
