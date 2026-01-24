package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"regexp"
	"strings"

	"github.com/mholzen/workflowy/pkg/mcp"
	"github.com/mholzen/workflowy/pkg/mirror"
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
		getMoveCommand(),
		getDeleteCommand(),
		getCompleteCommand(),
		getUncompleteCommand(),
		getTargetsCommand(),
		getReportCommand(),
		getSearchCommand(),
		getReplaceCommand(),
		getTransformCommand(),
		getIDCommand(),
		getMcpCommand(),
		getServeCommand(),
		getVersionCommand(),
	}
}

func getGetCommand() *cli.Command {
	return &cli.Command{
		Name:      "get",
		Usage:     "Get node and descendants",
		UsageText: "workflowy get [<id>] [options]",
		Arguments: getFetchArguments(),
		Flags:     getFetchFlags(),
		Action: withOptionalClient(func(ctx context.Context, cmd *cli.Command, client workflowy.Client) error {
			params, err := getAndValidateFetchParams(cmd)
			if err != nil {
				return err
			}

			itemID, err := workflowy.ResolveNodeID(ctx, client, params.itemID)
			if err != nil {
				return fmt.Errorf("cannot resolve ID: %w", err)
			}

			result, err := fetchItems(cmd, ctx, client, itemID, params.depth)
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
		Usage:     "List descendants as flat list",
		UsageText: "workflowy list [<id>] [options]",
		Arguments: getFetchArguments(),
		Flags:     getFetchFlags(),
		Action: withOptionalClient(func(ctx context.Context, cmd *cli.Command, client workflowy.Client) error {
			params, err := getAndValidateFetchParams(cmd)
			if err != nil {
				return err
			}

			itemID, err := workflowy.ResolveNodeID(ctx, client, params.itemID)
			if err != nil {
				return fmt.Errorf("cannot resolve ID: %w", err)
			}

			treeResult, err := fetchItems(cmd, ctx, client, itemID, params.depth)
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
		Name:      "create",
		Usage:     "Create a new node",
		UsageText: "workflowy create [options] <name>",
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
				Usage: "Parent ID: UUID or target key (default: root)",
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

			// Initialize write guard for access control
			guard, err := NewWriteGuard(ctx, client, getWriteRootID(cmd))
			if err != nil {
				return err
			}

			nameArg := cmd.StringArg("name")
			nameFlag := cmd.String("name")
			readStdin := cmd.Bool("read-stdin")
			readFile := cmd.String("read-file")

			inputSources := 0
			if nameArg != "" {
				inputSources++
			}
			if nameFlag != "" {
				inputSources++
			}
			if readStdin {
				inputSources++
			}
			if readFile != "" {
				inputSources++
			}

			if inputSources == 0 {
				return fmt.Errorf("must provide node name via argument, --name, --read-stdin, or --read-file")
			}
			if inputSources > 1 {
				return fmt.Errorf("cannot use multiple input sources (choose one: argument, --name, --read-stdin, or --read-file)")
			}

			var name string

			if nameArg != "" {
				name = nameArg
				slog.Debug("using name from argument", "name", name)
			} else if nameFlag != "" {
				name = nameFlag
				slog.Debug("using name from flag", "name", name)
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

			// Default parent to write-root-id if not specified and restrictions are in effect
			rawParentID := guard.DefaultParent(getParentID(cmd))

			parentID, err := workflowy.ResolveNodeID(ctx, client, rawParentID)
			if err != nil {
				return fmt.Errorf("cannot resolve parent ID: %w", err)
			}

			// Validate parent is within write-root scope
			if err := guard.ValidateParent(parentID, "create"); err != nil {
				return err
			}

			req := &workflowy.CreateNodeRequest{
				ParentID: parentID,
				Name:     name,
			}
			if err := req.SetPosition(cmd.String("position")); err != nil {
				return err
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

			if format == "json" {
				printJSON(response)
			} else {
				fmt.Printf("%s created\n", response.ItemID)
			}
			return nil
		}),
	}
}

func getUpdateCommand() *cli.Command {
	return &cli.Command{
		Name:      "update",
		Usage:     "Update an existing node",
		UsageText: "workflowy update <id> [<name>] [options]",
		Arguments: []cli.Argument{
			&cli.StringArg{
				Name:      "id",
				UsageText: "<id>",
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

			// Initialize write guard for access control
			guard, err := NewWriteGuard(ctx, client, getWriteRootID(cmd))
			if err != nil {
				return err
			}

			rawItemID := cmd.StringArg("id")
			if rawItemID == "" {
				return fmt.Errorf("id is required")
			}

			itemID, err := workflowy.ResolveNodeID(ctx, client, rawItemID)
			if err != nil {
				return fmt.Errorf("cannot resolve ID: %w", err)
			}

			// Validate target is within write-root scope
			if err := guard.ValidateTarget(itemID, "update"); err != nil {
				return err
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

			if format == "json" {
				printJSON(response)
			} else {
				fmt.Printf("%s updated\n", itemID)
			}
			return nil
		}),
	}
}

func getMoveCommand() *cli.Command {
	return &cli.Command{
		Name:      "move",
		Usage:     "Move a node to a new parent",
		UsageText: "workflowy move <id> <parent-id> [options]",
		Arguments: []cli.Argument{
			&cli.StringArg{
				Name:      "id",
				UsageText: "<id>",
			},
			&cli.StringArg{
				Name:      "parent_id",
				UsageText: "<parent-id>",
			},
		},
		Flags: []cli.Flag{
			getAPIKeyFlag(),
			&cli.StringFlag{
				Name:  "position",
				Usage: "Position in new parent: top or bottom (default: top)",
			},
		},
		Action: withClient(func(ctx context.Context, cmd *cli.Command, client workflowy.Client) error {
			format := cmd.String("format")
			if err := validateFormat(format); err != nil {
				return err
			}

			// Initialize write guard for access control
			guard, err := NewWriteGuard(ctx, client, getWriteRootID(cmd))
			if err != nil {
				return err
			}

			rawItemID := cmd.StringArg("id")
			if rawItemID == "" {
				return fmt.Errorf("id is required")
			}

			rawParentID := cmd.StringArg("parent_id")
			if rawParentID == "" {
				return fmt.Errorf("parent-id is required")
			}

			itemID, err := workflowy.ResolveNodeID(ctx, client, rawItemID)
			if err != nil {
				return fmt.Errorf("cannot resolve ID: %w", err)
			}

			parentID, err := workflowy.ResolveNodeID(ctx, client, rawParentID)
			if err != nil {
				return fmt.Errorf("cannot resolve parent ID: %w", err)
			}

			// Validate both target and destination are within write-root scope
			if err := guard.ValidateTarget(itemID, "move"); err != nil {
				return err
			}
			if err := guard.ValidateParent(parentID, "move destination"); err != nil {
				return err
			}

			req := &workflowy.MoveNodeRequest{
				ParentID: parentID,
			}
			if err := req.SetPosition(cmd.String("position")); err != nil {
				return err
			}

			slog.Debug("moving node", "item_id", itemID, "parent_id", parentID)
			response, err := client.MoveNode(ctx, itemID, req)
			if err != nil {
				return fmt.Errorf("cannot move node: %w", err)
			}

			if format == "json" {
				printJSON(response)
			} else {
				fmt.Printf("%s moved to %s\n", itemID, parentID)
			}
			return nil
		}),
	}
}

func getDeleteCommand() *cli.Command {
	return &cli.Command{
		Name:      "delete",
		Usage:     "Permanently delete a node",
		UsageText: "workflowy delete <id> [options]",
		Arguments: []cli.Argument{
			&cli.StringArg{
				Name:      "id",
				UsageText: "<id>",
			},
		},
		Flags: getMethodFlags(),
		Action: withClient(func(ctx context.Context, cmd *cli.Command, client workflowy.Client) error {
			format := cmd.String("format")
			if err := validateFormat(format); err != nil {
				return err
			}

			// Initialize write guard for access control
			guard, err := NewWriteGuard(ctx, client, getWriteRootID(cmd))
			if err != nil {
				return err
			}

			rawItemID := cmd.StringArg("id")
			if rawItemID == "" {
				return fmt.Errorf("id is required")
			}

			itemID, err := workflowy.ResolveNodeID(ctx, client, rawItemID)
			if err != nil {
				return fmt.Errorf("cannot resolve ID: %w", err)
			}

			// Validate target is within write-root scope
			if err := guard.ValidateTarget(itemID, "delete"); err != nil {
				return err
			}

			slog.Debug("deleting node", "item_id", itemID)

			response, err := client.DeleteNode(ctx, itemID)
			if err != nil {
				return fmt.Errorf("cannot delete node: %w", err)
			}

			if format == "json" {
				printJSON(response)
			} else {
				fmt.Printf("%s deleted\n", itemID)
			}
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

func getTargetsCommand() *cli.Command {
	return &cli.Command{
		Name:      "targets",
		Usage:     "List all available targets (shortcuts and system targets)",
		UsageText: "workflowy targets [options]",
		Flags:     getMethodFlags(),
		Action: withClient(func(ctx context.Context, cmd *cli.Command, client workflowy.Client) error {
			format := cmd.String("format")
			if err := validateFormat(format); err != nil {
				return err
			}

			slog.Debug("listing targets")
			response, err := client.ListTargets(ctx)
			if err != nil {
				return fmt.Errorf("cannot list targets: %w", err)
			}

			printOutput(response.Targets, format, false)
			return nil
		}),
	}
}

func getCompletionCommand(commandName, usage, action string) *cli.Command {
	return &cli.Command{
		Name:      commandName,
		Usage:     usage,
		UsageText: "workflowy " + commandName + " <id> [options]",
		Arguments: []cli.Argument{
			&cli.StringArg{
				Name:      "id",
				UsageText: "<id>",
			},
		},
		Flags: getMethodFlags(),
		Action: withClient(func(ctx context.Context, cmd *cli.Command, client workflowy.Client) error {
			format := cmd.String("format")
			if err := validateFormat(format); err != nil {
				return err
			}

			// Initialize write guard for access control
			guard, err := NewWriteGuard(ctx, client, getWriteRootID(cmd))
			if err != nil {
				return err
			}

			rawItemID := cmd.StringArg("id")
			if rawItemID == "" {
				return fmt.Errorf("id is required")
			}

			itemID, err := workflowy.ResolveNodeID(ctx, client, rawItemID)
			if err != nil {
				return fmt.Errorf("cannot resolve ID: %w", err)
			}

			// Validate target is within write-root scope
			if err := guard.ValidateTarget(itemID, commandName); err != nil {
				return err
			}

			slog.Debug(action+" node", "item_id", itemID)

			var response *workflowy.UpdateNodeResponse

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
		Name:      "report",
		Usage:     "Generate reports from Workflowy data",
		UsageText: "workflowy report <subcommand> [options]",
		Commands: []*cli.Command{
			getCountReportCommand(),
			getChildrenReportCommand(),
			getCreatedReportCommand(),
			getModifiedReportCommand(),
			getMirrorReportCommand(),
		},
	}
}

func getCountReportCommand() *cli.Command {
	return getCountReportCommandWithDeps(DefaultReportDeps(), withOptionalClient)
}

func getCountReportCommandWithDeps(deps ReportDeps, clientProvider ClientProvider) *cli.Command {
	return &cli.Command{
		Name:      "count",
		Usage:     "Generate descendant count report",
		UsageText: "workflowy report count [options]",
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
		Name:      "children",
		Usage:     "Rank nodes by immediate children count",
		UsageText: "workflowy report children [options]",
		Flags:     getRankingReportFlags(),
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
		Name:      "created",
		Usage:     "Rank nodes by creation date (oldest first)",
		UsageText: "workflowy report created [options]",
		Flags:     getRankingReportFlags(),
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
		Name:      "modified",
		Usage:     "Rank nodes by modification date (oldest first)",
		UsageText: "workflowy report modified [options]",
		Flags:     getRankingReportFlags(),
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

func getMirrorReportCommand() *cli.Command {
	return &cli.Command{
		Name:      "mirrors",
		Usage:     "Rank nodes by mirror count (most mirrored first)",
		UsageText: "workflowy report mirrors [options]",
		Flags:     getMirrorReportFlags(),
		Action: withOptionalClient(func(ctx context.Context, cmd *cli.Command, client workflowy.Client) error {
			method := cmd.String("method")
			if method != "" && method != "backup" {
				return fmt.Errorf("mirror report requires --method=backup (mirror data is only available in backup files)")
			}

			items, err := loadTree(ctx, cmd, client)
			if err != nil {
				return err
			}

			infos := mirror.CollectMirrorInfos(items)
			topN := cmd.Int("top-n")
			ranked := mirror.RankByMirrorCount(infos, topN)

			report := &reports.MirrorCountReportOutput{
				Ranked: ranked,
				TopN:   topN,
			}

			return outputReport(ctx, cmd, client, report, os.Stdout)
		}),
	}
}

func getSearchCommand() *cli.Command {
	return &cli.Command{
		Name:      "search",
		Usage:     "Search for nodes by name",
		UsageText: "workflowy search <pattern> [options]",
		Arguments: []cli.Argument{
			&cli.StringArg{
				Name:      "pattern",
				UsageText: "Search pattern (text or regex with -E)",
			},
		},
		Flags: append(getSearchFlags(), getMethodFlags()...),
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

			itemID, err := workflowy.ResolveNodeID(ctx, client, getID(cmd))
			if err != nil {
				return fmt.Errorf("cannot resolve ID: %w", err)
			}
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

func getReplaceCommand() *cli.Command {
	return &cli.Command{
		Name:      "replace",
		Usage:     "Search and replace text in node names using regular expressions",
		UsageText: "workflowy replace <pattern> <substitution> [options]",
		Description: `Search for a pattern and replace matches in node names.

The pattern is a regular expression. The substitution string supports group
references using Go's syntax:
  - $1, $2, ... $9 for numbered groups (when not followed by digits/letters)
  - ${1}, ${2}, etc. for numbered groups (always safe, use when followed by alphanumerics)
  - ${name} for named groups
  - $$ for a literal dollar sign

Examples:
  # Simple text replacement
  workflowy replace "old-text" "new-text"

  # Case-insensitive replacement
  workflowy replace -i "todo" "TODO"

  # Using capture groups (note: use ${N} when followed by alphanumerics)
  workflowy replace "(\w+)-(\d+)" "${2}_${1}"   # "task-123" → "123_task"
  workflowy replace "(\w+) (\w+)" "$2 $1"       # "hello world" → "world hello"

  # Preview changes without applying
  workflowy replace --dry-run "pattern" "replacement"

  # Interactive mode - confirm each replacement
  workflowy replace --interactive "pattern" "replacement"

  # Limit to a specific subtree
  workflowy replace --parent-id=1a2b3c --depth=3 "pattern" "replacement"`,
		Arguments: []cli.Argument{
			&cli.StringArg{
				Name:      "pattern",
				UsageText: "Regular expression pattern to match",
			},
			&cli.StringArg{
				Name:      "substitution",
				UsageText: "Replacement string (supports $1, $2, etc. for groups)",
			},
		},
		Flags: getReplaceFlags(),
		Action: withClient(func(ctx context.Context, cmd *cli.Command, client workflowy.Client) error {
			format := cmd.String("format")
			if err := validateFormat(format); err != nil {
				return err
			}

			// Initialize write guard for access control
			guard, err := NewWriteGuard(ctx, client, getWriteRootID(cmd))
			if err != nil {
				return err
			}

			pattern := cmd.StringArg("pattern")
			if pattern == "" {
				return fmt.Errorf("pattern is required")
			}

			substitution := cmd.StringArg("substitution")

			if cmd.Bool("ignore-case") {
				pattern = "(?i)" + pattern
			}

			re, err := regexp.Compile(pattern)
			if err != nil {
				return fmt.Errorf("invalid regular expression: %w", err)
			}

			items, err := loadTree(ctx, cmd, client)
			if err != nil {
				return err
			}

			parentID, err := workflowy.ResolveNodeID(ctx, client, getParentID(cmd))
			if err != nil {
				return fmt.Errorf("cannot resolve parent ID: %w", err)
			}

			// Validate parent is within write-root scope
			if err := guard.ValidateParent(parentID, "replace"); err != nil {
				return err
			}

			searchRoot := items
			if parentID != "None" {
				rootItem := findItemByID(items, parentID)
				if rootItem == nil {
					return fmt.Errorf("parent item not found: %s", parentID)
				}
				searchRoot = []*workflowy.Item{rootItem}
			}

			opts := ReplaceOptions{
				Pattern:     re,
				Replacement: substitution,
				Interactive: cmd.Bool("interactive"),
				DryRun:      cmd.Bool("dry-run"),
				Depth:       int(cmd.Int("depth")),
			}

			var results []ReplaceResult
			collectReplacements(searchRoot, opts, 0, &results)

			if len(results) == 0 {
				if format == "json" {
					fmt.Println("[]")
				} else {
					fmt.Println("No matches found")
				}
				return nil
			}

			appliedCount := 0
			skippedCount := 0

			for i := range results {
				result := &results[i]

				if opts.DryRun {
					continue
				}

				shouldApply := true
				if opts.Interactive {
					confirm, quit := promptConfirmation(*result)
					if quit {
						result.Skipped = true
						result.SkipReason = "user quit"
						for j := i + 1; j < len(results); j++ {
							results[j].Skipped = true
							results[j].SkipReason = "user quit"
						}
						skippedCount += len(results) - i
						break
					}
					shouldApply = confirm
					if !shouldApply {
						result.Skipped = true
						result.SkipReason = "user declined"
						skippedCount++
						continue
					}
				}

				if shouldApply {
					req := &workflowy.UpdateNodeRequest{
						Name: &result.NewName,
					}
					_, err := client.UpdateNode(ctx, result.ID, req)
					if err != nil {
						result.Skipped = true
						result.SkipReason = fmt.Sprintf("update failed: %v", err)
						skippedCount++
						continue
					}
					result.Applied = true
					appliedCount++
				}
			}

			if format == "json" {
				printJSON(results)
			} else {
				for _, result := range results {
					fmt.Println(result.String())
				}
				if opts.DryRun {
					fmt.Printf("\nDry run: %d node(s) would be updated\n", len(results))
				} else {
					fmt.Printf("\nUpdated %d node(s)", appliedCount)
					if skippedCount > 0 {
						fmt.Printf(", skipped %d", skippedCount)
					}
					fmt.Println()
				}
			}

			return nil
		}),
	}
}

func getMcpCommand() *cli.Command {
	return &cli.Command{
		Name:      "mcp",
		Usage:     "Run as MCP server (stdio transport)",
		UsageText: "workflowy mcp [options]",
		Description: `Start the Workflowy MCP server for integration with AI assistants like Claude.

The server communicates via stdio using the Model Context Protocol (MCP).

Tool groups:
  read   Get, List, Search, Targets, and Report tools (default)
  write  Create, Update, Delete, Complete, Uncomplete, Replace, Transform tools
  all    All available tools

Examples:
  workflowy mcp                      # Read-only tools (safe)
  workflowy mcp --expose=all         # All tools including write operations
  workflowy mcp --expose=read,write  # Explicit groups
  workflowy mcp --expose=get,list    # Specific tools only`,
		Flags: []cli.Flag{
			getAPIKeyFlag(),
			&cli.StringFlag{
				Name:  "expose",
				Value: "read",
				Usage: "Tools to expose: read, write, all, or comma-separated tool names",
			},
			getWriteRootIdFlag(),
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			serverConfig := mcp.Config{
				APIKeyFile:        cmd.String("api-key-file"),
				DefaultAPIKeyFile: defaultAPIKeyFile,
				Expose:            cmd.String("expose"),
				Version:           version,
				WriteRootID:       cmd.String("write-root-id"),
			}
			return mcp.RunServer(ctx, serverConfig)
		},
	}
}

func getServeCommand() *cli.Command {
	return &cli.Command{
		Name:      "serve",
		Usage:     "Run as hosted MCP server (streamable HTTP transport with OAuth)",
		UsageText: "workflowy serve [options]",
		Description: `Start the Workflowy MCP server over HTTP with optional OAuth authentication.

This command runs the MCP server using streamable HTTP transport, which is suitable
for hosted deployments accessible to remote MCP clients.

OAuth Authentication:
  When --oauth-issuer is specified, the server requires OAuth 2.0 bearer tokens.
  Clients must obtain tokens from the specified authorization server and include
  them in the Authorization header. The server implements RFC 9728 Protected
  Resource Metadata for automatic OAuth discovery.

TLS/HTTPS:
  For production deployments, use --tls-cert and --tls-key to enable HTTPS.
  This is required for secure OAuth token transmission.

Examples:
  # Start server on port 8080 (no auth, development only)
  workflowy serve --addr=:8080

  # Start with OAuth authentication
  workflowy serve --addr=:8443 \
    --tls-cert=cert.pem --tls-key=key.pem \
    --oauth-issuer=https://auth.example.com \
    --base-url=https://mcp.example.com

  # Restrict to read-only tools
  workflowy serve --addr=:8080 --expose=read`,
		Flags: []cli.Flag{
			getAPIKeyFlag(),
			&cli.StringFlag{
				Name:  "addr",
				Value: ":8080",
				Usage: "Address to listen on (e.g., :8080 or localhost:8080)",
			},
			&cli.StringFlag{
				Name:  "base-url",
				Usage: "Canonical URL of this server (for OAuth resource indicator)",
			},
			&cli.StringFlag{
				Name:  "expose",
				Value: "read",
				Usage: "Tools to expose: read, write, all, or comma-separated tool names",
			},
			getWriteRootIdFlag(),
			&cli.StringFlag{
				Name:  "tls-cert",
				Usage: "Path to TLS certificate file for HTTPS",
			},
			&cli.StringFlag{
				Name:  "tls-key",
				Usage: "Path to TLS key file for HTTPS",
			},
			&cli.StringFlag{
				Name:  "oauth-issuer",
				Usage: "OAuth authorization server URL (enables OAuth authentication)",
			},
			&cli.BoolFlag{
				Name:  "oauth-require-auth",
				Value: true,
				Usage: "Require authentication for all requests (when OAuth is enabled)",
			},
			&cli.StringSliceFlag{
				Name:  "oauth-scope",
				Usage: "OAuth scopes this server accepts (can be specified multiple times)",
			},
			&cli.StringFlag{
				Name:  "endpoint-path",
				Value: "/mcp",
				Usage: "Path for the MCP endpoint",
			},
			&cli.BoolFlag{
				Name:  "cors",
				Usage: "Enable CORS for browser-based clients",
			},
			&cli.StringSliceFlag{
				Name:  "cors-origin",
				Usage: "Allowed CORS origins (if empty, allows all when --cors is enabled)",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			// Build OAuth config if issuer is specified
			var oauthConfig *mcp.OAuthConfig
			oauthIssuer := cmd.String("oauth-issuer")
			if oauthIssuer != "" {
				baseURL := cmd.String("base-url")
				if baseURL == "" {
					// Infer base URL from address
					addr := cmd.String("addr")
					protocol := "http"
					if cmd.String("tls-cert") != "" {
						protocol = "https"
					}
					baseURL = fmt.Sprintf("%s://localhost%s", protocol, addr)
				}

				oauthConfig = &mcp.OAuthConfig{
					AuthorizationServers: []string{oauthIssuer},
					Resource:             baseURL,
					ResourceName:         "Workflowy MCP Server",
					RequireAuth:          cmd.Bool("oauth-require-auth"),
					Scopes:               cmd.StringSlice("oauth-scope"),
				}

				slog.Info("OAuth authentication enabled",
					"issuer", oauthIssuer,
					"require_auth", oauthConfig.RequireAuth,
				)
			}

			httpConfig := mcp.HTTPConfig{
				Config: mcp.Config{
					APIKeyFile:        cmd.String("api-key-file"),
					DefaultAPIKeyFile: defaultAPIKeyFile,
					Expose:            cmd.String("expose"),
					Version:           version,
					WriteRootID:       cmd.String("write-root-id"),
				},
				Addr:           cmd.String("addr"),
				BaseURL:        cmd.String("base-url"),
				TLSCertFile:    cmd.String("tls-cert"),
				TLSKeyFile:     cmd.String("tls-key"),
				OAuth:          oauthConfig,
				EndpointPath:   cmd.String("endpoint-path"),
				EnableCORS:     cmd.Bool("cors"),
				AllowedOrigins: cmd.StringSlice("cors-origin"),
			}

			return mcp.RunHTTPServer(ctx, httpConfig)
		},
	}
}

func getIDCommand() *cli.Command {
	return &cli.Command{
		Name:      "id",
		Usage:     "Resolve a short ID or target key to full UUID",
		UsageText: "workflowy id <id>",
		Arguments: []cli.Argument{
			&cli.StringArg{
				Name:      "id",
				UsageText: "<id>",
			},
		},
		Flags: []cli.Flag{
			getAPIKeyFlag(),
		},
		Action: withClient(func(ctx context.Context, cmd *cli.Command, client workflowy.Client) error {
			rawID := cmd.StringArg("id")
			if rawID == "" {
				return fmt.Errorf("id is required")
			}

			fullID, err := workflowy.ResolveNodeID(ctx, client, rawID)
			if err != nil {
				return err
			}

			fmt.Println(fullID)
			return nil
		}),
	}
}

func getVersionCommand() *cli.Command {
	return &cli.Command{
		Name:      "version",
		Usage:     "Show version information",
		UsageText: "workflowy version",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			fmt.Printf("workflowy version %s\n", version)
			fmt.Printf("commit: %s\n", commit)
			fmt.Printf("built: %s\n", date)
			return nil
		},
	}
}

func createClient(apiKeyFile string) (*workflowy.WorkflowyClient, error) {
	option, err := workflowy.ResolveAPIKey(apiKeyFile, defaultAPIKeyFile)
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
