package search

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mholzen/workflowy/pkg/workflowy"
)

type TreeNode struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	URL      string      `json:"url"`
	IsMatch  bool        `json:"is_match,omitempty"`
	Children []*TreeNode `json:"children,omitempty"`

	HighlightedName string          `json:"highlighted_name,omitempty"`
	MatchPositions  []MatchPosition `json:"match_positions,omitempty"`

	CreatedAt  int64 `json:"-"`
	ModifiedAt int64 `json:"-"`
}

func (n *TreeNode) String() string {
	return n.renderAtDepth(0)
}

func (n *TreeNode) renderAtDepth(depth int) string {
	indent := strings.Repeat("  ", depth)
	displayName := n.Name
	if n.HighlightedName != "" {
		displayName = n.HighlightedName
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s- [%s](%s)", indent, displayName, n.URL)
	for _, child := range n.Children {
		fmt.Fprintf(&b, "\n%s", child.renderAtDepth(depth+1))
	}
	return b.String()
}

func SearchItemsTree(items []*workflowy.Item, pattern string, useRegexp, ignoreCase bool) []*TreeNode {
	nodeMap := make(map[string]*TreeNode)
	var roots []*TreeNode
	rootSet := make(map[string]bool)

	for _, item := range items {
		collectTreeResults(item, nil, pattern, useRegexp, ignoreCase, nodeMap, &roots, rootSet)
	}

	return roots
}

func collectTreeResults(item *workflowy.Item, ancestors []*workflowy.Item, pattern string, useRegexp, ignoreCase bool, nodeMap map[string]*TreeNode, roots *[]*TreeNode, rootSet map[string]bool) {
	name := stripHTMLTags(item.Name)
	matchPositions := FindMatches(name, pattern, useRegexp, ignoreCase)

	if len(matchPositions) > 0 {
		insertMatchIntoTree(item, ancestors, matchPositions, nodeMap, roots, rootSet)
	}

	childAncestors := append(append([]*workflowy.Item{}, ancestors...), item)
	for _, child := range item.Children {
		collectTreeResults(child, childAncestors, pattern, useRegexp, ignoreCase, nodeMap, roots, rootSet)
	}
}

func insertMatchIntoTree(item *workflowy.Item, ancestors []*workflowy.Item, matchPositions []MatchPosition, nodeMap map[string]*TreeNode, roots *[]*TreeNode, rootSet map[string]bool) {
	// Ensure all ancestor nodes exist in the trie
	var parent *TreeNode
	for _, ancestor := range ancestors {
		node, exists := nodeMap[ancestor.ID]
		if !exists {
			node = &TreeNode{
				ID:         ancestor.ID,
				Name:       stripHTMLTags(ancestor.Name),
				URL:        fmt.Sprintf("https://workflowy.com/#/%s", ancestor.ID),
				CreatedAt:  ancestor.CreatedAt,
				ModifiedAt: ancestor.ModifiedAt,
			}
			nodeMap[ancestor.ID] = node
			if parent == nil {
				if !rootSet[ancestor.ID] {
					*roots = append(*roots, node)
					rootSet[ancestor.ID] = true
				}
			} else {
				parent.Children = append(parent.Children, node)
			}
		}
		parent = node
	}

	// Create the match leaf node
	name := stripHTMLTags(item.Name)
	matchNode := &TreeNode{
		ID:              item.ID,
		Name:            name,
		URL:             fmt.Sprintf("https://workflowy.com/#/%s", item.ID),
		IsMatch:         true,
		HighlightedName: HighlightMatches(name, matchPositions),
		MatchPositions:  matchPositions,
		CreatedAt:       item.CreatedAt,
		ModifiedAt:      item.ModifiedAt,
	}
	nodeMap[item.ID] = matchNode

	if parent == nil {
		if !rootSet[item.ID] {
			*roots = append(*roots, matchNode)
			rootSet[item.ID] = true
		}
	} else {
		parent.Children = append(parent.Children, matchNode)
	}
}

func SortTreeNodes(nodes []*TreeNode, order OrderBy) {
	sort.SliceStable(nodes, func(i, j int) bool {
		cmp := compareTreeNodes(nodes[i], nodes[j], order)
		if order.Ascending {
			return cmp < 0
		}
		return cmp > 0
	})
	for _, node := range nodes {
		if len(node.Children) > 0 {
			SortTreeNodes(node.Children, order)
		}
	}
}

func compareTreeNodes(a, b *TreeNode, order OrderBy) int {
	switch order.Field {
	case "match", "parent", "path":
		return strings.Compare(a.Name, b.Name)
	case "modified":
		return compareInt64(descendantTimestamp(a, "modified", order.Ascending), descendantTimestamp(b, "modified", order.Ascending))
	case "created":
		return compareInt64(descendantTimestamp(a, "created", order.Ascending), descendantTimestamp(b, "created", order.Ascending))
	default:
		return 0
	}
}

func descendantTimestamp(node *TreeNode, field string, ascending bool) int64 {
	ts := treeNodeTimestamp(node, field)
	for _, child := range node.Children {
		v := descendantTimestamp(child, field, ascending)
		if ascending && v < ts {
			ts = v
		} else if !ascending && v > ts {
			ts = v
		}
	}
	return ts
}

func treeNodeTimestamp(node *TreeNode, field string) int64 {
	if field == "created" {
		return node.CreatedAt
	}
	return node.ModifiedAt
}
