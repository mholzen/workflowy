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
	// Create root report node
	reportRoot := &workflowy.Item{
		Name:     c.Title(),
		Children: []*workflowy.Item{convertDescendantNode(c.Descendants)},
	}

	return reportRoot, nil
}

// convertDescendantNode recursively converts a descendant count node to a Workflowy item
func convertDescendantNode(node workflowy.Descendants) *workflowy.Item {
	nodeValue := node.NodeValue()

	// Format the name with statistics
	name := fmt.Sprintf("%s (%.1f%%, %d descendants)",
		(*nodeValue).Name(),
		node.RatioToRoot*100,
		node.Count,
	)

	// Create the item
	item := &workflowy.Item{
		Name:     name,
		Children: make([]*workflowy.Item, 0),
	}

	// Recursively convert children
	for child := range node.Children() {
		childNode := convertDescendantNode(child.Node())
		item.Children = append(item.Children, childNode)
	}

	return item
}
