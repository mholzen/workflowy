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

	"github.com/mholzen/workflowy/pkg/formatter"
	"github.com/mholzen/workflowy/pkg/reports"
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
			{
				Name:  "export",
				Usage: "Export all nodes from WorkFlowy",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:  "force-refresh",
						Usage: "Force refresh from API, bypassing cache",
					},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					setupLogging(cmd.String("log"))

					format := cmd.String("format")
					if format != "json" && format != "md" && format != "markdown" {
						return fmt.Errorf("format must be 'json', 'md', or 'markdown'")
					}

					client := createClient(cmd.String("api-key-file"))
					apiCtx := context.Background()

					forceRefresh := cmd.Bool("force-refresh")
					slog.Debug("exporting all nodes", "force_refresh", forceRefresh)

					response, err := client.ExportNodesWithCache(apiCtx, forceRefresh)
					if err != nil {
						return fmt.Errorf("error exporting nodes: %w", err)
					}

					slog.Info("export complete", "node_count", len(response.Nodes))

					// Reconstruct tree from flat export
					slog.Debug("reconstructing tree from export data")
					root := workflowy.BuildTreeFromExport(response.Nodes)
					slog.Debug("tree reconstructed", "top_level_count", len(root.Children))

					// Output the tree
					if format == "json" {
						printJSON(root)
					} else {
						// For markdown, output as nested tree
						for _, child := range root.Children {
							fmt.Print(itemToMarkdown(child, 0))
						}
					}

					return nil
				},
			},
			{
				Name:  "tree",
				Usage: "Display entire WorkFlowy tree from backup file or export API",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "use-backup-file",
						Usage: "Use backup file instead of API (specify filename or leave empty for latest)",
					},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					setupLogging(cmd.String("log"))

					format := cmd.String("format")
					if format != "json" && format != "md" && format != "markdown" {
						return fmt.Errorf("format must be 'json', 'md', or 'markdown'")
					}

					var items []*workflowy.Item
					var err error

					backupFile := cmd.String("use-backup-file")
					if backupFile != "" {
						// Use backup file
						if backupFile == "true" || backupFile == "1" {
							// Flag was set without value, use latest
							slog.Debug("using latest backup file")
							items, err = workflowy.ReadLatestBackup()
						} else {
							// Flag has a specific filename
							slog.Debug("using backup file", "file", backupFile)
							items, err = workflowy.ReadBackupFile(backupFile)
						}
						if err != nil {
							return fmt.Errorf("error reading backup file: %w", err)
						}
					} else {
						// Use export API with cache
						client := createClient(cmd.String("api-key-file"))
						apiCtx := context.Background()

						slog.Debug("using export API with cache")
						response, err := client.ExportNodesWithCache(apiCtx, false)
						if err != nil {
							return fmt.Errorf("error exporting nodes: %w", err)
						}

						slog.Debug("reconstructing tree from export data")
						root := workflowy.BuildTreeFromExport(response.Nodes)
						items = root.Children
					}

					slog.Info("tree loaded", "top_level_count", len(items))

					// Output the tree
					if format == "json" {
						printJSON(items)
					} else {
						// For markdown, output as nested tree
						for _, child := range items {
							fmt.Print(itemToMarkdown(child, 0))
						}
					}

					return nil
				},
			},
			{
				Name:  "report",
				Usage: "Generate reports from WorkFlowy data",
				Commands: []*cli.Command{
					{
						Name:  "count",
						Usage: "Generate descendant count report",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "use-backup-file",
								Usage: "Use backup file instead of API (specify filename or leave empty for latest)",
							},
							&cli.StringFlag{
								Name:  "item-id",
								Value: "None",
								Usage: "Item ID to start from (default: root)",
							},
							&cli.Float64Flag{
								Name:  "threshold",
								Value: 0.01,
								Usage: "Minimum ratio threshold for filtering (0.0 to 1.0)",
							},
							&cli.BoolFlag{
								Name:  "upload",
								Usage: "Upload report to WorkFlowy instead of printing",
							},
							&cli.StringFlag{
								Name:  "parent-id",
								Value: "None",
								Usage: "Parent node ID for uploaded report (default: root)",
							},
							&cli.StringFlag{
								Name:  "position",
								Usage: "Position in parent: top or bottom",
							},
						},
						Action: func(ctx context.Context, cmd *cli.Command) error {
							setupLogging(cmd.String("log"))

							// Load tree
							items, err := loadTree(cmd)
							if err != nil {
								return err
							}

							// Find the root item
							var rootItem *workflowy.Item
							itemID := cmd.String("item-id")
							if itemID == "None" && len(items) > 0 {
								// Create a virtual root containing all top-level items
								rootItem = &workflowy.Item{
									ID:       "root",
									Name:     "Root",
									Children: items,
								}
							} else {
								rootItem = findItemByID(items, itemID)
								if rootItem == nil {
									return fmt.Errorf("item with ID %s not found", itemID)
								}
							}

							// Count descendants
							threshold := cmd.Float64("threshold")
							slog.Info("counting descendants", "threshold", threshold)
							descendants := workflowy.CountDescendants(rootItem, threshold)

							// Check if we should upload
							report := &reports.CountReportOutput{
								RootItem:    rootItem,
								Descendants: descendants,
								Threshold:   threshold,
							}
							if err := handleReportUpload(cmd, report); err != nil {
								return err
							}
							if cmd.Bool("upload") {
								return nil // Already uploaded
							}

							// Output results to stdout
							format := cmd.String("format")
							if format == "json" {
								printJSON(descendants)
							} else {
								// Markdown format
								fmt.Printf("# Descendant Count Report\n\n")
								fmt.Printf("Root: %s\n", rootItem.Name)
								fmt.Printf("Threshold: %.2f%%\n", threshold*100)
								fmt.Printf("Total descendants: %d\n\n", descendants.Count)
								printCountTree(descendants, 0)
							}

							return nil
						},
					},
					{
						Name:  "children",
						Usage: "Rank nodes by immediate children count",
						Flags: append([]cli.Flag{
							&cli.StringFlag{
								Name:  "use-backup-file",
								Usage: "Use backup file instead of API (specify filename or leave empty for latest)",
							},
							&cli.StringFlag{
								Name:  "item-id",
								Value: "None",
								Usage: "Item ID to start from (default: root)",
							},
							&cli.Float64Flag{
								Name:  "threshold",
								Value: 0.01,
								Usage: "Minimum ratio threshold for filtering (0.0 to 1.0)",
							},
							&cli.IntFlag{
								Name:  "top-n",
								Value: 20,
								Usage: "Number of top results to show (0 for all)",
							},
						}, uploadFlags()...),
						Action: func(ctx context.Context, cmd *cli.Command) error {
							setupLogging(cmd.String("log"))

							descendants, err := loadAndCountDescendants(cmd)
							if err != nil {
								return err
							}

							// Collect nodes with timestamps
							nodesWithTimestamps := workflowy.CollectNodesWithTimestamps(descendants)

							// Rank by children count
							topN := cmd.Int("top-n")
							ranked := workflowy.RankByChildrenCount(nodesWithTimestamps, topN)

							// Check if we should upload
							report := &reports.ChildrenCountReportOutput{
								Ranked: ranked,
								TopN:   topN,
							}
							if err := handleReportUpload(cmd, report); err != nil {
								return err
							}
							if cmd.Bool("upload") {
								return nil
							}

							// Output results to stdout
							format := cmd.String("format")
							if format == "json" {
								printJSON(ranked)
							} else {
								fmt.Printf("# Top Nodes by Children Count\n\n")
								for i, r := range ranked {
									fmt.Printf("%d. %s\n", i+1, r.String())
								}
							}

							return nil
						},
					},
					{
						Name:  "created",
						Usage: "Rank nodes by creation date (oldest first)",
						Flags: append([]cli.Flag{
							&cli.StringFlag{
								Name:  "use-backup-file",
								Usage: "Use backup file instead of API (specify filename or leave empty for latest)",
							},
							&cli.StringFlag{
								Name:  "item-id",
								Value: "None",
								Usage: "Item ID to start from (default: root)",
							},
							&cli.Float64Flag{
								Name:  "threshold",
								Value: 0.01,
								Usage: "Minimum ratio threshold for filtering (0.0 to 1.0)",
							},
							&cli.IntFlag{
								Name:  "top-n",
								Value: 20,
								Usage: "Number of top results to show (0 for all)",
							},
						}, uploadFlags()...),
						Action: func(ctx context.Context, cmd *cli.Command) error {
							setupLogging(cmd.String("log"))

							descendants, err := loadAndCountDescendants(cmd)
							if err != nil {
								return err
							}

							// Collect nodes with timestamps
							nodesWithTimestamps := workflowy.CollectNodesWithTimestamps(descendants)

							// Rank by created date
							topN := cmd.Int("top-n")
							ranked := workflowy.RankByCreated(nodesWithTimestamps, topN)

							// Check if we should upload
							report := &reports.CreatedReportOutput{
								Ranked: ranked,
								TopN:   topN,
							}
							if err := handleReportUpload(cmd, report); err != nil {
								return err
							}
							if cmd.Bool("upload") {
								return nil
							}

							// Output results to stdout
							format := cmd.String("format")
							if format == "json" {
								printJSON(ranked)
							} else {
								fmt.Printf("# Oldest Nodes by Creation Date\n\n")
								for i, r := range ranked {
									fmt.Printf("%d. %s\n", i+1, r.String())
								}
							}

							return nil
						},
					},
					{
						Name:  "modified",
						Usage: "Rank nodes by modification date (oldest first)",
						Flags: append([]cli.Flag{
							&cli.StringFlag{
								Name:  "use-backup-file",
								Usage: "Use backup file instead of API (specify filename or leave empty for latest)",
							},
							&cli.StringFlag{
								Name:  "item-id",
								Value: "None",
								Usage: "Item ID to start from (default: root)",
							},
							&cli.Float64Flag{
								Name:  "threshold",
								Value: 0.01,
								Usage: "Minimum ratio threshold for filtering (0.0 to 1.0)",
							},
							&cli.IntFlag{
								Name:  "top-n",
								Value: 20,
								Usage: "Number of top results to show (0 for all)",
							},
						}, uploadFlags()...),
						Action: func(ctx context.Context, cmd *cli.Command) error {
							setupLogging(cmd.String("log"))

							descendants, err := loadAndCountDescendants(cmd)
							if err != nil {
								return err
							}

							// Collect nodes with timestamps
							nodesWithTimestamps := workflowy.CollectNodesWithTimestamps(descendants)

							// Rank by modified date
							topN := cmd.Int("top-n")
							ranked := workflowy.RankByModified(nodesWithTimestamps, topN)

							// Check if we should upload
							report := &reports.ModifiedReportOutput{
								Ranked: ranked,
								TopN:   topN,
							}
							if err := handleReportUpload(cmd, report); err != nil {
								return err
							}
							if cmd.Bool("upload") {
								return nil
							}

							// Output results
							format := cmd.String("format")
							if format == "json" {
								printJSON(ranked)
							} else {
								fmt.Printf("# Oldest Nodes by Modification Date\n\n")
								for i, r := range ranked {
									fmt.Printf("%d. %s\n", i+1, r.String())
								}
							}

							return nil
						},
					},
				},
			},
			{
				Name:  "markdown",
				Usage: "Convert WorkFlowy item to markdown",
				Arguments: []cli.Argument{
					&cli.StringArg{
						Name:      "item_id",
						Value:     "None",
						UsageText: "WorkFlowy item ID (default: root)",
					},
				},
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "use-backup-file",
						Usage: "Use backup file instead of API (specify filename or leave empty for latest)",
					},
					&cli.StringFlag{
						Name:  "output",
						Usage: "Output file (default: stdout)",
					},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					setupLogging(cmd.String("log"))

					// Load tree
					items, err := loadTree(cmd)
					if err != nil {
						return err
					}

					// Find the item to convert
					itemID := cmd.StringArg("item_id")
					var targetItems []*workflowy.Item

					if itemID == "None" {
						// Use all root items
						targetItems = items
					} else {
						// Find specific item
						item := findItemByID(items, itemID)
						if item == nil {
							return fmt.Errorf("item with ID %s not found", itemID)
						}
						targetItems = []*workflowy.Item{item}
					}

					// Create formatter
					fmtr := formatter.NewDefaultFormatter()

					// Convert to markdown
					slog.Info("converting to markdown", "item_count", len(targetItems))
					markdown, err := fmtr.FormatTree(targetItems)
					if err != nil {
						return fmt.Errorf("error formatting markdown: %w", err)
					}

					// Output
					outputFile := cmd.String("output")
					if outputFile != "" {
						slog.Info("writing to file", "file", outputFile)
						err := os.WriteFile(outputFile, []byte(markdown), 0644)
						if err != nil {
							return fmt.Errorf("error writing file: %w", err)
						}
					} else {
						fmt.Print(markdown)
					}

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
		case *workflowy.ListChildrenResponse:
			v.Items = filterEmptyNames(v.Items)
		case *workflowy.CreateNodeResponse:
			// No filtering needed for create response
		}
	}

	// Sort items by priority before output
	switch v := data.(type) {
	case *workflowy.Item:
		if len(v.Children) > 0 {
			sortItemsByPriority(v.Children)
		}
	case *workflowy.ListChildrenResponse:
		sortItemsByPriority(v.Items)
	case *workflowy.CreateNodeResponse:
		// No sorting needed for create response
	}

	if format == "md" || format == "markdown" {
		switch v := data.(type) {
		case *workflowy.Item:
			fmt.Print(itemToMarkdown(v, 0))
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

// uploadFlags returns the standard upload flags for report commands
func uploadFlags() []cli.Flag {
	return []cli.Flag{
		&cli.BoolFlag{
			Name:  "upload",
			Usage: "Upload report to WorkFlowy instead of printing",
		},
		&cli.StringFlag{
			Name:  "parent-id",
			Value: "None",
			Usage: "Parent node ID for uploaded report (default: root)",
		},
		&cli.StringFlag{
			Name:  "position",
			Usage: "Position in parent: top or bottom",
		},
	}
}

// handleReportUpload handles uploading a report if --upload flag is set
func handleReportUpload(cmd *cli.Command, report reports.ReportOutput) error {
	if !cmd.Bool("upload") {
		return nil // Not uploading
	}

	client := createClient(cmd.String("api-key-file"))
	apiCtx := context.Background()

	opts := reports.UploadOptions{
		ParentID: cmd.String("parent-id"),
		Position: cmd.String("position"),
	}

	nodeID, err := reports.UploadReport(apiCtx, client, report, opts)
	if err != nil {
		return err
	}

	fmt.Printf("Report uploaded successfully!\n")
	fmt.Printf("URL: https://workflowy.com/#/%s\n", nodeID)
	return nil
}

// loadTree loads the tree from either backup file or API
func loadTree(cmd *cli.Command) ([]*workflowy.Item, error) {
	var items []*workflowy.Item
	var err error

	backupFile := cmd.String("use-backup-file")
	if backupFile != "" {
		// Use backup file
		if backupFile == "true" || backupFile == "1" {
			// Flag was set without value, use latest
			slog.Debug("using latest backup file")
			items, err = workflowy.ReadLatestBackup()
		} else {
			// Flag has a specific filename
			slog.Debug("using backup file", "file", backupFile)
			items, err = workflowy.ReadBackupFile(backupFile)
		}
		if err != nil {
			return nil, fmt.Errorf("error reading backup file: %w", err)
		}
	} else {
		// Use export API with cache
		client := createClient(cmd.String("api-key-file"))
		apiCtx := context.Background()

		slog.Debug("using export API with cache")
		response, err := client.ExportNodesWithCache(apiCtx, false)
		if err != nil {
			return nil, fmt.Errorf("error exporting nodes: %w", err)
		}

		slog.Debug("reconstructing tree from export data")
		root := workflowy.BuildTreeFromExport(response.Nodes)
		items = root.Children
	}

	return items, nil
}

// loadAndCountDescendants loads tree and counts descendants
func loadAndCountDescendants(cmd *cli.Command) (workflowy.Descendants, error) {
	items, err := loadTree(cmd)
	if err != nil {
		return nil, err
	}

	// Find the root item
	var rootItem *workflowy.Item
	itemID := cmd.String("item-id")
	if itemID == "None" && len(items) > 0 {
		// Create a virtual root containing all top-level items
		rootItem = &workflowy.Item{
			ID:       "root",
			Name:     "Root",
			Children: items,
		}
	} else {
		rootItem = findItemByID(items, itemID)
		if rootItem == nil {
			return nil, fmt.Errorf("item with ID %s not found", itemID)
		}
	}

	// Count descendants
	threshold := cmd.Float64("threshold")
	slog.Info("counting descendants", "threshold", threshold)
	return workflowy.CountDescendants(rootItem, threshold), nil
}

// findItemByID searches for an item by ID in the tree
func findItemByID(items []*workflowy.Item, id string) *workflowy.Item {
	for _, item := range items {
		if item.ID == id {
			return item
		}
		if found := findItemByID(item.Children, id); found != nil {
			return found
		}
	}
	return nil
}

// printCountTree prints the descendant count tree in markdown format
func printCountTree(node workflowy.Descendants, depth int) {
	indent := strings.Repeat("  ", depth)
	nodeValue := node.NodeValue()

	// Print current node with counts
	fmt.Printf("%s- %s (descendants: %d, children: %d, ratio: %.2f%%)\n",
		indent,
		(**nodeValue).String(),
		node.Count,
		node.ChildrenCount,
		node.RatioToRoot*100,
	)

	// Print children
	for child := range node.Children() {
		printCountTree(child.Node(), depth+1)
	}
}
