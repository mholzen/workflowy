// TODO: Add --name and --note flags to search command to allow searching in notes
// (currently only searches in item.Name)
package search

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/mholzen/workflowy/pkg/workflowy"
)

type Result struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	HighlightedName string          `json:"highlighted_name"`
	URL             string          `json:"url"`
	MatchPositions  []MatchPosition `json:"match_positions"`
}

func (r Result) String() string {
	return fmt.Sprintf("- [%s](%s)", r.HighlightedName, r.URL)
}

type MatchPosition struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

func SearchItems(items []*workflowy.Item, pattern string, useRegexp, ignoreCase bool) []Result {
	var results []Result

	for _, item := range items {
		collectSearchResults(item, pattern, useRegexp, ignoreCase, &results)
	}

	return results
}

func collectSearchResults(item *workflowy.Item, pattern string, useRegexp, ignoreCase bool, results *[]Result) {
	name := stripHTMLTags(item.Name)
	matchPositions := FindMatches(name, pattern, useRegexp, ignoreCase)

	if len(matchPositions) > 0 {
		highlightedName := HighlightMatches(name, matchPositions)
		*results = append(*results, Result{
			ID:              item.ID,
			Name:            name,
			HighlightedName: highlightedName,
			URL:             fmt.Sprintf("https://workflowy.com/#/%s", item.ID),
			MatchPositions:  matchPositions,
		})
	}

	for _, child := range item.Children {
		collectSearchResults(child, pattern, useRegexp, ignoreCase, results)
	}
}

func FindMatches(text, pattern string, useRegexp, ignoreCase bool) []MatchPosition {
	var positions []MatchPosition

	if useRegexp {
		re, err := CompileRegexp(pattern, ignoreCase)
		if err != nil {
			return positions
		}

		matches := re.FindAllStringIndex(text, -1)
		for _, match := range matches {
			positions = append(positions, MatchPosition{Start: match[0], End: match[1]})
		}
	} else {
		searchText := text
		searchPattern := pattern

		if ignoreCase {
			searchText = strings.ToLower(text)
			searchPattern = strings.ToLower(pattern)
		}

		start := 0
		for {
			index := strings.Index(searchText[start:], searchPattern)
			if index == -1 {
				break
			}
			absIndex := start + index
			positions = append(positions, MatchPosition{
				Start: absIndex,
				End:   absIndex + len(pattern),
			})
			start = absIndex + len(pattern)
		}
	}

	return positions
}

func CompileRegexp(pattern string, ignoreCase bool) (*regexp.Regexp, error) {
	if ignoreCase {
		pattern = "(?i)" + pattern
	}
	return regexp.Compile(pattern)
}

// GroupedResult represents search results grouped by a generic key.
type GroupedResult struct {
	GroupKey   string   `json:"group_key"`
	GroupLabel string   `json:"group_label"`
	Children   []Result `json:"children"`
}

func (g GroupedResult) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "- %s", g.GroupLabel)
	for _, child := range g.Children {
		fmt.Fprintf(&b, "\n  %s", child.String())
	}
	return b.String()
}

// GroupingStrategy determines how search results are grouped.
type GroupingStrategy interface {
	GroupKey(item *workflowy.Item, ancestors []*workflowy.Item) string
	GroupLabel(item *workflowy.Item, ancestors []*workflowy.Item) string
}

var htmlTagStripper = regexp.MustCompile(`<[^>]*>`)

func stripHTMLTags(s string) string {
	return htmlTagStripper.ReplaceAllString(s, "")
}

type parentStrategy struct{}

func (s parentStrategy) GroupKey(_ *workflowy.Item, ancestors []*workflowy.Item) string {
	if len(ancestors) == 0 {
		return "(root)"
	}
	return ancestors[len(ancestors)-1].ID
}

func (s parentStrategy) GroupLabel(_ *workflowy.Item, ancestors []*workflowy.Item) string {
	if len(ancestors) == 0 {
		return "[(root)](https://workflowy.com/)"
	}
	parent := ancestors[len(ancestors)-1]
	return fmt.Sprintf("[%s](https://workflowy.com/#/%s)", stripHTMLTags(parent.Name), parent.ID)
}

type pathStrategy struct {
	MaxLen int
}

