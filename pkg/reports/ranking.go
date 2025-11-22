package reports

import (
	"fmt"
	"time"

	"github.com/mholzen/workflowy/pkg/workflowy"
)

// ChildrenCountReportOutput wraps children count ranking results
type ChildrenCountReportOutput struct {
	Ranked []workflowy.ChildrenCountRankable
	TopN   int
}

// Title returns the report title
func (r *ChildrenCountReportOutput) Title() string {
	if r.TopN > 0 {
		return fmt.Sprintf("Top %d Nodes by Children Count - %s", r.TopN, GenerateTimestamp())
	}
	return fmt.Sprintf("Nodes by Children Count - %s", GenerateTimestamp())
}

// ToNodes converts the ranking to WorkFlowy items
func (r *ChildrenCountReportOutput) ToNodes() (*workflowy.Item, error) {
	children := make([]*workflowy.Item, len(r.Ranked))

	for i, rankable := range r.Ranked {
		nodeValue := rankable.Node.Count.NodeValue()

		name := fmt.Sprintf("%d. %s (%d children)",
			i+1,
			(*nodeValue).Name(),
			rankable.Node.Count.ChildrenCount,
		)

		children[i] = &workflowy.Item{
			Name: name,
		}
	}

	return &workflowy.Item{
		Name:     r.Title(),
		Children: children,
	}, nil
}

// CreatedReportOutput wraps created date ranking results
type CreatedReportOutput struct {
	Ranked []workflowy.TimestampRankable
	TopN   int
}

// Title returns the report title
func (r *CreatedReportOutput) Title() string {
	if r.TopN > 0 {
		return fmt.Sprintf("Top %d Oldest Nodes by Creation Date - %s", r.TopN, GenerateTimestamp())
	}
	return fmt.Sprintf("Oldest Nodes by Creation Date - %s", GenerateTimestamp())
}

// ToNodes converts the ranking to WorkFlowy items
func (r *CreatedReportOutput) ToNodes() (*workflowy.Item, error) {
	children := make([]*workflowy.Item, len(r.Ranked))

	for i, rankable := range r.Ranked {
		nodeValue := rankable.Node.Count.NodeValue()
		timestamp := rankable.Node.CreatedAt

		var name string
		if timestamp == 0 {
			name = fmt.Sprintf("%d. (no date): %s", i+1, (*nodeValue).Name())
		} else {
			date := time.Unix(timestamp, 0).Format("2006-01-02 15:04:05")
			name = fmt.Sprintf("%d. %s: %s", i+1, date, (*nodeValue).Name())
		}

		children[i] = &workflowy.Item{
			Name: name,
		}
	}

	return &workflowy.Item{
		Name:     r.Title(),
		Children: children,
	}, nil
}

// ModifiedReportOutput wraps modified date ranking results
type ModifiedReportOutput struct {
	Ranked []workflowy.TimestampRankable
	TopN   int
}

// Title returns the report title
func (r *ModifiedReportOutput) Title() string {
	if r.TopN > 0 {
		return fmt.Sprintf("Top %d Oldest Nodes by Modification Date - %s", r.TopN, GenerateTimestamp())
	}
	return fmt.Sprintf("Oldest Nodes by Modification Date - %s", GenerateTimestamp())
}

// ToNodes converts the ranking to WorkFlowy items
func (r *ModifiedReportOutput) ToNodes() (*workflowy.Item, error) {
	children := make([]*workflowy.Item, len(r.Ranked))

	for i, rankable := range r.Ranked {
		nodeValue := rankable.Node.Count.NodeValue()
		var timestamp int64
		if rankable.UseModified {
			timestamp = rankable.Node.ModifiedAt
		} else {
			timestamp = rankable.Node.CreatedAt
		}

		var name string
		if timestamp == 0 {
			name = fmt.Sprintf("%d. (no date): %s", i+1, (*nodeValue).Name())
		} else {
			date := time.Unix(timestamp, 0).Format("2006-01-02 15:04:05")
			name = fmt.Sprintf("%d. %s: %s", i+1, date, (*nodeValue).Name())
		}

		children[i] = &workflowy.Item{
			Name: name,
		}
	}

	return &workflowy.Item{
		Name:     r.Title(),
		Children: children,
	}, nil
}
