package collections

import (
	"iter"
)

type TreeProvider[T any] interface {
	Node() T
	Children() iter.Seq[TreeProvider[T]]
}

type Tree[T any] struct {
	node     T
	children []TreeProvider[T]
}

func (t *Tree[T]) Children() iter.Seq[TreeProvider[T]] {
	return iter.Seq[TreeProvider[T]](func(yield func(TreeProvider[T]) bool) {
		for _, child := range t.children {
			var treeProvider TreeProvider[T] = child
			if !yield(treeProvider) {
				break
			}
		}
	})
}

func (t *Tree[T]) SetChildren(children []TreeProvider[T]) {
	t.children = children
}

func (t *Tree[T]) Node() T {
	return t.node
}

func NewTree[T any](node T, children []TreeProvider[T]) Tree[T] {
	return Tree[T]{node: node, children: children}
}

func CopyTree[T TreeProvider[T]](root T) Tree[T] {
	return copyTree(root)
}

func copyTree[T TreeProvider[T]](node T) Tree[T] {
	children := []TreeProvider[T]{}
	for child := range node.Children() {
		tree := copyTree(child.Node())
		children = append(children, &tree)
	}
	return Tree[T]{node: node, children: children}
}
