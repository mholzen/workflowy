package markdown

import (
	"fmt"
	"strings"
	"github.com/mholzen/workflowy/pkg/collections"
)

type TreeProviderWithString collections.TreeProvider[fmt.Stringer]

type NestedListGenerator struct {
	Prefix string
}

func GenerateNestedList[T TreeProviderWithString](data T, indentLevel int, generator NestedListGenerator) string {
	indent := strings.Repeat("  ", indentLevel)

	res := indent + generator.Prefix + data.Node().String()
	for child := range data.Children() {
		res += "\n" + GenerateNestedList(child, indentLevel+1, generator)
	}
	return res
}

func GenerateNestedUL[T TreeProviderWithString](data T, indentLevel int) string {
	generator := NestedListGenerator{Prefix: "- "}
	return GenerateNestedList(data, indentLevel, generator)
}

func GenerateNestedOL[T TreeProviderWithString](data T, indentLevel int) string {
	generator := NestedListGenerator{Prefix: fmt.Sprintf("%d. ", indentLevel+1)}
	return GenerateNestedList(data, indentLevel, generator)
}
