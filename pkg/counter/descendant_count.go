package counter

import (
	"encoding/json"
	"fmt"
	"iter"
	"sort"
	"github.com/mholzen/workflowy/pkg/collections"
)

type DescendantTreeCount[T any] struct {
	node                T
	Count               int
	ChildrenCount       int
	RatioToParent       float64
	RatioToRoot         float64
	BelowThresholdCount int
	children            []*DescendantTreeCount[T]
}

func NewDescendantTreeCount[T any](node T) *DescendantTreeCount[T] {
	return &DescendantTreeCount[T]{
		node:     node,
		Count:    1,
		children: make([]*DescendantTreeCount[T], 0),
	}
}

func (d *DescendantTreeCount[T]) Node() *DescendantTreeCount[T] {
	return d
}

func (d *DescendantTreeCount[T]) NodeValue() T {
	return d.node
}

func (d *DescendantTreeCount[T]) SetNode(node T) {
	d.node = node
}

func (d *DescendantTreeCount[T]) Children() iter.Seq[TreeProvider[*DescendantTreeCount[T]]] {
	return iter.Seq[TreeProvider[*DescendantTreeCount[T]]](func(yield func(TreeProvider[*DescendantTreeCount[T]]) bool) {
		for _, child := range d.children {
			if !yield(child) {
				break
			}
		}
	})
}

func (d *DescendantTreeCount[T]) Add(node *DescendantTreeCount[T]) {
	d.children = append(d.children, node)
	d.Count += node.Count
}

func (d *DescendantTreeCount[T]) SetChildren(children []*DescendantTreeCount[T]) {
	d.children = children
	d.ChildrenCount = len(children)
	for _, child := range children {
		d.Count += child.Count
	}
}

func (d *DescendantTreeCount[T]) String() string {
	return fmt.Sprintf("count: %d, children: %d, node: %v (root: %f, parent: %f)", d.Count, len(d.children), d.node, d.RatioToRoot, d.RatioToParent)
}

func (d *DescendantTreeCount[T]) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Node                T                         `json:"node"`
		Count               int                       `json:"count"`
		ChildrenCount       int                       `json:"childrenCount"`
		RatioToParent       float64                   `json:"ratioToParent"`
		RatioToRoot         float64                   `json:"ratioToRoot"`
		BelowThresholdCount int                       `json:"belowThresholdCount"`
		Children            []*DescendantTreeCount[T] `json:"children"`
	}{
		Node:                d.node,
		Count:               d.Count,
		ChildrenCount:       d.ChildrenCount,
		RatioToParent:       d.RatioToParent,
		RatioToRoot:         d.RatioToRoot,
		BelowThresholdCount: d.BelowThresholdCount,
		Children:            d.children,
	})
}

type DescendantTracker[T any] struct {
	Parent   *T
	Children []*DescendantTreeCount[*T]
}

func CountDescendantTree[T TreeProvider[T]](root T) *DescendantTreeCount[*T] {
	frames := collections.Stack[DescendantTracker[T]]{}
	pop := false

	TraverseTreePost(root, func(node T, parent *T, last bool) bool {
		newNode := NewDescendantTreeCount(&node)

		if pop {
			pop = false
			frame := frames.Pop()

			// all children have been visited
			newNode.SetChildren(frame.Children)
		}

		isNewParent := frames.IsEmpty() || frames.Top().Parent != parent
		if isNewParent {
			frames.Push(DescendantTracker[T]{Parent: parent})
		}

		frame := frames.Top()
		frame.Children = append(frame.Children, newNode)

		if last {
			// because it is post order, the next item traversed will be the parent
			// (which is where we want to create the new parent node)
			// so we act on iteration
			// if there are none, we find the result top node's first child
			pop = true
		}
		return true
	})

	return frames.Top().Children[0]
}

type TreeTraversePostTracker[F any, T any] struct {
	Parent   *F
	Children []*T
}

