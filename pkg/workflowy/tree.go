package workflowy

import "strings"

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
