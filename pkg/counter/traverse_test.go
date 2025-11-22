package counter

import (
	"iter"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testTreeNode struct {
	val      int
	children []*testTreeNode
}

func (n *testTreeNode) Node() *testTreeNode {
	return n
}

func (n *testTreeNode) Children() iter.Seq[TreeProvider[*testTreeNode]] {
	return iter.Seq[TreeProvider[*testTreeNode]](func(yield func(TreeProvider[*testTreeNode]) bool) {
		for _, child := range n.children {
			if !yield(child) {
				break
			}
		}
	})
}

func Test_testTreeNodeImplementsTreeProvider(t *testing.T) {
	var _ TreeProvider[*testTreeNode] = &testTreeNode{}
	assert.Implements(t, (*TreeProvider[*testTreeNode])(nil), &testTreeNode{})
}

func Test_TraversePost(t *testing.T) {
	root := &testTreeNode{val: 1, children: []*testTreeNode{
		{val: 2, children: []*testTreeNode{
			{val: 3},
			{val: 4},
		}},
	}}

	expected := []int{3, 4, 2, 1}
	actual := []int{}
	TraverseTreePost(root, func(node *testTreeNode, parent **testTreeNode, last bool) bool {
		actual = append(actual, node.val)
		return true
	})
	assert.Equal(t, expected, actual)
}