func FilterDescendantTree[T any](descendantTreeCount *DescendantTreeCount[*T], threshold float64) *DescendantTreeCount[*T] {
	eliminated := []*DescendantTreeCount[*T]{}
	frames := collections.Stack[TreeTraversePostTracker[*DescendantTreeCount[*T], DescendantTreeCount[*T]]]{}
	pop := false

	TraverseTreePost(descendantTreeCount, func(node *DescendantTreeCount[*T], parent **DescendantTreeCount[*T], last bool) bool {

		var isEliminated bool
		var ratioToParent float64
		if parent == nil {
			isEliminated = false
			ratioToParent = 1.0
		} else {
			ratioToParent = float64(node.Count) / float64((*parent).Count)
			isEliminated = node.RatioToRoot < threshold
		}

		newNode := NewDescendantTreeCount(node.NodeValue())
		newNode.RatioToParent = ratioToParent
		newNode.RatioToRoot = node.RatioToRoot

		if pop {
			pop = false
			frame := frames.Pop()

			var belowThresholdCount int
			for _, child := range frame.Children {
				belowThresholdCount += child.BelowThresholdCount
			}
			belowThresholdCount += len(eliminated)
			newNode.BelowThresholdCount = belowThresholdCount

			newNode.SetChildren(frame.Children)
			eliminated = eliminated[:0]
		}
		newNode.Count = node.Count
		newNode.ChildrenCount = node.ChildrenCount

		isNewParent := frames.IsEmpty() || frames.Top().Parent != parent
		if isNewParent {
			frames.Push(TreeTraversePostTracker[*DescendantTreeCount[*T], DescendantTreeCount[*T]]{Parent: parent})
		}
		frame := frames.Top()

		if isEliminated {
			eliminated = append(eliminated, node)
		} else {
			frame.Children = append(frame.Children, newNode)
		}

		if last {
			pop = true
		}

		return true
	})
	res := frames.Top().Children[0]
	return res
}

func CalculateRatioToRoot[T any](descendantTreeCount *DescendantTreeCount[*T]) {
	rootCount := descendantTreeCount.Count
	calculateRatioToRootRecursive(descendantTreeCount, rootCount)
}

func calculateRatioToRootRecursive[T any](node *DescendantTreeCount[*T], rootCount int) {
	node.RatioToRoot = float64(node.Count) / float64(rootCount)
	for _, child := range node.children {
		calculateRatioToRootRecursive(child, rootCount)
	}
}

func SortDescendantTree[T any](descendantTreeCount *DescendantTreeCount[*T]) *DescendantTreeCount[*T] {
	frames := collections.Stack[TreeTraversePostTracker[*DescendantTreeCount[*T], DescendantTreeCount[*T]]]{}
	pop := false

	TraverseTreePost(descendantTreeCount, func(node *DescendantTreeCount[*T], parent **DescendantTreeCount[*T], last bool) bool {
		newNode := NewDescendantTreeCount(node.NodeValue())
		newNode.Count = node.Count
		newNode.RatioToParent = node.RatioToParent
		newNode.RatioToRoot = node.RatioToRoot
		newNode.BelowThresholdCount = node.BelowThresholdCount

		if pop {
			pop = false
			frame := frames.Pop()

			sort.Slice(frame.Children, func(i, j int) bool {
				return frame.Children[i].Count > frame.Children[j].Count
			})

			newNode.SetChildren(frame.Children)
		}
		newNode.Count = node.Count
		newNode.ChildrenCount = node.ChildrenCount

		isNewParent := frames.IsEmpty() || frames.Top().Parent != parent
		if isNewParent {
			frames.Push(TreeTraversePostTracker[*DescendantTreeCount[*T], DescendantTreeCount[*T]]{Parent: parent})
		}
		frame := frames.Top()
		frame.Children = append(frame.Children, newNode)

		if last {
			pop = true
		}

		return true
	})

	return frames.Top().Children[0]
}

func CollectAllNodes[T any](descendantTreeCount *DescendantTreeCount[*T]) []*DescendantTreeCount[*T] {
	result := []*DescendantTreeCount[*T]{}

	TraverseTreePost(descendantTreeCount, func(node *DescendantTreeCount[*T], parent **DescendantTreeCount[*T], last bool) bool {
		result = append(result, node)
		return true
	})

	return result
}
