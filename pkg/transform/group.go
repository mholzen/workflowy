package transform

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/mholzen/workflowy/pkg/workflowy"
)

// GroupOptions configures the group transform.
type GroupOptions struct {
	Field       string // "created" or "modified"
	Format      string // Go time format string for grouping
	Granularity string // "year", "month", "day", or custom
	Ascending   bool   // Sort order (false = newest first)
	DryRun      bool
}

// GroupResult represents the result of grouping children.
type GroupResult struct {
	ParentID     string        `json:"parent_id"`
	ParentURL    string        `json:"parent_url"`
	Groups       []DateGroup   `json:"groups"`
	Applied      bool          `json:"applied"`
	NoDateItems  []GroupedItem `json:"no_date_items,omitempty"`
}

// DateGroup represents a group of items under a date header.
type DateGroup struct {
	DateKey        string        `json:"date_key"`
	TimeTag        string        `json:"time_tag"`
	HeaderID       string        `json:"header_id,omitempty"`
	ExistingHeader bool          `json:"existing_header"`
	Items          []GroupedItem `json:"items"`
	CreatedID      string        `json:"created_id,omitempty"`
}

// GroupedItem represents an item that will be grouped.
type GroupedItem struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	URL        string `json:"url"`
	Timestamp  int64  `json:"timestamp"`
	DateKey    string `json:"date_key"`
}

func (g DateGroup) String() string {
	status := ""
	if g.ExistingHeader {
		status = " [existing header]"
	}
	return fmt.Sprintf("%s (%d items)%s → %s", g.DateKey, len(g.Items), status, g.TimeTag)
}

func (r GroupResult) String() string {
	var b strings.Builder
	for _, g := range r.Groups {
		fmt.Fprintln(&b, g.String())
		for _, item := range g.Items {
			fmt.Fprintf(&b, "  - %s\n", truncateName(item.Name, 60))
		}
	}
	if len(r.NoDateItems) > 0 {
		fmt.Fprintf(&b, "No date (%d items)\n", len(r.NoDateItems))
		for _, item := range r.NoDateItems {
			fmt.Fprintf(&b, "  - %s\n", truncateName(item.Name, 60))
		}
	}
	return b.String()
}

