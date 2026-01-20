package mirror

import (
	"sort"

	"github.com/mholzen/workflowy/pkg/workflowy"
)

// MirrorLocation represents a location where a node is mirrored
type MirrorLocation struct {
	ID         string
	ParentID   string
	ParentName string
}

// MirrorInfo represents mirror metadata for a node
type MirrorInfo struct {
	NodeID          string
	NodeName        string
	ParentID        string            // parent of this node
	ParentName      string            // parent's name
	OriginalID      string            // set on mirror copies (points to the original)
	MirrorLocations []MirrorLocation  // locations where this is mirrored (with parent info)
	Item            *workflowy.Item   // reference to the original item
}

// MirrorCount returns the number of locations where this node is mirrored
func (m *MirrorInfo) MirrorCount() int {
	return len(m.MirrorLocations)
}

// extractMirrorRootIDs extracts raw mirror root IDs from item metadata
func extractMirrorRootIDs(item *workflowy.Item) ([]string, string) {
	if item == nil || item.Data == nil {
		return nil, ""
	}

	mirrorData, ok := item.Data["mirror"]
	if !ok {
		return nil, ""
	}

	mirrorMap, ok := mirrorData.(map[string]any)
	if !ok {
		return nil, ""
	}

	var originalID string
	if oid, ok := mirrorMap["originalId"].(string); ok {
		originalID = oid
	}

	var ids []string
	if mirrorRootIDs, ok := mirrorMap["mirrorRootIds"].(map[string]any); ok {
		for id := range mirrorRootIDs {
			ids = append(ids, id)
		}
	}

	return ids, originalID
}

// parentInfo holds parent information for a node
type parentInfo struct {
	parentID   string
	parentName string
}

// CollectMirrorInfos walks a tree and collects nodes with mirrorRootIds (originals that have mirrors)
// It also resolves parent information for each mirror location
func CollectMirrorInfos(items []*workflowy.Item) []*MirrorInfo {
	// First pass: build a map of nodeID -> parent info
	parentMap := make(map[string]parentInfo)
	for _, item := range items {
		buildParentMap(item, "", "", parentMap)
	}

	// Second pass: collect mirror infos and resolve parent info
	var result []*MirrorInfo
	for _, item := range items {
		collectFromItem(item, &result, parentMap)
	}
	return result
}

func buildParentMap(item *workflowy.Item, parentID, parentName string, parentMap map[string]parentInfo) {
	if item == nil {
		return
	}

	parentMap[item.ID] = parentInfo{parentID: parentID, parentName: parentName}

	for _, child := range item.Children {
		buildParentMap(child, item.ID, item.Name, parentMap)
	}
}

func collectFromItem(item *workflowy.Item, result *[]*MirrorInfo, parentMap map[string]parentInfo) {
	if item == nil {
		return
	}

	mirrorIDs, originalID := extractMirrorRootIDs(item)
	if len(mirrorIDs) > 0 {
		info := &MirrorInfo{
			NodeID:     item.ID,
			NodeName:   item.Name,
			OriginalID: originalID,
			Item:       item,
		}

		// Set parent info for the original node
		if pInfo, ok := parentMap[item.ID]; ok {
			info.ParentID = pInfo.parentID
			info.ParentName = pInfo.parentName
		}

		for _, mirrorID := range mirrorIDs {
			loc := MirrorLocation{ID: mirrorID}
			if pInfo, ok := parentMap[mirrorID]; ok {
				loc.ParentID = pInfo.parentID
				loc.ParentName = pInfo.parentName
			}
			info.MirrorLocations = append(info.MirrorLocations, loc)
		}

		*result = append(*result, info)
	}

	for _, child := range item.Children {
		collectFromItem(child, result, parentMap)
	}
}

// RankByMirrorCount sorts mirror infos by count descending and returns top N
// If topN is 0, returns all
func RankByMirrorCount(infos []*MirrorInfo, topN int) []*MirrorInfo {
	sorted := make([]*MirrorInfo, len(infos))
	copy(sorted, infos)

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].MirrorCount() > sorted[j].MirrorCount()
	})

	if topN > 0 && topN < len(sorted) {
		return sorted[:topN]
	}
	return sorted
}
