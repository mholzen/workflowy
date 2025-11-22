package collections

type Stack[T any] struct {
	Items []T
}

func (s Stack[T]) IsEmpty() bool {
	return len(s.Items) == 0
}

func (s *Stack[T]) Push(item T) {
	s.Items = append(s.Items, item)
}

func (s *Stack[T]) Pop() *T {
	top := &s.Items[len(s.Items)-1]
	s.Items = s.Items[:len(s.Items)-1]
	return top
}

func (s *Stack[T]) Top() *T {
	return &s.Items[len(s.Items)-1]
}
