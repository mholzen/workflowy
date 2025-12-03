package main

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/mholzen/workflowy/pkg/workflowy"
)

type SearchResult struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	HighlightedName string          `json:"highlighted_name"`
	URL             string          `json:"url"`
	MatchPositions  []MatchPosition `json:"match_positions"`
}

type MatchPosition struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

func (sr SearchResult) String() string {
	return fmt.Sprintf("- [%s](%s)", sr.HighlightedName, sr.URL)
}

func findRootItem(items []*workflowy.Item, itemID string) *workflowy.Item {
	if itemID == "None" {
		return nil
	}

	return findItemByID(items, itemID)
}

func searchItems(items []*workflowy.Item, pattern string, useRegexp, ignoreCase bool) []SearchResult {
	var results []SearchResult

	for _, item := range items {
		collectSearchResults(item, pattern, useRegexp, ignoreCase, &results)
	}

	return results
}

func collectSearchResults(item *workflowy.Item, pattern string, useRegexp, ignoreCase bool, results *[]SearchResult) {
	name := item.Name
	matchPositions := findMatches(name, pattern, useRegexp, ignoreCase)

	if len(matchPositions) > 0 {
		highlightedName := highlightMatches(name, matchPositions)
		*results = append(*results, SearchResult{
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

func findMatches(text, pattern string, useRegexp, ignoreCase bool) []MatchPosition {
	var positions []MatchPosition

	if useRegexp {
		re, err := regexp.Compile(maybeIgnoreCase(pattern, ignoreCase))
		if err != nil {
			slog.Warn("invalid regex pattern", "pattern", pattern, "error", err)
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

func maybeIgnoreCase(pattern string, ignoreCase bool) string {
	if ignoreCase {
		return "(?i)" + pattern
	}
	return pattern
}

func highlightMatches(text string, positions []MatchPosition) string {
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
