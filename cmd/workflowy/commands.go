package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/mholzen/workflowy/pkg/reports"
	"github.com/mholzen/workflowy/pkg/workflowy"
	"github.com/urfave/cli/v3"
)

func getCommands() []*cli.Command {
	return []*cli.Command{
		getGetCommand(),
		getListCommand(),
		getCreateCommand(),
		getUpdateCommand(),
		getCompleteCommand(),
		getUncompleteCommand(),
		getReportCommand(),
		getSearchCommand(),
		getVersionCommand(),
	}
}

func getGetCommand() *cli.Command {
	return &cli.Command{
		Name:      "get",
		Usage:     "Get item with optional recursive children (root if omitted)",
		Arguments: getFetchArguments(),
		Flags:     getFetchFlags(),
		Action: withOptionalClient(func(ctx context.Context, cmd *cli.Command, client workflowy.Client) error {
			params, err := getAndValidateFetchParams(cmd)
			if err != nil {
				return err
			}

			result, err := fetchItems(cmd, ctx, client, params.itemID, params.depth)
			if err != nil {
				return err
			}

			printOutput(result, params.format, cmd.Bool("include-empty-names"))
			return nil
		}),
	}
}

func getListCommand() *cli.Command {
	return &cli.Command{
		Name:      "list",
		Usage:     "List descendants of an item as flat list (root if omitted)",
		Arguments: getFetchArguments(),
		Flags:     getFetchFlags(),
		Action: withOptionalClient(func(ctx context.Context, cmd *cli.Command, client workflowy.Client) error {
			params, err := getAndValidateFetchParams(cmd)
			if err != nil {
				return err
			}

			treeResult, err := fetchItems(cmd, ctx, client, params.itemID, params.depth)
			if err != nil {
				return err
			}

			flatList := flattenTree(treeResult)

			printOutput(flatList, params.format, cmd.Bool("include-empty-names"))
			return nil
		}),
	}
}

func getCreateCommand() *cli.Command {
	return &cli.Command{
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
		Action: withClient(func(ctx context.Context, cmd *cli.Command, client workflowy.Client) error {
			format := cmd.String("format")
			if err := validateFormat(format); err != nil {
				return err
			}

			position := cmd.String("position")
			if err := validatePosition(position); err != nil {
				return err
			}

			nameArg := cmd.StringArg("name")
			readStdin := cmd.Bool("read-stdin")
			readFile := cmd.String("read-file")

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
					return fmt.Errorf("cannot read stdin: %w", err)
				}
				name = strings.TrimSpace(string(stdinBytes))
			} else if readFile != "" {
				slog.Debug("reading from file", "file", readFile)
				fileBytes, err := os.ReadFile(readFile)
				if err != nil {
					return fmt.Errorf("cannot read file: %w", err)
				}
				name = strings.TrimSpace(string(fileBytes))
			}

			if name == "" {
				return fmt.Errorf("name cannot be empty")
			}

			req := &workflowy.CreateNodeRequest{
				ParentID: cmd.String("parent-id"),
				Name:     name,
			}

			if position != "" {
				req.Position = &position
			}
			if layoutMode := cmd.String("layout-mode"); layoutMode != "" {
				req.LayoutMode = &layoutMode
			}
			if note := cmd.String("note"); note != "" {
				req.Note = &note
			}

			slog.Debug("creating node", "parent_id", req.ParentID, "name", name)
			response, err := client.CreateNode(ctx, req)
			if err != nil {
				return fmt.Errorf("cannot create node: %w", err)
			}

			printOutput(response, format, cmd.Bool("include-empty-names"))
			return nil
		}),
	}
}

func getUpdateCommand() *cli.Command {
	return &cli.Command{
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
		Action: withClient(func(ctx context.Context, cmd *cli.Command, client workflowy.Client) error {
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

			if req.Name == nil && req.Note == nil && req.LayoutMode == nil {
				return fmt.Errorf("must specify at least one field to update (<name>, --name, --note, or --layout-mode)")
			}

			slog.Debug("updating node", "item_id", itemID)
			response, err := client.UpdateNode(ctx, itemID, req)
			if err != nil {
				return fmt.Errorf("cannot update node: %w", err)
			}

			printOutput(response, format, cmd.Bool("include-empty-names"))
			return nil
		}),
	}
}

func getCompleteCommand() *cli.Command {
	return getCompletionCommand("complete", "Mark a node as complete", "completing")
}

func getUncompleteCommand() *cli.Command {
	return getCompletionCommand("uncomplete", "Mark a node as uncomplete", "uncompleting")
}

func getCompletionCommand(commandName, usage, action string) *cli.Command {
	return &cli.Command{
		Name:  commandName,
		Usage: usage,
		Arguments: []cli.Argument{
			&cli.StringArg{
				Name:      "item_id",
				UsageText: "<item_id>",
			},
		},
		Flags: getMethodFlags(),
		Action: withClient(func(ctx context.Context, cmd *cli.Command, client workflowy.Client) error {
			format := cmd.String("format")
			if err := validateFormat(format); err != nil {
				return err
			}

			itemID := cmd.StringArg("item_id")
			if itemID == "" {
				return fmt.Errorf("item_id is required")
			}

			slog.Debug(action+" node", "item_id", itemID)

			var response *workflowy.UpdateNodeResponse
			var err error

			if commandName == "complete" {
				response, err = client.CompleteNode(ctx, itemID)
			} else {
				response, err = client.UncompleteNode(ctx, itemID)
			}

			if err != nil {
				return fmt.Errorf("cannot %s node: %w", commandName, err)
			}

			if format == "json" {
				printJSON(response)
			} else {
				fmt.Printf("%s %sd\n", itemID, commandName)
			}
			return nil
		}),
	}
}

