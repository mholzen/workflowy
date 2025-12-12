package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/mholzen/workflowy/pkg/formatter"
	"github.com/mholzen/workflowy/pkg/workflowy"
)

func printJSON(response interface{}) {
	printJSONToWriter(os.Stdout, response)
}

func printJSONToWriter(w io.Writer, response interface{}) {
	prettyJSON, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		log.Fatalf("cannot format JSON: %v", err)
	}
	fmt.Fprintf(w, "%s\n", prettyJSON)
}

func sortItemsByPriority(items []*workflowy.Item) {
	sort.Slice(items, func(i, j int) bool {
		return items[i].Priority < items[j].Priority
	})

	for _, item := range items {
		if len(item.Children) > 0 {
			sortItemsByPriority(item.Children)
		}
	}
}

func itemToMarkdownList(item *workflowy.Item, depth int) string {
	indent := strings.Repeat("  ", depth)
	result := fmt.Sprintf("%s- %s\n", indent, item.Name)

	for _, child := range item.Children {
		result += itemToMarkdownList(child, depth+1)
	}

	return result
}

func responseToMarkdownList(response *workflowy.ListChildrenResponse) string {
	var result strings.Builder

	for _, item := range response.Items {
		result.WriteString(itemToMarkdownList(item, 0))
	}

	return result.String()
}

func filterEmptyNames(items []*workflowy.Item) []*workflowy.Item {
	filtered := make([]*workflowy.Item, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Name) != "" {
			if len(item.Children) > 0 {
				item.Children = filterEmptyNames(item.Children)
			}
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func printOutput(data interface{}, format string, showEmptyNames bool) {
	if !showEmptyNames {
		switch v := data.(type) {
		case *workflowy.Item:
			if len(v.Children) > 0 {
				v.Children = filterEmptyNames(v.Children)
			}
		case *workflowy.ListChildrenResponse:
			v.Items = filterEmptyNames(v.Items)
		case *workflowy.CreateNodeResponse:
		}
	}

	switch v := data.(type) {
	case *workflowy.Item:
		if len(v.Children) > 0 {
			sortItemsByPriority(v.Children)
		}
	case *workflowy.ListChildrenResponse:
		sortItemsByPriority(v.Items)
	case *workflowy.CreateNodeResponse:
	}

	switch format {
	case "list":
		switch v := data.(type) {
		case *workflowy.Item:
			fmt.Print(itemToMarkdownList(v, 0))
		case *workflowy.ListChildrenResponse:
			fmt.Print(responseToMarkdownList(v))
		case []SearchResult:
			for _, result := range v {
				fmt.Println(result.String())
			}
		default:
			printJSON(data)
		}
	case "markdown":
		switch v := data.(type) {
		case *workflowy.Item:
			output, err := formatter.FormatItemsAsMarkdown(v.Children)
			if err != nil {
				log.Fatalf("cannot format markdown: %v", err)
			}
			fmt.Print(output)
		case *workflowy.ListChildrenResponse:
			output, err := formatter.FormatItemsAsMarkdown(v.Items)
			if err != nil {
				log.Fatalf("cannot format markdown: %v", err)
			}
			fmt.Print(output)
		case []SearchResult:
			for _, result := range v {
				fmt.Println(result.String())
			}
		default:
			printJSON(data)
		}
	default:
		printJSON(data)
	}
}
