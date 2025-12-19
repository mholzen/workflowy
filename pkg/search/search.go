package search

import (
	"fmt"
	"regexp"
	"strings"

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
	name := item.Name
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
