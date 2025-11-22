package reports

import (
	"time"

	"github.com/mholzen/workflowy/pkg/workflowy"
)

// ReportOutput represents a report that can be converted to WorkFlowy nodes
type ReportOutput interface {
	// ToNodes converts the report to a tree of WorkFlowy items
	ToNodes() (*workflowy.Item, error)

	// Title returns the report title
	Title() string
}

// GenerateTimestamp returns a formatted timestamp for report titles
func GenerateTimestamp() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

// CreateReportNote creates a standard report note with generation time
func CreateReportNote() string {
	return "Generated: " + GenerateTimestamp()
}
