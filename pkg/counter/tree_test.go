package counter

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_CopyTree(t *testing.T) {
	root := &testTreeNode{
		val: 1, children: []*testTreeNode{
			{val: 2},
			{val: 3},
			{val: 4},
		},
	}

	result := CopyTree(root)

	require.Equal(t, 1, result.Node.val)
	require.Equal(t, 3, len(result.children))
	require.Equal(t, 2, result.children[0].Node.val)
	require.Equal(t, 3, result.children[1].Node.val)
	require.Equal(t, 4, result.children[2].Node.val)
}

func Test_FilterTree(t *testing.T) {
	root := &testTreeNode{
		val: 1, children: []*testTreeNode{
			{val: 2},
			{val: 3},
			{val: 4},
		},
	}

	result := FilterTree(root, func(node *testTreeNode) bool {
		return node.val%2 != 0
	})
	require.NotNil(t, result)
	require.Equal(t, 1, result.Node.val)
	require.Equal(t, 1, len(result.children))
	require.Equal(t, 3, result.children[0].Node.val)
}