func truncateName(s string, maxLen int) string {
	// Strip HTML tags first
	s = stripHTMLTags(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

var htmlTagPattern = regexp.MustCompile(`<[^>]*>`)

func stripHTMLTags(s string) string {
	return htmlTagPattern.ReplaceAllString(s, "")
}

// TimeTagPattern matches Workflowy time tags.
var TimeTagPattern = regexp.MustCompile(`<time\s+startYear="(\d+)"(?:\s+startMonth="(\d+)")?(?:\s+startDay="(\d+)")?[^>]*>[^<]*</time>`)

// ParseGroupBy parses a --by value into GroupOptions fields.
func ParseGroupBy(value string) (field, format, granularity string, err error) {
	for _, f := range []string{"modified", "created"} {
		if value == f {
			return f, "2006-01-02", "day", nil
		}
		prefix := f + "."
		if strings.HasPrefix(value, prefix) {
			unit := value[len(prefix):]
			format, granularity = unitToFormatAndGranularity(unit)
			return f, format, granularity, nil
		}
	}
	return "", "", "", fmt.Errorf("invalid --by value: %q (expected modified, created, modified.<unit>, or created.<unit>)", value)
}

func unitToFormatAndGranularity(unit string) (format, granularity string) {
	switch unit {
	case "year":
		return "2006", "year"
	case "month":
		return "2006-01", "month"
	case "day":
		return "2006-01-02", "day"
	default:
		// Custom Go time format
		return unit, "custom"
	}
}

// ParseOrder parses a --order value.
func ParseOrder(value, field string) (ascending bool, err error) {
	if value == "" {
		// Default: newest first for date fields
		return false, nil
	}

	if strings.HasPrefix(value, "+") {
		value = value[1:]
		ascending = true
	} else if strings.HasPrefix(value, "-") {
		value = value[1:]
		ascending = false
	} else {
		// No prefix means descending for dates
		ascending = false
	}

	switch value {
	case "modified", "created":
		return ascending, nil
	default:
		return false, fmt.Errorf("invalid --order value: %q (expected +modified, -modified, +created, or -created)", value)
	}
}

// GenerateTimeTag creates a Workflowy time tag for a given date and granularity.
func GenerateTimeTag(t time.Time, granularity string) string {
	year := t.Year()
	month := int(t.Month())
	day := t.Day()

	switch granularity {
	case "year":
		return fmt.Sprintf(`<time startYear="%d">%d</time>`, year, year)
	case "month":
		label := t.Format("Jan 2006")
		return fmt.Sprintf(`<time startYear="%d" startMonth="%d">%s</time>`, year, month, label)
	default: // day or custom
		label := t.Format("Mon, Jan 2, 2006")
		return fmt.Sprintf(`<time startYear="%d" startMonth="%d" startDay="%d">%s</time>`, year, month, day, label)
	}
}

// ExtractDateFromTimeTag extracts year, month, day from a time tag.
func ExtractDateFromTimeTag(name string) (year, month, day int, found bool) {
	matches := TimeTagPattern.FindStringSubmatch(name)
	if matches == nil {
		return 0, 0, 0, false
	}

	fmt.Sscanf(matches[1], "%d", &year)
	if matches[2] != "" {
		fmt.Sscanf(matches[2], "%d", &month)
	}
	if matches[3] != "" {
		fmt.Sscanf(matches[3], "%d", &day)
	}
	return year, month, day, true
}

// DateKeyFromTimeTag extracts a date key from a time tag based on granularity.
func DateKeyFromTimeTag(name, granularity string) (string, bool) {
	year, month, day, found := ExtractDateFromTimeTag(name)
	if !found {
		return "", false
	}

	switch granularity {
	case "year":
		return fmt.Sprintf("%04d", year), true
	case "month":
		if month == 0 {
			return "", false
		}
		return fmt.Sprintf("%04d-%02d", year, month), true
	default: // day
		if month == 0 || day == 0 {
			return "", false
		}
		return fmt.Sprintf("%04d-%02d-%02d", year, month, day), true
	}
}

// CollectGroupResults groups children of the given items by date.
func CollectGroupResults(items []*workflowy.Item, opts GroupOptions) []GroupResult {
	var results []GroupResult

	for _, item := range items {
		result := groupChildren(item, opts)
		if len(result.Groups) > 0 || len(result.NoDateItems) > 0 {
			results = append(results, result)
		}
	}

	return results
}

func groupChildren(parent *workflowy.Item, opts GroupOptions) GroupResult {
	result := GroupResult{
		ParentID:  parent.ID,
		ParentURL: "https://workflowy.com/#/" + parent.ID,
	}

	if len(parent.Children) == 0 {
		return result
	}

	// Find existing date headers and regular items
	existingHeaders := make(map[string]*workflowy.Item) // dateKey -> header item
	var regularItems []*workflowy.Item

	for _, child := range parent.Children {
		if dateKey, found := DateKeyFromTimeTag(child.Name, opts.Granularity); found {
			existingHeaders[dateKey] = child
		} else {
			regularItems = append(regularItems, child)
		}
	}

	// Group regular items by date
	groups := make(map[string]*DateGroup)
	var groupOrder []string
	var noDateItems []GroupedItem

	for _, item := range regularItems {
		var timestamp int64
		if opts.Field == "created" {
			timestamp = item.CreatedAt
		} else {
			timestamp = item.ModifiedAt
		}

		if timestamp == 0 {
			noDateItems = append(noDateItems, GroupedItem{
				ID:        item.ID,
				Name:      item.Name,
				URL:       "https://workflowy.com/#/" + item.ID,
				Timestamp: 0,
				DateKey:   "",
			})
			continue
		}

		t := time.Unix(timestamp, 0)
		dateKey := t.Format(opts.Format)
		timeTag := GenerateTimeTag(t, opts.Granularity)

		if _, exists := groups[dateKey]; !exists {
			existingHeader := existingHeaders[dateKey]
			group := &DateGroup{
				DateKey:        dateKey,
				TimeTag:        timeTag,
				ExistingHeader: existingHeader != nil,
			}
			if existingHeader != nil {
				group.HeaderID = existingHeader.ID
				group.TimeTag = existingHeader.Name // Use existing tag
			}
			groups[dateKey] = group
			groupOrder = append(groupOrder, dateKey)
		}

		groups[dateKey].Items = append(groups[dateKey].Items, GroupedItem{
			ID:        item.ID,
			Name:      item.Name,
			URL:       "https://workflowy.com/#/" + item.ID,
			Timestamp: timestamp,
			DateKey:   dateKey,
		})
	}

	// Sort groups by date
	sort.Slice(groupOrder, func(i, j int) bool {
		if opts.Ascending {
			return groupOrder[i] < groupOrder[j]
		}
		return groupOrder[i] > groupOrder[j]
	})

	// Build result
	for _, dateKey := range groupOrder {
		result.Groups = append(result.Groups, *groups[dateKey])
	}
	result.NoDateItems = noDateItems

	return result
}

// GroupApplier extends Applier with MoveNode for group operations.
type GroupApplier interface {
	Applier
	MoveNode(ctx context.Context, itemID string, req *workflowy.MoveNodeRequest) (*workflowy.MoveNodeResponse, error)
}

// ApplyGroupResults applies the grouping by creating headers and moving items.
func ApplyGroupResults(ctx context.Context, client GroupApplier, results []GroupResult, granularity string) error {
	for i := range results {
		result := &results[i]
		if err := applyGroupResult(ctx, client, result, granularity); err != nil {
			return err
		}
		result.Applied = true
	}
	return nil
}

func applyGroupResult(ctx context.Context, client GroupApplier, result *GroupResult, granularity string) error {
	// Process groups in reverse order so positioning works correctly
	// (we insert at "top" so last group ends up at top)
	for i := len(result.Groups) - 1; i >= 0; i-- {
		group := &result.Groups[i]

		// Create header if it doesn't exist
		if !group.ExistingHeader {
			position := "top"
			req := &workflowy.CreateNodeRequest{
				ParentID: result.ParentID,
				Name:     group.TimeTag,
				Position: &position,
			}
			resp, err := client.CreateNode(ctx, req)
			if err != nil {
				return fmt.Errorf("failed to create header for %s: %w", group.DateKey, err)
			}
			group.CreatedID = resp.ItemID
			group.HeaderID = resp.ItemID
		}

		// Move items under the header (in reverse to maintain order)
		for j := len(group.Items) - 1; j >= 0; j-- {
			item := group.Items[j]
			position := "top"
			req := &workflowy.MoveNodeRequest{
				ParentID: group.HeaderID,
				Position: &position,
			}
			if _, err := client.MoveNode(ctx, item.ID, req); err != nil {
				return fmt.Errorf("failed to move item %s: %w", item.ID, err)
			}
		}
	}

	// Handle no-date items - create a "No date" header and move them there
	if len(result.NoDateItems) > 0 {
		position := "bottom"
		req := &workflowy.CreateNodeRequest{
			ParentID: result.ParentID,
			Name:     "No date",
			Position: &position,
		}
		resp, err := client.CreateNode(ctx, req)
		if err != nil {
			return fmt.Errorf("failed to create 'No date' header: %w", err)
		}

		for j := len(result.NoDateItems) - 1; j >= 0; j-- {
			item := result.NoDateItems[j]
			pos := "top"
			moveReq := &workflowy.MoveNodeRequest{
				ParentID: resp.ItemID,
				Position: &pos,
			}
			if _, err := client.MoveNode(ctx, item.ID, moveReq); err != nil {
				return fmt.Errorf("failed to move no-date item %s: %w", item.ID, err)
			}
		}
	}

	return nil
}

