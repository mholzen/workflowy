package workflowy

import (
	"fmt"
	"strings"
)

// NodeNotFoundError indicates a node was not found in the tree.
type NodeNotFoundError struct {
	NodeID string
}

func (e *NodeNotFoundError) Error() string {
	return fmt.Sprintf("node not found: %s", e.NodeID)
}

func FindItemByID(items []*Item, id string) *Item {
	for _, item := range items {
		if item.ID == id {
			return item
		}
		if found := FindItemByID(item.Children, id); found != nil {
			return found
		}
	}
	return nil
}

func FindRootItem(items []*Item, itemID string) *Item {
	if itemID == "None" {
		return nil
	}
	return FindItemByID(items, itemID)
}

func FindItemInTree(items []*Item, targetID string, maxDepth int) *Item {
	for _, item := range items {
		if item.ID == targetID {
			if maxDepth >= 0 {
				LimitItemDepth(item, maxDepth)
			}
			return item
		}
		if found := FindItemInTree(item.Children, targetID, maxDepth); found != nil {
			return found
		}
	}
	return nil
}

func LimitItemDepth(item *Item, maxDepth int) {
	if maxDepth == 0 {
		item.Children = nil
		return
	}
	for _, child := range item.Children {
		LimitItemDepth(child, maxDepth-1)
	}
}

func LimitItemsDepth(items []*Item, depth int) {
	for _, item := range items {
		if depth <= 1 {
			item.Children = nil
		} else {
			LimitItemDepth(item, depth-1)
		}
	}
}

func FlattenTree(data interface{}) *ListChildrenResponse {
	var items []*Item

	switch v := data.(type) {
	case *Item:
		items = FlattenItem(v)
	case *ListChildrenResponse:
		for _, item := range v.Items {
			items = append(items, FlattenItem(item)...)
		}
	}

	return &ListChildrenResponse{Items: items}
}

func FlattenItem(item *Item) []*Item {
	result := []*Item{item}

	for _, child := range item.Children {
		result = append(result, FlattenItem(child)...)
	}

	item.Children = nil
	return result
}

func FilterEmptyItem(item *Item) *Item {
	if item == nil {
		return nil
	}
	item.Children = FilterEmpty(item.Children)
	return item
}

func FilterEmptyList(list *ListChildrenResponse) *ListChildrenResponse {
	if list == nil {
		return nil
	}
	list.Items = FilterEmpty(list.Items)
	return list
}

func FilterEmpty(items []*Item) []*Item {
	filtered := make([]*Item, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Name) == "" {
			continue
		}
		if len(item.Children) > 0 {
			item.Children = FilterEmpty(item.Children)
		}
		filtered = append(filtered, item)
	}
	return filtered
}

// IsRestricted returns true if rootID represents an active restriction (not empty and not "None").
func IsRestricted(rootID string) bool {
	return rootID != "" && rootID != "None"
}

// IsWriteRestricted returns true if writeRootID represents an active restriction.
func IsWriteRestricted(writeRootID string) bool {
	return IsRestricted(writeRootID)
}

// IsDescendantOf checks if targetID equals rootID or is a descendant of rootID.
func IsDescendantOf(items []*Item, rootID, targetID string) bool {
	if !IsRestricted(rootID) {
		return true
	}
	if rootID == targetID {
		return true
	}
	root := FindItemByID(items, rootID)
	if root == nil {
		return false
	}
	return FindItemByID(root.Children, targetID) != nil
}

// ValidateAccess returns an error if targetID is not within rootID scope.
// Returns NodeNotFoundError if target is not found in the tree at all.
// Returns a standard error if target exists but is not within rootID.
func ValidateAccess(items []*Item, rootID, targetID, rootLabel, operation string) error {
	if !IsRestricted(rootID) {
		return nil
	}
	if rootID == targetID {
		return nil
	}
	root := FindItemByID(items, rootID)
	if root == nil {
		return fmt.Errorf("%s denied: %s %s not found in cache", operation, rootLabel, rootID)
	}
	if FindItemByID(root.Children, targetID) != nil {
		return nil // Found as descendant
	}
	// Not found under root - could be not found at all or outside scope
	if FindItemByID(items, targetID) == nil {
		return &NodeNotFoundError{NodeID: targetID}
	}
	return fmt.Errorf("%s denied: %s is not within %s %s", operation, targetID, rootLabel, rootID)
}

// ValidateWriteAccess returns an error if targetID is not within writeRootID scope.
func ValidateWriteAccess(items []*Item, writeRootID, targetID, operation string) error {
	return ValidateAccess(items, writeRootID, targetID, "write-root", operation)
}

// ValidateReadAccess returns an error if targetID is not within readRootID scope.
func ValidateReadAccess(items []*Item, readRootID, targetID, operation string) error {
	return ValidateAccess(items, readRootID, targetID, "read-root", operation)
}

// FindItemWithAncestors searches for targetID in the tree and returns the item
// along with its ancestor chain. ancestors[0] is the top-level parent,
// ancestors[len-1] is the immediate parent. Returns (nil, nil) if not found.
func FindItemWithAncestors(items []*Item, targetID string) (*Item, []*Item) {
	return findItemWithAncestors(items, targetID, nil)
}

func findItemWithAncestors(items []*Item, targetID string, ancestors []*Item) (*Item, []*Item) {
	for _, item := range items {
		if item.ID == targetID {
			return item, ancestors
		}
		if found, chain := findItemWithAncestors(item.Children, targetID, append(ancestors, item)); found != nil {
			return found, chain
		}
	}
	return nil, nil
}

func SliceAncestorsTo(ancestors []*Item, ancestorID string) ([]*Item, error) {
	for i, a := range ancestors {
		if a.ID == ancestorID {
			return ancestors[i:], nil
		}
	}
	return nil, fmt.Errorf("ancestor %s not found in ancestor chain", ancestorID)
}

func TruncateAncestors(ancestors []*Item, depth int) []*Item {
	if depth == 0 {
		return nil
	}
	if ancestors == nil {
		return nil
	}
	if depth < 0 || depth >= len(ancestors) {
		return ancestors
	}
	return ancestors[len(ancestors)-depth:]
}

// BuildAncestorSpine creates a spine tree from ancestors and target.
// Each ancestor is shallow-copied with only the path child in its Children.
// The target keeps its children, depth-limited by maxDepth.
// Returns the target as-is when ancestors is empty.
func BuildAncestorSpine(target *Item, ancestors []*Item, maxDepth int) *Item {
	if maxDepth >= 0 {
		LimitItemDepth(target, maxDepth)
	}

	if len(ancestors) == 0 {
		return target
	}

	// Build spine from bottom up: start with target, wrap in ancestor copies
	child := target
	for i := len(ancestors) - 1; i >= 0; i-- {
		ancestor := ancestors[i]
		copy := &Item{
			ID:          ancestor.ID,
			Name:        ancestor.Name,
			Note:        ancestor.Note,
			Priority:    ancestor.Priority,
			Data:        ancestor.Data,
			CreatedAt:   ancestor.CreatedAt,
			ModifiedAt:  ancestor.ModifiedAt,
			CompletedAt: ancestor.CompletedAt,
			Children:    []*Item{child},
		}
		child = copy
	}

	return child
}
