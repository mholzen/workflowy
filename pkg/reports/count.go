package reports

import (
	"fmt"

	"github.com/mholzen/workflowy/pkg/workflowy"
)

// CountReportOutput wraps descendant count results for conversion to nodes
type CountReportOutput struct {
	RootItem    *workflowy.Item
	Descendants workflowy.Descendants
	Threshold   float64
}

// Title returns the report title
func (c *CountReportOutput) Title() string {
	return fmt.Sprintf("Descendant Count Report (threshold: %.2f%%) - %s",
		c.Threshold*100, GenerateTimestamp())
}

// ToNodes converts the count report to a tree of Workflowy items
func (c *CountReportOutput) ToNodes() (*workflowy.Item, error) {
	reportRoot := &workflowy.Item{
		Name:     c.Title(),
		Children: []*workflowy.Item{convertDescendantNode(c.Descendants)},
	}

	return reportRoot, nil
}

func convertDescendantNode(node workflowy.Descendants) *workflowy.Item {
	nodeValue := node.NodeValue()

	name := fmt.Sprintf("%s (%.1f%%, %d descendants)",
		(*nodeValue).String(),
		node.RatioToRoot*100,
		node.Count,
	)

	item := &workflowy.Item{
		Name:     name,
		Children: make([]*workflowy.Item, 0),
	}

	for child := range node.Children() {
		childNode := convertDescendantNode(child.Node())
		item.Children = append(item.Children, childNode)
	}

	return item
}
