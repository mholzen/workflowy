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
	"regexp"
	"sort"
	"strings"

	"github.com/mholzen/workflowy/pkg/formatter"
	"github.com/mholzen/workflowy/pkg/reports"
	"github.com/mholzen/workflowy/pkg/workflowy"
	"github.com/urfave/cli/v3"
)

// Version information, set by goreleaser at build time
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// Custom help template with DESCRIPTION after COMMANDS
func main() {
	cmd := &cli.Command{
		Name:  "workflowy",
		Usage: "Interact with Workflowy API",
		Description: `Retieve, create and update nodes.  Generate usage reports and upload them to Workflowy.

Specify how to access the data using the --method flag:
  --method=get      Use GET API (default for depth 1-3)
  --method=export   Use Export API (default for depth 4+, --all)
  --method=backup   Use local backup file (fastest, offline)

Further customize the access method with the following flags:
  --api-key-file    Path to API key file (default: ~/.workflowy/api.key)
  --force-refresh   Bypass export cache (use with --method=export)
  --backup-file     Path to backup file (default: latest in ~/Dropbox/Apps/Workflowy/Data)

Examples:
  workflowy get --method=backup
  workflowy list --force-refresh
  workflowy report count --upload`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "format",
				Aliases: []string{"f"},
				Value:   "list",
				Usage:   "Output format: list, json, or markdown",
			},
			&cli.StringFlag{
				Name:  "log",
				Value: "info",
				Usage: "Log level: debug, info, warn, error",
			},
		},
		Before: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
			setupLogging(cmd.String("log"))
			return ctx, nil
		},
		Commands: []*cli.Command{
			{
				Name:      "get",
				Usage:     "Get item with optional recursive children (root if omitted)",
				Arguments: getFetchArguments,
				Flags:     getFetchFlags(),
				Action: func(ctx context.Context, cmd *cli.Command) error {
					params, err := getAndValidateFetchParams(cmd)
					if err != nil {
						return err
					}

					result, err := fetchItems(cmd, ctx, params.itemID, params.depth)
					if err != nil {
						return err
					}

					printOutput(result, params.format, cmd.Bool("include-empty-names"))
					return nil
				},
			},
			{
				Name:      "list",
				Usage:     "List descendants of an item as flat list (root if omitted)",
				Arguments: getFetchArguments,
				Flags:     getFetchFlags(),
				Action: func(ctx context.Context, cmd *cli.Command) error {
					params, err := getAndValidateFetchParams(cmd)
					if err != nil {
						return err
					}

					treeResult, err := fetchItems(cmd, ctx, params.itemID, params.depth)
					if err != nil {
						return err
					}

					flatList := flattenTree(treeResult)

					printOutput(flatList, params.format, cmd.Bool("include-empty-names"))
					return nil
				},
			},
			{
				Name:  "create",
				Usage: "Create a new node",
				Arguments: []cli.Argument{
					&cli.StringArg{
						Name:      "name",
						UsageText: "Node name (or use --read-stdin or --read-file)",
					},
				},
				Flags: getWriteFlags(
					&cli.StringFlag{
						Name:  "parent-id",
						Value: "None",
						Usage: "Parent node UUID, target key, or \"None\" for top-level",
					},
					&cli.StringFlag{
						Name:  "position",
						Usage: "Position: \"top\" or \"bottom\" (omit for API default)",
					},
					&cli.BoolFlag{
						Name:  "read-stdin",
						Usage: "Read node name from stdin instead of argument",
					},
					&cli.StringFlag{
						Name:  "read-file",
						Usage: "Read node name from file instead of argument",
					},
				),
				Action: func(ctx context.Context, cmd *cli.Command) error {
					format := cmd.String("format")
					if err := validateFormat(format); err != nil {
						return err
					}

					position := cmd.String("position")
					if err := validatePosition(position); err != nil {
						return err
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

					slog.Debug("creating node", "parent_id", req.ParentID, "name", name)
					response, err := client.CreateNode(ctx, req)
					if err != nil {
						return fmt.Errorf("error creating node: %w", err)
					}

					printOutput(response, format, cmd.Bool("include-empty-names"))
					return nil
				},
			},
			{
				Name:  "update",
				Usage: "Update an existing node",
				Arguments: []cli.Argument{
					&cli.StringArg{
						Name:      "item_id",
						UsageText: "<item_id>",
					},
					&cli.StringArg{
						Name:      "nameArgument",
						UsageText: "[<name>] (or use flags for specific fields)",
					},
				},
				Flags: getWriteFlags(),
				Action: func(ctx context.Context, cmd *cli.Command) error {
					format := cmd.String("format")
					if err := validateFormat(format); err != nil {
						return err
					}

					itemID := cmd.StringArg("item_id")
					if itemID == "" {
						return fmt.Errorf("item_id is required")
					}

					content := cmd.StringArg("nameArgument")
					nameFlag := cmd.String("name")
					noteFlag := cmd.String("note")
					layoutMode := cmd.String("layout-mode")

					// Build request - if content argument is provided, use it as name
					// Otherwise, use flags
					req := &workflowy.UpdateNodeRequest{}

					if content != "" && nameFlag != "" {
						return fmt.Errorf("cannot specify both content argument and --name flag")
					}

					if content != "" {
						req.Name = &content
					} else if nameFlag != "" {
						req.Name = &nameFlag
					}

					if noteFlag != "" {
						req.Note = &noteFlag
					}

					if layoutMode != "" {
						req.LayoutMode = &layoutMode
					}

					// Ensure at least one field is being updated
					if req.Name == nil && req.Note == nil && req.LayoutMode == nil {
						return fmt.Errorf("must specify at least one field to update (<name>, --name, --note, or --layout-mode)")
					}

					client := createClient(cmd.String("api-key-file"))

					slog.Debug("updating node", "item_id", itemID)
					response, err := client.UpdateNode(ctx, itemID, req)
					if err != nil {
						return fmt.Errorf("error updating node: %w", err)
					}

					printOutput(response, format, cmd.Bool("include-empty-names"))
					return nil
				},
			},
			{
				Name:  "report",
				Usage: "Generate reports from Workflowy data",
				Commands: []*cli.Command{
					{
						Name:  "count",
						Usage: "Generate descendant count report",
						Flags: getReportFlags(
							&cli.Float64Flag{
								Name:  "threshold",
								Value: 0.01,
								Usage: "Minimum ratio threshold for filtering (0.0 to 1.0)",
							},
						),
						Action: func(ctx context.Context, cmd *cli.Command) error {
							items, err := loadTree(ctx, cmd)
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
							if err := uploadReport(ctx, cmd, report); err != nil {
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
						Flags: getRankingReportFlags(),
						Action: func(ctx context.Context, cmd *cli.Command) error {
							descendants, err := loadAndCountDescendants(ctx, cmd)
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
							if err := uploadReport(ctx, cmd, report); err != nil {
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
						Flags: getRankingReportFlags(),
						Action: func(ctx context.Context, cmd *cli.Command) error {
							descendants, err := loadAndCountDescendants(ctx, cmd)
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
							if err := uploadReport(ctx, cmd, report); err != nil {
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
						Flags: getRankingReportFlags(),
						Action: func(ctx context.Context, cmd *cli.Command) error {
							descendants, err := loadAndCountDescendants(ctx, cmd)
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
							if err := uploadReport(ctx, cmd, report); err != nil {
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
			Name:  "search",
			Usage: "Search for items by name",
			Arguments: []cli.Argument{
				&cli.StringArg{
					Name:      "pattern",
					UsageText: "Search pattern (text or regex with -E)",
				},
			},
			Flags: append([]cli.Flag{
				&cli.BoolFlag{
					Name:    "ignore-case",
					Aliases: []string{"i"},
					Usage:   "Case-insensitive search",
				},
				&cli.BoolFlag{
					Name:    "regexp",
					Aliases: []string{"E"},
					Usage:   "Treat pattern as regular expression",
				},
				&cli.StringFlag{
					Name:  "item-id",
					Value: "None",
					Usage: "Search within specific subtree (default: root)",
				},
			}, getMethodFlags()...),
			Action: func(ctx context.Context, cmd *cli.Command) error {
				format := cmd.String("format")
				if err := validateFormat(format); err != nil {
					return err
				}

				pattern := cmd.StringArg("pattern")
				if pattern == "" {
					return fmt.Errorf("search pattern is required")
				}

				method := cmd.String("method")
				if method == "get" {
					slog.Warn("GET method not supported for search, switching to export")
					method = "export"
				}

				items, err := loadTree(ctx, cmd)
				if err != nil {
					return err
				}

				itemID := cmd.String("item-id")
				rootItem := findRootItem(items, itemID)
				if rootItem == nil && itemID != "None" {
					return fmt.Errorf("item not found: %s", itemID)
				}

				searchRoot := items
				if rootItem != nil {
					searchRoot = []*workflowy.Item{rootItem}
				}

				results := searchItems(
					searchRoot,
					pattern,
					cmd.Bool("regexp"),
					cmd.Bool("ignore-case"),
				)

				printOutput(results, format, false)
				return nil
			},
		},
			{
				Name:  "version",
				Usage: "Show version information",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					fmt.Printf("workflowy version %s\n", version)
					fmt.Printf("commit: %s\n", commit)
					fmt.Printf("built: %s\n", date)
					return nil
				},
			},
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}

func getMethodFlags() []cli.Flag {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Error getting home directory: %v", err)
	}
	defaultAPIKeyFile := filepath.Join(homeDir, ".workflowy", "api.key")

	return []cli.Flag{
		&cli.StringFlag{
			Name:  "method",
			Usage: "Access method: get, export or backup\n\tDefaults to 'get' for depth 1-3, 'export' for depth 4+, 'backup' if no api key provided",
		},
		&cli.StringFlag{
			Name:  "api-key-file",
			Value: defaultAPIKeyFile,
			Usage: "Path to API key file",
		},
		&cli.StringFlag{
			Name:  "backup-file",
			Usage: "Path to backup file (default: latest in ~/Dropbox/Apps/Workflowy/Data)",
		},
		&cli.BoolFlag{
			Name:  "force-refresh",
			Usage: "Force refresh from API when using export (bypassing cache)",
		},
	}
}

func getFetchFlags() []cli.Flag {
	flags := []cli.Flag{
		&cli.IntFlag{
			Name:    "depth",
			Aliases: []string{"d"},
			Value:   2,
			Usage:   "Recursion depth for get/list operations (positive integer)",
		},
		&cli.BoolFlag{
			Name:  "all",
			Usage: "Get/list all descendants (equivalent to --depth=-1)",
		},
		&cli.BoolFlag{
			Name:  "include-empty-names",
			Value: false,
			Usage: "Include items with empty names",
		},
	}
	flags = append(flags, getMethodFlags()...)
	return flags
}

func getWriteFlags(commandFlags ...cli.Flag) []cli.Flag {
	flags := []cli.Flag{
		&cli.StringFlag{
			Name:  "name",
			Usage: "Update node name/title",
		},
		&cli.StringFlag{
			Name:  "note",
			Usage: "Additional note content",
		},
		&cli.StringFlag{
			Name:  "layout-mode",
			Usage: "Display mode: bullets, todo, h1, h2, h3",
		},
	}
	flags = append(flags, commandFlags...)
	return flags
}

func getReportFlags(commandFlags ...cli.Flag) []cli.Flag {
	flags := make([]cli.Flag, 0)
	flags = append(flags, getMethodFlags()...)
	flags = append(flags, commandFlags...)

	// Starting point for all reports
	flags = append(flags, &cli.StringFlag{
		Name:  "item-id",
		Value: "None",
		Usage: "Item ID to start from (default: root)",
	})

	// Upload flags
	flags = append(flags,
		&cli.BoolFlag{
			Name:  "upload",
			Usage: "Upload report to Workflowy instead of printing",
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
	)

	return flags
}

func getRankingReportFlags() []cli.Flag {
	reportFlags := getReportFlags()
	reportFlags = append(reportFlags,
		&cli.StringFlag{
			Name:  "item-id",
			Value: "None",
			Usage: "Item ID to start from (default: root)",
		},
		&cli.IntFlag{
			Name:  "top-n",
			Value: 20,
			Usage: "Number of top results to show (0 for all)",
		},
	)
	return reportFlags
}

var getFetchArguments = []cli.Argument{
	&cli.StringArg{
		Name:      "item_id",
		Value:     "None",
		UsageText: "Workflowy item ID (default: root)",
	},
}

type FetchParameters struct {
	format string
	depth  int
	itemID string
}

func getAndValidateFetchParams(cmd *cli.Command) (FetchParameters, error) {
	format := cmd.String("format")
	if err := validateFormat(format); err != nil {
		return FetchParameters{}, err
	}

	depth := cmd.Int("depth")
	if cmd.Bool("all") {
		depth = -1
	}
	itemID := cmd.StringArg("item_id")
	return FetchParameters{format: format, depth: depth, itemID: itemID}, nil
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

// validateFormat validates the output format and returns an error if invalid
func validateFormat(format string) error {
	if format != "list" && format != "json" && format != "markdown" {
		return fmt.Errorf("format must be 'list', 'json', or 'markdown'")
	}
	return nil
}

// validatePosition validates the position parameter and returns an error if invalid
func validatePosition(position string) error {
	if position != "" && position != "top" && position != "bottom" {
		return fmt.Errorf("position must be 'top' or 'bottom'")
	}
	return nil
}

// fetchItems retrieves items using the smart access method selection
func fetchItems(cmd *cli.Command, apiCtx context.Context, itemID string, depth int) (interface{}, error) {
	client := createClient(cmd.String("api-key-file"))

	// Determine access method
	method := cmd.String("method")
	backupFile := cmd.String("backup-file")

	// Validate method flag
	if method != "" && method != "get" && method != "export" && method != "backup" {
		return nil, fmt.Errorf("method must be 'get', 'export', or 'backup'")
	}

	// Determine which method to use
	var useMethod string
	if method != "" {
		// Explicitly specified
		useMethod = method
	} else {
		// Smart heuristic: depth 1-3 uses GET API, depth -1 or 4+ uses Export API
		if depth == -1 || depth >= 4 {
			useMethod = "export"
		} else {
			useMethod = "get"
		}
	}

	slog.Debug("access method determined", "method", useMethod, "depth", depth)

	var result interface{}

	// Method 1: Backup file
	switch useMethod {
	case "backup":
		var items []*workflowy.Item
		var err error

		if backupFile != "" {
			slog.Debug("using backup file", "file", backupFile)
			items, err = workflowy.ReadBackupFile(backupFile)
		} else {
			slog.Debug("using latest backup file")
			items, err = workflowy.ReadLatestBackup()
		}
		if err != nil {
			return nil, fmt.Errorf("error reading backup file: %w", err)
		}

		if itemID != "None" {
			found := findItemInTree(items, itemID, depth)
			if found == nil {
				return nil, fmt.Errorf("item %s not found in backup", itemID)
			}
			result = found
		} else {
			result = &workflowy.ListChildrenResponse{Items: items}
		}

	case "export":
		slog.Debug("using export API", "depth", depth)
		forceRefresh := cmd.Bool("force-refresh")
		response, err := client.ExportNodesWithCache(apiCtx, forceRefresh)
		if err != nil {
			return nil, fmt.Errorf("error exporting nodes: %w", err)
		}

		slog.Debug("reconstructing tree from export data")
		root := workflowy.BuildTreeFromExport(response.Nodes)

		if itemID != "None" {
			found := findItemInTree(root.Children, itemID, depth)
			if found == nil {
				return nil, fmt.Errorf("item %s not found", itemID)
			}
			result = found
		} else {
			result = &workflowy.ListChildrenResponse{Items: root.Children}
		}

	case "get":
		slog.Debug("using GET API", "depth", depth)
		if depth < 0 {
			return nil, fmt.Errorf("depth must be non-negative when using GET API (use --method=export for depth=-1)")
		}

		if itemID == "None" {
			slog.Debug("fetching root items", "depth", depth)
			response, err := client.ListChildrenRecursiveWithDepth(apiCtx, itemID, depth)
			if err != nil {
				return nil, fmt.Errorf("error fetching root items: %w", err)
			}
			result = response
		} else {
			slog.Debug("fetching item", "item_id", itemID, "depth", depth)
			item, err := client.GetItem(apiCtx, itemID)
			if err != nil {
				return nil, fmt.Errorf("error getting item: %w", err)
			}

			if depth > 0 {
				childrenResp, err := client.ListChildrenRecursiveWithDepth(apiCtx, itemID, depth)
				if err != nil {
					return nil, fmt.Errorf("error fetching children: %w", err)
				}
				item.Children = childrenResp.Items
			}
			result = item
		}

	default:
		return nil, fmt.Errorf("unknown access method: %s", useMethod)
	}

	return result, nil
}

func findItemInTree(items []*workflowy.Item, targetID string, maxDepth int) *workflowy.Item {
	for _, item := range items {
		if item.ID == targetID {
			// Found the item, now limit its depth if needed
			if maxDepth >= 0 {
				limitItemDepth(item, maxDepth)
			}
			return item
		}
		// Recursively search children
		if found := findItemInTree(item.Children, targetID, maxDepth); found != nil {
			return found
		}
	}
	return nil
}

func limitItemDepth(item *workflowy.Item, maxDepth int) {
	if maxDepth == 0 {
		// No children allowed at this depth
		item.Children = nil
		return
	}
	// Recursively limit children
	for _, child := range item.Children {
		limitItemDepth(child, maxDepth-1)
	}
}

func flattenTree(data interface{}) *workflowy.ListChildrenResponse {
	var items []*workflowy.Item

	switch v := data.(type) {
	case *workflowy.Item:
		// Single item - flatten it and its children
		items = flattenItem(v)
	case *workflowy.ListChildrenResponse:
		// List of items - flatten each
		for _, item := range v.Items {
			items = append(items, flattenItem(item)...)
		}
	}

	return &workflowy.ListChildrenResponse{Items: items}
}

func flattenItem(item *workflowy.Item) []*workflowy.Item {
	result := []*workflowy.Item{item}

	// Add all descendants
	for _, child := range item.Children {
		result = append(result, flattenItem(child)...)
	}

	// Remove children since the output is flattened
	item.Children = nil
	return result
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
				log.Fatalf("Error formatting markdown: %v", err)
			}
			fmt.Print(output)
		case *workflowy.ListChildrenResponse:
			output, err := formatter.FormatItemsAsMarkdown(v.Items)
			if err != nil {
				log.Fatalf("Error formatting markdown: %v", err)
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

func uploadReport(ctx context.Context, cmd *cli.Command, report reports.ReportOutput) error {
	if !cmd.Bool("upload") {
		return nil // Not uploading
	}

	client := createClient(cmd.String("api-key-file"))

	opts := reports.UploadOptions{
		ParentID: cmd.String("parent-id"),
		Position: cmd.String("position"),
	}

	nodeID, err := reports.UploadReport(ctx, client, report, opts)
	if err != nil {
		return err
	}

	fmt.Printf("Report uploaded successfully!\n")
	fmt.Printf("URL: https://workflowy.com/#/%s\n", nodeID)
	return nil
}

func loadTree(ctx context.Context, cmd *cli.Command) ([]*workflowy.Item, error) {
	var items []*workflowy.Item
	var err error

	// Determine access method
	method := cmd.String("method")
	backupFile := cmd.String("backup-file")

	// Validate method flag
	if method != "" && method != "export" && method != "backup" {
		return nil, fmt.Errorf("method must be 'export' or 'backup'")
	}

	// Determine which method to use (default to export for reports)
	useMethod := method
	if useMethod == "" {
		useMethod = "export"
	}

	slog.Debug("loadTree access method", "method", useMethod)

	// Method 1: Backup file
	if useMethod == "backup" {
		if backupFile != "" {
			slog.Debug("using backup file", "file", backupFile)
			items, err = workflowy.ReadBackupFile(backupFile)
		} else {
			slog.Debug("using latest backup file")
			items, err = workflowy.ReadLatestBackup()
		}
		if err != nil {
			return nil, fmt.Errorf("error reading backup file: %w", err)
		}

		// Method 2: Export API
	} else {
		client := createClient(cmd.String("api-key-file"))
		forceRefresh := cmd.Bool("force-refresh")

		slog.Debug("using export API", "force_refresh", forceRefresh)
		response, err := client.ExportNodesWithCache(ctx, forceRefresh)
		if err != nil {
			return nil, fmt.Errorf("error exporting nodes: %w", err)
		}

		slog.Debug("reconstructing tree from export data")
		root := workflowy.BuildTreeFromExport(response.Nodes)
		items = root.Children
	}

	return items, nil
}

func loadAndCountDescendants(ctx context.Context, cmd *cli.Command) (workflowy.Descendants, error) {
	items, err := loadTree(ctx, cmd)
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

func printCountTree(node workflowy.Descendants, depth int) {
	indent := strings.Repeat("  ", depth)
	nodeValue := node.NodeValue()

	fmt.Printf("%s- %s (%.1f%%, %d descendants)\n",
		indent,
		(**nodeValue).String(),
		node.RatioToRoot*100,
		node.Count,
	)

	for child := range node.Children() {
		printCountTree(child.Node(), depth+1)
	}
}

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
