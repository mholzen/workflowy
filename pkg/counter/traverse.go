package counter

func TraverseTreePost[T TreeProvider[T]](node T, yield func(T, *T, bool) bool) {
	traverseTreePost(node, nil, true, yield)
}

func traverseTreePost[T TreeProvider[T]](node T, parent *T, last bool, yield func(T, *T, bool) bool) {
	children := []TreeProvider[T]{}
	for child := range node.Children() {
		children = append(children, child)
	}

	for i, child := range children {
		isLast := (i == len(children)-1)
		traverseTreePost(child.Node(), &node, isLast, yield)
	}
	yield(node, parent, last)
}
