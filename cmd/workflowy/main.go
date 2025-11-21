package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
			{
				Name:  "post",
				Usage: "Create a new node",
				Arguments: []cli.Argument{
					&cli.StringArg{
						Name:      "name",
						UsageText: "Node name (or use --read-stdin or --read-file)",
					},
				},
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "parent-id",
						Value: "None",
						Usage: "Parent node UUID, target key, or \"None\" for top-level",
					},
					&cli.StringFlag{
						Name:  "position",
						Usage: "Position: \"top\" or \"bottom\" (omit for API default)",
					},
					&cli.StringFlag{
						Name:  "layout-mode",
						Usage: "Display mode: bullets, todo, h1, h2, h3",
					},
					&cli.StringFlag{
						Name:  "note",
						Usage: "Additional note content",
					},
					&cli.BoolFlag{
						Name:  "read-stdin",
						Usage: "Read node name from stdin instead of argument",
					},
					&cli.StringFlag{
						Name:  "read-file",
						Usage: "Read node name from file instead of argument",
					},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					setupLogging(cmd.String("log"))

					format := cmd.String("format")
					if format != "json" && format != "md" && format != "markdown" {
						return fmt.Errorf("format must be 'json', 'md', or 'markdown'")
					}

					// Validate position if provided
					position := cmd.String("position")
					if position != "" && position != "top" && position != "bottom" {
						return fmt.Errorf("position must be 'top' or 'bottom'")
					}

					// Determine input source and read name
					nameArg := cmd.StringArg("name")
					readStdin := cmd.Bool("read-stdin")
					readFile := cmd.String("read-file")

					// Count how many input sources are specified
					inputSources := 0
					if nameArg != "" {
						inputSources++
					}
					if readStdin {
						inputSources++
					}
					if readFile != "" {
						inputSources++
					}

					if inputSources == 0 {
						return fmt.Errorf("must provide node name via argument, --read-stdin, or --read-file")
					}
					if inputSources > 1 {
						return fmt.Errorf("cannot use multiple input sources (choose one: argument, --read-stdin, or --read-file)")
					}

					var name string
					var err error

					if nameArg != "" {
						name = nameArg
						slog.Debug("using name from argument", "name", name)
					} else if readStdin {
						slog.Debug("reading from stdin")
						stdinBytes, err := io.ReadAll(os.Stdin)
						if err != nil {
							return fmt.Errorf("error reading stdin: %w", err)
						}
						name = strings.TrimSpace(string(stdinBytes))
					} else if readFile != "" {
						slog.Debug("reading from file", "file", readFile)
						fileBytes, err := os.ReadFile(readFile)
						if err != nil {
							return fmt.Errorf("error reading file: %w", err)
						}
						name = strings.TrimSpace(string(fileBytes))
					}

					if name == "" {
						return fmt.Errorf("name cannot be empty")
					}

					// Build request
					req := &workflowy.CreateNodeRequest{
						ParentID: cmd.String("parent-id"),
						Name:     name,
					}

					// Add optional fields only if provided
					if position != "" {
						req.Position = &position
					}
					if layoutMode := cmd.String("layout-mode"); layoutMode != "" {
						req.LayoutMode = &layoutMode
					}
					if note := cmd.String("note"); note != "" {
						req.Note = &note
					}

					client := createClient(cmd.String("api-key-file"))
					apiCtx := context.Background()

					slog.Debug("creating node", "parent_id", req.ParentID, "name", name)
					response, err := client.CreateNode(apiCtx, req)
					if err != nil {
						return fmt.Errorf("error creating node: %w", err)
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
