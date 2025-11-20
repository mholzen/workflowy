package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mholzen/workflowy/pkg/workflowy"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	verb := os.Args[1]
	format, depth, apiKeyFile, logLevel, showEmptyNames, args := parseFlags(verb, os.Args[2:])

	setupLogging(logLevel)

	switch verb {
	case "get":
		handleGet(args, format, depth, apiKeyFile, showEmptyNames)
	case "list":
		handleList(args, format, apiKeyFile, showEmptyNames)
	default:
		fmt.Printf("Unknown command: %s\n\n", verb)
		printUsage()
		os.Exit(1)
	}
}

func parseFlags(verb string, args []string) (format string, depth int, apiKeyFile string, logLevel string, showEmptyNames bool, remainingArgs []string) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Error getting home directory: %v", err)
	}
	defaultAPIKeyFile := filepath.Join(homeDir, ".workflowy", "api.key")

	fs := flag.NewFlagSet(verb, flag.ExitOnError)
	formatPtr := fs.String("format", "md", "Output format: json, md, or markdown")
	fs.StringVar(formatPtr, "f", "md", "Output format: json, md, or markdown (shorthand)")
	depthPtr := fs.Int("depth", 2, "Recursion depth for tree operations (positive integer)")
	fs.IntVar(depthPtr, "d", 2, "Recursion depth for tree operations (shorthand)")
	apiKeyFilePtr := fs.String("api-key-file", defaultAPIKeyFile, "Path to API key file")
	logLevelPtr := fs.String("log", "info", "Log level: debug, info, warn, error")
	showEmptyNamesPtr := fs.Bool("include-empty-names", false, "Include items with empty names")

	fs.Parse(args)

	if *formatPtr != "json" && *formatPtr != "md" && *formatPtr != "markdown" {
		fmt.Printf("Error: format must be 'json', 'md', or 'markdown'\n")
		os.Exit(1)
	}

	if *depthPtr < 0 {
		fmt.Printf("Error: depth must be a non-negative integer\n")
		os.Exit(1)
	}

	return *formatPtr, *depthPtr, *apiKeyFilePtr, *logLevelPtr, *showEmptyNamesPtr, fs.Args()
}

func setupLogging(level string) {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		fmt.Printf("Error: log level must be one of: debug, info, warn, error\n")
		os.Exit(1)
	}

	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
	})
	slog.SetDefault(slog.New(handler))
}

func printUsage() {
	progName := filepath.Base(os.Args[0])
	fmt.Printf("Usage: %s <command> [options] [item_id]\n\n", progName)
	fmt.Println("Commands:")
	fmt.Printf("  %s get [item_id] [options]                 Get item with optional recursive children (root if omitted)\n", progName)
	fmt.Printf("  %s list [item_id] [options]                List children of an item (root if omitted)\n", progName)
	fmt.Println("\nGlobal Options:")
	fmt.Println("  --format, -f json|md|markdown    Output format (default: md)")
	fmt.Println("  --depth, -d N           Recursion depth for tree operations (default: 2)")
	fmt.Println("  --api-key-file FILE     Path to API key file (default: ~/.workflowy/api.key)")
	fmt.Println("  --log LEVEL             Log level: debug, info, warn, error (default: info)")
	fmt.Println("  --include-empty-names   Include items with empty names (default: exclude)")
	fmt.Println("\nIf no item_id is provided, operations will be performed on the root")
	fmt.Println("\nObtaining item_id:")
	fmt.Println("  In your browser's developer tools, inspect the item text in WorkFlowy.")
	fmt.Println("  The item_id comes from the 'projectid' attribute on an encompassing <div>,")
	fmt.Println("  usually 3 layers above the <span> containing the text.")
	fmt.Println("\nFormat options:")
	fmt.Println("  json      Output as JSON (default)")
	fmt.Println("  md        Output as Markdown list")
	fmt.Println("  markdown  Output as Markdown list (alias for md)")
}

// Common helper functions
func getItemID(args []string) string {
	if len(args) < 1 || args[0] == "" {
		// No item_id provided or empty string - default to root
		return "None"
	}
	return args[0]
}

