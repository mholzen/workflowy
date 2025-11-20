package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mholzen/workflowy/pkg/workflowy"
	"github.com/urfave/cli/v3"
)

func main() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Error getting home directory: %v", err)
	}
	defaultAPIKeyFile := filepath.Join(homeDir, ".workflowy", "api.key")

	cmd := &cli.Command{
		Name:  "workflowy",
		Usage: "Interact with WorkFlowy API",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "format",
				Aliases: []string{"f"},
				Value:   "md",
				Usage:   "Output format: json, md, or markdown",
			},
			&cli.IntFlag{
				Name:    "depth",
				Aliases: []string{"d"},
				Value:   2,
				Usage:   "Recursion depth for tree operations (positive integer)",
			},
			&cli.StringFlag{
				Name:  "api-key-file",
				Value: defaultAPIKeyFile,
				Usage: "Path to API key file",
			},
			&cli.StringFlag{
				Name:  "log",
				Value: "info",
				Usage: "Log level: debug, info, warn, error",
			},
			&cli.BoolFlag{
				Name:  "include-empty-names",
				Value: false,
				Usage: "Include items with empty names",
			},
		},
		Commands: []*cli.Command{
			{
				Name:  "get",
				Usage: "Get item with optional recursive children (root if omitted)",
				Arguments: []cli.Argument{
					&cli.StringArg{
						Name:      "item_id",
						Value:     "None",
						UsageText: "WorkFlowy item ID (default: root)",
					},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					setupLogging(cmd.String("log"))

					format := cmd.String("format")
					if format != "json" && format != "md" && format != "markdown" {
						return fmt.Errorf("format must be 'json', 'md', or 'markdown'")
					}

					depth := cmd.Int("depth")
					if depth < 0 {
						return fmt.Errorf("depth must be a non-negative integer")
					}

					itemID := cmd.StringArg("item_id")
					client := createClient(cmd.String("api-key-file"))

					apiCtx := context.Background()

					// Special case: "None" means root, so list root children instead
					if itemID == "None" {
						slog.Debug("fetching root items", "depth", depth)
						response, err := client.ListChildrenRecursiveWithDepth(apiCtx, itemID, depth)
						if err != nil {
							return fmt.Errorf("error fetching root items: %w", err)
						}
						printOutput(response, format, cmd.Bool("include-empty-names"))
						return nil
					}

					// Normal case: get specific item
					slog.Debug("fetching item", "item_id", itemID, "depth", depth)
					item, err := client.GetItem(apiCtx, itemID)
					if err != nil {
						return fmt.Errorf("error getting item: %w", err)
					}

					// If depth > 0, fetch children recursively and attach them
					if depth > 0 {
						childrenResp, err := client.ListChildrenRecursiveWithDepth(apiCtx, itemID, depth)
						if err != nil {
							return fmt.Errorf("error fetching children: %w", err)
						}
						item.Children = childrenResp.Items
					}

					printOutput(item, format, cmd.Bool("include-empty-names"))
					return nil
				},
			},
			{
				Name:  "list",
				Usage: "List direct children of an item (root if omitted)",
				Arguments: []cli.Argument{
					&cli.StringArg{
						Name:      "item_id",
						Value:     "None",
						UsageText: "WorkFlowy item ID (default: root)",
					},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					setupLogging(cmd.String("log"))

					format := cmd.String("format")
					if format != "json" && format != "md" && format != "markdown" {
						return fmt.Errorf("format must be 'json', 'md', or 'markdown'")
					}

					itemID := cmd.StringArg("item_id")
					client := createClient(cmd.String("api-key-file"))

					apiCtx := context.Background()
					slog.Debug("fetching direct children", "item_id", itemID)
					response, err := client.ListChildren(apiCtx, itemID)
					if err != nil {
						return fmt.Errorf("error listing children: %w", err)
					}

					printOutput(response, format, cmd.Bool("include-empty-names"))
					return nil
				},
			},
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
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