func getReportCommand() *cli.Command {
	return &cli.Command{
		Name:  "report",
		Usage: "Generate reports from Workflowy data",
		Commands: []*cli.Command{
			getCountReportCommand(),
			getChildrenReportCommand(),
			getCreatedReportCommand(),
			getModifiedReportCommand(),
		},
	}
}

func getCountReportCommand() *cli.Command {
	return getCountReportCommandWithDeps(DefaultReportDeps(), withOptionalClient)
}

func getCountReportCommandWithDeps(deps ReportDeps, clientProvider ClientProvider) *cli.Command {
	return &cli.Command{
		Name:  "count",
		Usage: "Generate descendant count report",
		Flags: getReportFlags(
			&cli.Float64Flag{
				Name:  "threshold",
				Value: 0.01,
				Usage: "Minimum ratio threshold for filtering (0.0 to 1.0)",
			},
		),
		Action: clientProvider(countReportAction(deps)),
	}
}

func getChildrenReportCommand() *cli.Command {
	return &cli.Command{
		Name:  "children",
		Usage: "Rank nodes by immediate children count",
		Flags: getRankingReportFlags(),
		Action: withOptionalClient(func(ctx context.Context, cmd *cli.Command, client workflowy.Client) error {
			descendants, err := loadAndCountDescendants(ctx, cmd, client)
			if err != nil {
				return err
			}

			nodesWithTimestamps := workflowy.CollectNodesWithTimestamps(descendants)

			topN := cmd.Int("top-n")
			ranked := workflowy.RankByChildrenCount(nodesWithTimestamps, topN)

			report := &reports.ChildrenCountReportOutput{
				Ranked: ranked,
				TopN:   topN,
			}

			return outputReport(ctx, cmd, client, report, os.Stdout)
		}),
	}
}

func getCreatedReportCommand() *cli.Command {
	return &cli.Command{
		Name:  "created",
		Usage: "Rank nodes by creation date (oldest first)",
		Flags: getRankingReportFlags(),
		Action: withOptionalClient(func(ctx context.Context, cmd *cli.Command, client workflowy.Client) error {
			descendants, err := loadAndCountDescendants(ctx, cmd, client)
			if err != nil {
				return err
			}

			nodesWithTimestamps := workflowy.CollectNodesWithTimestamps(descendants)

			topN := cmd.Int("top-n")
			ranked := workflowy.RankByCreated(nodesWithTimestamps, topN)

			report := &reports.CreatedReportOutput{
				Ranked: ranked,
				TopN:   topN,
			}

			return outputReport(ctx, cmd, client, report, os.Stdout)
		}),
	}
}

func getModifiedReportCommand() *cli.Command {
	return &cli.Command{
		Name:  "modified",
		Usage: "Rank nodes by modification date (oldest first)",
		Flags: getRankingReportFlags(),
		Action: withOptionalClient(func(ctx context.Context, cmd *cli.Command, client workflowy.Client) error {
			descendants, err := loadAndCountDescendants(ctx, cmd, client)
			if err != nil {
				return err
			}

			nodesWithTimestamps := workflowy.CollectNodesWithTimestamps(descendants)

			topN := cmd.Int("top-n")
			ranked := workflowy.RankByModified(nodesWithTimestamps, topN)

			report := &reports.ModifiedReportOutput{
				Ranked: ranked,
				TopN:   topN,
			}

			return outputReport(ctx, cmd, client, report, os.Stdout)
		}),
	}
}

func getSearchCommand() *cli.Command {
	return &cli.Command{
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
		Action: withOptionalClient(func(ctx context.Context, cmd *cli.Command, client workflowy.Client) error {
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
				return fmt.Errorf("cannot search using the GET method")
			}

			items, err := loadTree(ctx, cmd, client)
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
		}),
	}
}

func getVersionCommand() *cli.Command {
	return &cli.Command{
		Name:  "version",
		Usage: "Show version information",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			fmt.Printf("workflowy version %s\n", version)
			fmt.Printf("commit: %s\n", commit)
			fmt.Printf("built: %s\n", date)
			return nil
		},
	}
}

func createClient(apiKeyFile string) (*workflowy.WorkflowyClient, error) {
	option, err := workflowy.WithAPIKeyFromFile(apiKeyFile)
	if err != nil {
		return nil, err
	}
	return workflowy.NewWorkflowyClient(option), nil
}

type ClientActionFunc func(ctx context.Context, cmd *cli.Command, client workflowy.Client) error

type ClientProvider func(ClientActionFunc) cli.ActionFunc

func withClient(fn ClientActionFunc) cli.ActionFunc {
	return func(ctx context.Context, cmd *cli.Command) error {
		client, err := createClient(cmd.String("api-key-file"))
		if err != nil {
			return err
		}
		return fn(ctx, cmd, client)
	}
}

func withOptionalClient(fn ClientActionFunc) cli.ActionFunc {
	return func(ctx context.Context, cmd *cli.Command) error {
		client, err := createClient(cmd.String("api-key-file"))
		if err != nil {
			slog.Warn("cannot create API client -- using backup method", "error", err)
			return fn(ctx, cmd, nil)
		}
		return fn(ctx, cmd, client)
	}
}