func createClient(apiKeyFile string) *workflowy.WorkflowyClient {
	slog.Debug("loading API key", "file", apiKeyFile)
	return workflowy.NewWorkflowyClient(workflowy.WithAPIKeyFromFile(apiKeyFile))
}

func printJSON(response interface{}) {
	prettyJSON, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		log.Fatalf("Error formatting JSON: %v", err)
	}
	fmt.Printf("%s\n", prettyJSON)
}

func sortItemsByPriority(items []*workflowy.Item) {
	sort.Slice(items, func(i, j int) bool {
		return items[i].Priority < items[j].Priority
	})

	// Recursively sort children
	for _, item := range items {
		if len(item.Children) > 0 {
			sortItemsByPriority(item.Children)
		}
	}
}

func itemToMarkdown(item *workflowy.Item, depth int) string {
	indent := strings.Repeat("  ", depth)
	result := fmt.Sprintf("%s- %s\n", indent, item.Name)

	for _, child := range item.Children {
		result += itemToMarkdown(child, depth+1)
	}

	return result
}

func responseToMarkdown(response *workflowy.ListChildrenResponse) string {
	var result strings.Builder

	for _, item := range response.Items {
		result.WriteString(itemToMarkdown(item, 0))
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
	// Filter empty names unless requested to show them
	if !showEmptyNames {
		switch v := data.(type) {
		case *workflowy.Item:
			if len(v.Children) > 0 {
				v.Children = filterEmptyNames(v.Children)
			}
		case *workflowy.GetItemResponse:
			if len(v.Item.Children) > 0 {
				v.Item.Children = filterEmptyNames(v.Item.Children)
			}
		case *workflowy.ListChildrenResponse:
			v.Items = filterEmptyNames(v.Items)
		}
	}

	// Sort items by priority before output
	switch v := data.(type) {
	case *workflowy.Item:
		if len(v.Children) > 0 {
			sortItemsByPriority(v.Children)
		}
	case *workflowy.GetItemResponse:
		if len(v.Item.Children) > 0 {
			sortItemsByPriority(v.Item.Children)
		}
	case *workflowy.ListChildrenResponse:
		sortItemsByPriority(v.Items)
	}

	if format == "md" || format == "markdown" {
		switch v := data.(type) {
		case *workflowy.Item:
			fmt.Print(itemToMarkdown(v, 0))
		case *workflowy.GetItemResponse:
			fmt.Print(itemToMarkdown(&v.Item, 0))
		case *workflowy.ListChildrenResponse:
			fmt.Print(responseToMarkdown(v))
		default:
			// Fallback to JSON for unknown types
			printJSON(data)
		}
	} else {
		printJSON(data)
	}
}

func handleGet(args []string, format string, depth int, apiKeyFile string, showEmptyNames bool) {
	itemID := getItemID(args)
	client := createClient(apiKeyFile)

	ctx := context.Background()

	// Special case: "None" means root, so list root children instead
	if itemID == "None" {
		slog.Debug("fetching root items", "depth", depth)
		response, err := client.ListChildrenRecursiveWithDepth(ctx, itemID, depth)
		if err != nil {
			log.Fatalf("Error fetching root items: %v", err)
		}
		printOutput(response, format, showEmptyNames)
		return
	}

	// Normal case: get specific item
	slog.Debug("fetching item", "item_id", itemID, "depth", depth)
	item, err := client.GetItem(ctx, itemID)
	if err != nil {
		log.Fatalf("Error getting item: %v", err)
	}

	// If depth > 0, fetch children recursively and attach them
	if depth > 0 {
		childrenResp, err := client.ListChildrenRecursiveWithDepth(ctx, itemID, depth)
		if err != nil {
			log.Fatalf("Error fetching children: %v", err)
		}
		item.Children = childrenResp.Items
	}

	printOutput(item, format, showEmptyNames)
}

func handleList(args []string, format string, apiKeyFile string, showEmptyNames bool) {
	itemID := getItemID(args)
	client := createClient(apiKeyFile)

	ctx := context.Background()
	slog.Debug("fetching direct children", "item_id", itemID)
	response, err := client.ListChildren(ctx, itemID)
	if err != nil {
		log.Fatalf("Error listing children: %v", err)
	}

	printOutput(response, format, showEmptyNames)
}
