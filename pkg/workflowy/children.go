package workflowy

import (
	"fmt"
	"regexp"
	"sort"
)

const (
	DefaultChildrenLimit = 50
	MaxChildrenLimit     = 200
)

type ChildrenPageOptions struct {
	Limit      int
	Offset     int
	Compact    bool
	NameFilter string
	IgnoreCase bool
}

type ChildrenPage struct {
	Items      any  `json:"items"`
	Total      int  `json:"total"`
	Limit      int  `json:"limit"`
	Offset     int  `json:"offset"`
	NextOffset *int `json:"next_offset,omitempty"`
	HasMore    bool `json:"has_more"`
}

type CompactChild struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	LayoutMode  string `json:"layoutMode,omitempty"`
	Completed   bool   `json:"completed"`
	HasChildren *bool  `json:"has_children,omitempty"`
}

func NewChildrenPage(children []*Item, opts ChildrenPageOptions) (*ChildrenPage, error) {
	if opts.Limit == 0 {
		opts.Limit = DefaultChildrenLimit
	}
	if opts.Limit < 0 {
		return nil, fmt.Errorf("limit must be non-negative")
	}
	if opts.Limit > MaxChildrenLimit {
		return nil, fmt.Errorf("limit must be at most %d", MaxChildrenLimit)
	}
	if opts.Offset < 0 {
		return nil, fmt.Errorf("offset must be non-negative")
	}

	filtered, err := filterChildrenByName(children, opts.NameFilter, opts.IgnoreCase)
	if err != nil {
		return nil, err
	}

	ordered := SortChildrenStable(filtered)
	total := len(ordered)
	end := opts.Offset + opts.Limit
	if opts.Offset > total {
		end = total
	}
	if end > total {
		end = total
	}

	pageItems := []*Item{}
	if opts.Offset < total {
		pageItems = ordered[opts.Offset:end]
	}

	hasMore := end < total
	var nextOffset *int
	if hasMore {
		next := end
		nextOffset = &next
	}

	var items any
	if opts.Compact {
		items = CompactChildren(pageItems)
	} else {
		items = CloneChildrenWithoutDescendants(pageItems)
	}

	return &ChildrenPage{
		Items:      items,
		Total:      total,
		Limit:      opts.Limit,
		Offset:     opts.Offset,
		NextOffset: nextOffset,
		HasMore:    hasMore,
	}, nil
}

func SortChildrenStable(children []*Item) []*Item {
	ordered := append([]*Item(nil), children...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Priority != ordered[j].Priority {
			return ordered[i].Priority < ordered[j].Priority
		}
		return ordered[i].ID < ordered[j].ID
	})
	return ordered
}

func CompactChildren(children []*Item) []CompactChild {
	out := make([]CompactChild, 0, len(children))
	for _, child := range children {
		compact := CompactChild{
			ID:        child.ID,
			Name:      child.Name,
			Completed: child.CompletedAt != nil,
		}
		if child.Data != nil {
			if layoutMode, ok := child.Data["layoutMode"].(string); ok {
				compact.LayoutMode = layoutMode
			}
		}
		if child.Children != nil {
			hasChildren := len(child.Children) > 0
			compact.HasChildren = &hasChildren
		}
		out = append(out, compact)
	}
	return out
}

func CloneChildrenWithoutDescendants(children []*Item) []*Item {
	out := make([]*Item, 0, len(children))
	for _, child := range children {
		clone := *child
		clone.Children = nil
		out = append(out, &clone)
	}
	return out
}

func filterChildrenByName(children []*Item, pattern string, ignoreCase bool) ([]*Item, error) {
	if pattern == "" {
		return append([]*Item(nil), children...), nil
	}
	if ignoreCase {
		pattern = "(?i)" + pattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid name_filter regular expression: %w", err)
	}

	filtered := make([]*Item, 0, len(children))
	for _, child := range children {
		if re.MatchString(child.Name) {
			filtered = append(filtered, child)
		}
	}
	return filtered, nil
}
