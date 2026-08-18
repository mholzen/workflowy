package workflowy

import (
	"fmt"
	"sort"
	"strings"
)

const (
	DefaultPageLimit = 50
	MaxPageLimit     = 200
)

// NodeRef identifies the node whose children a page covers.
type NodeRef struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
	Note string `json:"note,omitempty"`
}

// Page describes one bounded window over an ordered result set.
type Page struct {
	Node       *NodeRef `json:"node,omitempty"`
	Items      any      `json:"items"`
	Total      int      `json:"total"`
	Limit      int      `json:"limit"`
	Offset     int      `json:"offset"`
	NextOffset *int     `json:"next_offset,omitempty"`
	HasMore    bool     `json:"has_more"`
}

// NewPage validates the requested window and returns pagination metadata.
// Callers supply DefaultPageLimit themselves when no limit was requested, so
// that an explicit limit of 0 stays an error instead of silently paging by 50.
func NewPage[T any](items []T, limit, offset int) (*Page, error) {
	if limit < 1 {
		return nil, fmt.Errorf("limit must be at least 1")
	}
	if limit > MaxPageLimit {
		return nil, fmt.Errorf("limit must be at most %d", MaxPageLimit)
	}
	if offset < 0 {
		return nil, fmt.Errorf("offset must be non-negative")
	}

	total := len(items)
	start := min(offset, total)
	end := min(start+limit, total)
	pageItems := items[start:end]
	if pageItems == nil {
		pageItems = make([]T, 0)
	}

	hasMore := end < total
	var nextOffset *int
	if hasMore {
		next := end
		nextOffset = &next
	}

	return &Page{
		Items:      pageItems,
		Total:      total,
		Limit:      limit,
		Offset:     offset,
		NextOffset: nextOffset,
		HasMore:    hasMore,
	}, nil
}

// Window returns the 1-based inclusive range this page covers. Both values are
// zero when the requested offset lies past the end of the result set.
func (p *Page) Window() (first, last int) {
	start := min(p.Offset, p.Total)
	end := min(start+p.Limit, p.Total)
	if start >= end {
		return 0, 0
	}
	return start + 1, end
}

// SortOrder is shared by get and list. A leading + or - selects direction.
type SortOrder struct {
	Field     string
	Ascending bool
}

// ParseSortOrder parses priority, name, created, and modified sort values.
func ParseSortOrder(value string) (SortOrder, error) {
	if value == "" {
		value = "priority"
	}

	ascending := true
	field := value
	if strings.HasPrefix(field, "+") {
		field = field[1:]
	} else if strings.HasPrefix(field, "-") {
		field = field[1:]
		ascending = false
	} else if field == "created" || field == "modified" {
		ascending = false
	}

	switch field {
	case "priority", "name", "created", "modified":
		return SortOrder{Field: field, Ascending: ascending}, nil
	default:
		return SortOrder{}, fmt.Errorf("unknown sort value: %q (expected priority, name, created, or modified)", value)
	}
}

// SortItems orders siblings and, when recursive is true, every descendant list.
func SortItems(items []*Item, order SortOrder, recursive bool) {
	sort.SliceStable(items, func(i, j int) bool {
		cmp := compareItems(items[i], items[j], order.Field)
		if order.Ascending {
			return cmp < 0
		}
		return cmp > 0
	})

	if recursive {
		for _, item := range items {
			SortItems(item.Children, order, true)
		}
	}
}

func compareItems(a, b *Item, field string) int {
	switch field {
	case "priority":
		return compareInts(a.Priority, b.Priority)
	case "name":
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	case "created":
		return compareInt64s(a.CreatedAt, b.CreatedAt)
	case "modified":
		return compareInt64s(a.ModifiedAt, b.ModifiedAt)
	default:
		return 0
	}
}

func compareInts(a, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func compareInt64s(a, b int64) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// IsStructuralSort reports whether an order describes a node's position among
// its siblings rather than a value carried by the node itself. Priority is the
// sibling index, so it only means anything inside one parent: sorting a
// flattened list by it interleaves nodes from unrelated parents. Structural
// orders are applied to the tree before flattening; the rest are applied to the
// flat result set.
func IsStructuralSort(order SortOrder) bool {
	return order.Field == "priority"
}

// SortedFlatList turns a get response into the flat list that list returns.
// A structural order is applied to each sibling group before flattening, so the
// outline survives; any other order ranks the whole flat result set by a value
// every node carries, which is what makes it pageable.
func SortedFlatList(data any, order SortOrder, includeEmpty bool) *ListChildrenResponse {
	structural := IsStructuralSort(order)
	if structural {
		SortTreeResult(data, order)
	}

	flat := FlattenTree(data)
	if !includeEmpty {
		flat = FilterEmptyList(flat)
	}
	if !structural {
		SortItems(flat.Items, order, false)
	}
	return flat
}

// NodeRefFor describes the node a get response paginates over, or nil at root.
func NodeRefFor(data any) *NodeRef {
	item, ok := data.(*Item)
	if !ok || item == nil {
		return nil
	}
	ref := &NodeRef{ID: item.ID, Name: item.Name}
	if item.Note != nil {
		ref.Note = *item.Note
	}
	return ref
}

// TopLevelItems returns the collection represented by a get response. For a
// specific node that collection is its direct children; for root it is the
// root-level node list.
func TopLevelItems(data any) []*Item {
	switch value := data.(type) {
	case *Item:
		if value == nil {
			return []*Item{}
		}
		return value.Children
	case *ListChildrenResponse:
		if value == nil {
			return []*Item{}
		}
		return value.Items
	default:
		return []*Item{}
	}
}

// SortTreeResult applies an item sort to either supported get response shape.
func SortTreeResult(data any, order SortOrder) {
	switch value := data.(type) {
	case *Item:
		if value != nil {
			SortItems(value.Children, order, true)
		}
	case *ListChildrenResponse:
		if value != nil {
			SortItems(value.Items, order, true)
		}
	}
}