func truncate(s string, maxLen int) string {
	if maxLen <= 0 || len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func (s pathStrategy) GroupKey(_ *workflowy.Item, ancestors []*workflowy.Item) string {
	if len(ancestors) == 0 {
		return "(root)"
	}
	ids := make([]string, len(ancestors))
	for i, a := range ancestors {
		ids[i] = a.ID
	}
	return strings.Join(ids, "/")
}

func (s pathStrategy) GroupLabel(_ *workflowy.Item, ancestors []*workflowy.Item) string {
	if len(ancestors) == 0 {
		return "[(root)](https://workflowy.com/)"
	}
	parts := make([]string, len(ancestors))
	for i, a := range ancestors {
		name := truncate(stripHTMLTags(a.Name), s.MaxLen)
		parts[i] = fmt.Sprintf("[%s](https://workflowy.com/#/%s)", name, a.ID)
	}
	return strings.Join(parts, " > ")
}

type dateStrategy struct {
	Field  string
	Format string
}

func (s dateStrategy) timestamp(item *workflowy.Item) time.Time {
	var epoch int64
	if s.Field == "created" {
		epoch = item.CreatedAt
	} else {
		epoch = item.ModifiedAt
	}
	return time.Unix(epoch, 0)
}

func (s dateStrategy) GroupKey(item *workflowy.Item, _ []*workflowy.Item) string {
	return s.timestamp(item).Format(s.Format)
}

func (s dateStrategy) GroupLabel(item *workflowy.Item, _ []*workflowy.Item) string {
	return s.timestamp(item).Format(s.Format)
}

// ParseGroupBy parses a --group-by value into a GroupingStrategy.
func ParseGroupBy(value string, pathMaxLen int) (GroupingStrategy, error) {
	switch value {
	case "parent":
		return parentStrategy{}, nil
	case "path":
		return pathStrategy{MaxLen: pathMaxLen}, nil
	}

	for _, field := range []string{"modified", "created"} {
		if value == field {
			return dateStrategy{Field: field, Format: "2006-01-02 15:04"}, nil
		}
		prefix := field + "."
		if strings.HasPrefix(value, prefix) {
			unit := value[len(prefix):]
			format := unitToFormat(unit)
			return dateStrategy{Field: field, Format: format}, nil
		}
	}

	return nil, fmt.Errorf("unknown --group-by value: %q (expected parent, path, modified.<unit>, or created.<unit>)", value)
}

func unitToFormat(unit string) string {
	switch unit {
	case "year":
		return "2006"
	case "month":
		return "2006-01"
	case "day":
		return "2006-01-02"
	default:
		return unit
	}
}

// SearchItemsGroupedBy searches items and groups results using the given strategy.
func SearchItemsGroupedBy(items []*workflowy.Item, pattern string, useRegexp, ignoreCase bool, strategy GroupingStrategy) []GroupedResult {
	groupMap := make(map[string]*GroupedResult)
	var groupOrder []string

	for _, item := range items {
		collectGroupedByResults(item, nil, pattern, useRegexp, ignoreCase, strategy, groupMap, &groupOrder)
	}

	results := make([]GroupedResult, 0, len(groupOrder))
	for _, key := range groupOrder {
		results = append(results, *groupMap[key])
	}
	return results
}

func collectGroupedByResults(item *workflowy.Item, ancestors []*workflowy.Item, pattern string, useRegexp, ignoreCase bool, strategy GroupingStrategy, groupMap map[string]*GroupedResult, groupOrder *[]string) {
	name := stripHTMLTags(item.Name)
	matchPositions := FindMatches(name, pattern, useRegexp, ignoreCase)

	if len(matchPositions) > 0 {
		key := strategy.GroupKey(item, ancestors)
		label := strategy.GroupLabel(item, ancestors)

		if _, exists := groupMap[key]; !exists {
			groupMap[key] = &GroupedResult{
				GroupKey:   key,
				GroupLabel: label,
			}
			*groupOrder = append(*groupOrder, key)
		}

		highlightedName := HighlightMatches(name, matchPositions)
		groupMap[key].Children = append(groupMap[key].Children, Result{
			ID:              item.ID,
			Name:            name,
			HighlightedName: highlightedName,
			URL:             fmt.Sprintf("https://workflowy.com/#/%s", item.ID),
			MatchPositions:  matchPositions,
		})
	}

	childAncestors := append(append([]*workflowy.Item{}, ancestors...), item)
	for _, child := range item.Children {
		collectGroupedByResults(child, childAncestors, pattern, useRegexp, ignoreCase, strategy, groupMap, groupOrder)
	}
}

func HighlightMatches(text string, positions []MatchPosition) string {
	if len(positions) == 0 {
		return text
	}

	var result strings.Builder
	lastEnd := 0

	for _, pos := range positions {
		result.WriteString(text[lastEnd:pos.Start])
		result.WriteString("**")
		result.WriteString(text[pos.Start:pos.End])
		result.WriteString("**")
		lastEnd = pos.End
	}

	result.WriteString(text[lastEnd:])
	return result.String()
}
