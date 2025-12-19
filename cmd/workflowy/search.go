package main

import (
	"github.com/mholzen/workflowy/pkg/search"
	"github.com/mholzen/workflowy/pkg/workflowy"
)

type SearchResult = search.Result
type MatchPosition = search.MatchPosition

func findRootItem(items []*workflowy.Item, itemID string) *workflowy.Item {
	return workflowy.FindRootItem(items, itemID)
}

func searchItems(items []*workflowy.Item, pattern string, useRegexp, ignoreCase bool) []SearchResult {
	return search.SearchItems(items, pattern, useRegexp, ignoreCase)
}
