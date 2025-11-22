package counter

// OBSOLETE: use collections.Tree instead

import (
	"iter"
	"github.com/mholzen/workflowy/pkg/collections"
)

type TreeProvider[T any] = collections.TreeProvider[T]

type Tree[T any] struct {
	Node     T
	children []Tree[T]
}

func (t *Tree[T]) Children() iter.Seq[Tree[T]] {
	return iter.Seq[Tree[T]](func(yield func(Tree[T]) bool) {
		for _, child := range t.children {
			if !yield(child) {
				break
			}
		}
	})
}

func CopyTree[T TreeProvider[T]](root T) Tree[T] {
	return copyTree(root)
}

func copyTree[T TreeProvider[T]](node T) Tree[T] {
	children := []Tree[T]{}
	for child := range node.Children() {
		children = append(children, copyTree(child.Node()))
	}
	return Tree[T]{Node: node, children: children}
}

func FilterTree[T TreeProvider[T]](root T, filter func(T) bool) *Tree[T] {
	return filterTree(root, filter)
}

func filterTree[T TreeProvider[T]](node T, filter func(T) bool) *Tree[T] {
	if !filter(node) {
		return nil
	}
	children := []Tree[T]{}
	for child := range node.Children() {
		filteredChild := filterTree(child.Node(), filter)
		if filteredChild != nil {
			children = append(children, *filteredChild)
		}
	}
	return &Tree[T]{Node: node, children: children}
}
