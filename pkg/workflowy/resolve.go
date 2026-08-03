package workflowy

import (
	"fmt"
	"regexp"
	"strings"
)

var fullNodeIDPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func ResolveNodeIDFromTree(items []*Item, rawID string) (string, error) {
	if rawID == "" || rawID == "None" {
		return rawID, nil
	}

	nodeID := nodeIDFromURL(rawID)

	if IsFullNodeID(rawID) {
		if FindItemByID(items, nodeID) == nil {
			return "", fmt.Errorf("Cannot resolve Workflowy node %q from tree: node was not found", rawID)
		}
		return nodeID, nil
	}

	if !IsShortID(nodeID) {
		return "", fmt.Errorf("Cannot resolve Workflowy node %q from tree: expected a full UUID or 12-character short ID", rawID)
	}

	matches := findNodeIDsWithSuffix(items, nodeID)
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("Cannot resolve Workflowy node %q from tree: node was not found", rawID)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("Cannot resolve Workflowy node %q from tree: short ID matches %d nodes", rawID, len(matches))
	}
}

func IsFullNodeID(rawID string) bool {
	return fullNodeIDPattern.MatchString(nodeIDFromURL(rawID))
}

func nodeIDFromURL(rawID string) string {
	nodeID := strings.TrimPrefix(rawID, "https://workflowy.com/#/")
	return strings.TrimPrefix(nodeID, "https://beta.workflowy.com/#/")
}

func findNodeIDsWithSuffix(items []*Item, suffix string) []string {
	var matches []string
	for _, item := range items {
		if strings.HasSuffix(item.ID, suffix) {
			matches = append(matches, item.ID)
		}
		matches = append(matches, findNodeIDsWithSuffix(item.Children, suffix)...)
	}
	return matches
}
