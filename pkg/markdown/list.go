package markdown

import (
	"fmt"
)

type ListGenerator struct {
	prefix PrefixGenerator
}

type PrefixGenerator func(int) string

func GenerateList[T fmt.Stringer](items []T, generator ListGenerator) string {
	if len(items) == 0 {
		return ""
	}
	res := generator.prefix(0) + items[0].String()
	for i, item := range items[1:] {
		res += "\n" + generator.prefix(i+1) + item.String()
	}
	return res
}

func NewListGenerator(prefix PrefixGenerator) ListGenerator {
	return ListGenerator{prefix: prefix}
}

func GenerateUL[T TreeProviderWithString](data T, indentLevel int) string {
	generator := NestedListGenerator{Prefix: "- "}
	return GenerateNestedList(data, indentLevel, generator)
}

func GenerateOL[T fmt.Stringer](items []T) string {
	generator := NewListGenerator(func(i int) string { return fmt.Sprintf("%d. ", i+1) })
	return GenerateList(items, generator)
}
