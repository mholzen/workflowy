package reports

import (
	"fmt"

	"github.com/mholzen/workflowy/pkg/mirror"
	"github.com/mholzen/workflowy/pkg/workflowy"
)

// MirrorCountReportOutput wraps mirror count ranking results
type MirrorCountReportOutput struct {
	Ranked []*mirror.MirrorInfo
	TopN   int
}

// Title returns the report title
func (r *MirrorCountReportOutput) Title() string {
	if r.TopN > 0 {
		return fmt.Sprintf("Top %d Nodes by Mirror Count - %s", r.TopN, GenerateTimestamp())
	}
	return fmt.Sprintf("Nodes by Mirror Count - %s", GenerateTimestamp())
}

// ToNodes converts the ranking to Workflowy items
func (r *MirrorCountReportOutput) ToNodes() (*workflowy.Item, error) {
	children := make([]*workflowy.Item, len(r.Ranked))

	for i, info := range r.Ranked {
		name := fmt.Sprintf("%d. %s (%d mirrors)",
			i+1,
			info.NodeName,
			info.MirrorCount(),
		)

		child := &workflowy.Item{
			Name:     name,
			Children: buildMirrorChildren(info),
		}

		children[i] = child
	}

	return &workflowy.Item{
		Name:     r.Title(),
		Children: children,
	}, nil
}

func buildMirrorChildren(info *mirror.MirrorInfo) []*workflowy.Item {
	var children []*workflowy.Item

	if info.NodeID != "" {
		var originalLink string
		if info.ParentName != "" {
			originalLink = fmt.Sprintf("original: [%s](https://workflowy.com/#/%s) in [%s](https://workflowy.com/#/%s)",
				info.NodeName, info.NodeID, info.ParentName, info.ParentID)
		} else {
			originalLink = fmt.Sprintf("original: [%s](https://workflowy.com/#/%s)",
				info.NodeName, info.NodeID)
		}
		children = append(children, &workflowy.Item{Name: originalLink})
	}

	for _, loc := range info.MirrorLocations {
		shortID := loc.ID
		if len(loc.ID) > 12 {
			shortID = loc.ID[len(loc.ID)-12:]
		}

		var mirrorLink string
		if loc.ParentName != "" {
			mirrorLink = fmt.Sprintf("[%s](https://workflowy.com/#/%s) in [%s](https://workflowy.com/#/%s)",
				shortID, loc.ID, loc.ParentName, loc.ParentID)
		} else {
			mirrorLink = fmt.Sprintf("[%s](https://workflowy.com/#/%s)", shortID, loc.ID)
		}
		children = append(children, &workflowy.Item{Name: mirrorLink})
	}

	return children
}
