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
	"strings"

	"github.com/mholzen/workflowy/pkg/workflowy"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	verb := os.Args[1]
	format, depth, apiKeyFile, logLevel, args := parseFlags(verb, os.Args[2:])

	setupLogging(logLevel)

	switch verb {
	case "get":
		handleGet(args, format, apiKeyFile)
	case "list":
		handleList(args, format, apiKeyFile)
	case "tree":
		handleTree(args, format, depth, apiKeyFile)
	default:
		fmt.Printf("Unknown command: %s\n\n", verb)
		printUsage()
		os.Exit(1)
	}
}

func parseFlags(verb string, args []string) (format string, depth int, apiKeyFile string, logLevel string, remainingArgs []string) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Error getting home directory: %v", err)
	}
	defaultAPIKeyFile := filepath.Join(homeDir, ".workflowy", "api.key")

	fs := flag.NewFlagSet(verb, flag.ExitOnError)
	formatPtr := fs.String("format", "json", "Output format: json or md")
	fs.StringVar(formatPtr, "f", "json", "Output format: json or md (shorthand)")
	depthPtr := fs.Int("depth", 2, "Recursion depth for tree operations (positive integer)")
	fs.IntVar(depthPtr, "d", 2, "Recursion depth for tree operations (shorthand)")
	apiKeyFilePtr := fs.String("api-key-file", defaultAPIKeyFile, "Path to API key file")
	logLevelPtr := fs.String("log", "info", "Log level: debug, info, warn, error")

	fs.Parse(args)

	if *formatPtr != "json" && *formatPtr != "md" {
		fmt.Printf("Error: format must be 'json' or 'md'\n")
		os.Exit(1)
	}

	if *depthPtr < 0 {
		fmt.Printf("Error: depth must be a non-negative integer\n")
		os.Exit(1)
	}

	return *formatPtr, *depthPtr, *apiKeyFilePtr, *logLevelPtr, fs.Args()
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
	fmt.Printf("  %s get [item_id]                           Get details for an item (root if omitted)\n", progName)
	fmt.Printf("  %s list [item_id]                          List children of an item (root if omitted)\n", progName)
	fmt.Printf("  %s tree [item_id]                          List children recursively (root if omitted)\n", progName)
	fmt.Println("\nGlobal Options:")
	fmt.Println("  --format, -f json|md    Output format (default: json)")
	fmt.Println("  --depth, -d N           Recursion depth for tree operations (default: 2)")
	fmt.Println("  --api-key-file FILE     Path to API key file (default: ~/.workflowy/api.key)")
	fmt.Println("  --log LEVEL             Log level: debug, info, warn, error (default: info)")
	fmt.Println("\nIf no item_id is provided, operations will be performed on the root")
	fmt.Println("\nObtaining item_id:")
	fmt.Println("  In your browser's developer tools, inspect the item text in WorkFlowy.")
	fmt.Println("  The item_id comes from the 'projectid' attribute on an encompassing <div>,")
	fmt.Println("  usually 3 layers above the <span> containing the text.")
	fmt.Println("\nFormat options:")
	fmt.Println("  json  Output as JSON (default)")
	fmt.Println("  md    Output as Markdown list")
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

func printOutput(data interface{}, format string) {
	if format == "md" {
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

func handleGet(args []string, format string, apiKeyFile string) {
	itemID := getItemID(args)
	client := createClient(apiKeyFile)

	ctx := context.Background()
	response, err := client.GetItem(ctx, itemID)
	if err != nil {
		log.Fatalf("Error getting item: %v", err)
	}

	printOutput(response, format)
}

func handleList(args []string, format string, apiKeyFile string) {
	itemID := getItemID(args)
	client := createClient(apiKeyFile)

	ctx := context.Background()
	fmt.Printf("Fetching direct children for item: %s\n", itemID)
	response, err := client.ListChildren(ctx, itemID)
	if err != nil {
		log.Fatalf("Error listing children: %v", err)
	}

	printOutput(response, format)
}

func handleTree(args []string, format string, depth int, apiKeyFile string) {
	itemID := getItemID(args)
	client := createClient(apiKeyFile)

	ctx := context.Background()
	fmt.Printf("Fetching complete tree for item: %s (depth: %d)\n", itemID, depth)
	response, err := client.ListChildrenRecursiveWithDepth(ctx, itemID, depth)
	if err != nil {
		log.Fatalf("Error fetching tree: %v", err)
	}

	printOutput(response, format)
}
