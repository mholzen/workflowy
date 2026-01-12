package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/mholzen/workflowy/pkg/transform"
	"github.com/mholzen/workflowy/pkg/workflowy"
	"github.com/urfave/cli/v3"
)

func getTransformCommand() *cli.Command {
	return &cli.Command{
		Name:  "transform",
		Usage: "Transform node names and/or notes using built-in or shell transformations",
		Description: `Apply transformations to node names and/or notes.

Built-in transforms: ` + strings.Join(transform.ListBuiltins(), ", ") + `

By default, transforms are applied to names. Use --note to transform notes,
or both --name and --note to transform both fields.

The split transform (--separator) splits text into child nodes:
  workflowy transform abc123 --separator=","     # Split by comma
  workflowy transform abc123 --separator="\n"    # Split by newline

Examples:
  workflowy transform abc123 lowercase
  workflowy transform abc123 uppercase --note
  workflowy transform abc123 trim --name --note
  workflowy transform abc123 -x 'echo {} | sed "s/foo/bar/"'
  workflowy transform abc123 uppercase --dry-run --depth 2
  workflowy transform abc123 --separator=", " --dry-run`,
		Arguments: []cli.Argument{
			&cli.StringArg{
				Name:      "item_id",
				UsageText: "<node-id>",
			},
			&cli.StringArg{
				Name:      "transform_name",
				UsageText: "[transform-name]",
			},
		},
		Flags: getTransformFlags(),
		Action: withClient(func(ctx context.Context, cmd *cli.Command, client workflowy.Client) error {
			return runTransform(ctx, cmd, client)
		}),
	}
}

func getTransformFlags() []cli.Flag {
	return append(getMethodFlags(),
		getDepthFlag(-1, "Maximum depth to traverse (-1 for unlimited)"),
		&cli.BoolFlag{
			Name:  "dry-run",
			Usage: "Show changes without applying them",
		},
		&cli.BoolFlag{
			Name:    "interactive",
			Aliases: []string{"i"},
			Usage:   "Confirm each transformation",
		},
		&cli.StringFlag{
			Name:    "exec",
			Aliases: []string{"x"},
			Usage:   "Shell command template (use {} for input text)",
		},
		&cli.StringFlag{
			Name:    "separator",
			Aliases: []string{"s"},
			Usage:   "Split text by separator and create child nodes for each part",
		},
		&cli.BoolFlag{
			Name:  "name",
			Usage: "Transform node names (default if neither --name nor --note specified)",
		},
		&cli.BoolFlag{
			Name:  "note",
			Usage: "Transform node notes",
		},
	)
}

func runTransform(ctx context.Context, cmd *cli.Command, client workflowy.Client) error {
	format := cmd.String("format")
	if err := validateFormat(format); err != nil {
		return err
	}

	rawItemID := cmd.StringArg("item_id")
	if rawItemID == "" {
		return fmt.Errorf("node-id is required")
	}

	itemID, err := workflowy.ResolveNodeID(ctx, client, rawItemID)
	if err != nil {
		return fmt.Errorf("cannot resolve item ID: %w", err)
	}

	items, err := loadTree(ctx, cmd, client)
	if err != nil {
		return err
	}

	searchRoot := items
	if itemID != "None" {
		rootItem := findItemByID(items, itemID)
		if rootItem == nil {
			return fmt.Errorf("item not found: %s", itemID)
		}
		searchRoot = []*workflowy.Item{rootItem}
	}

	separator := cmd.String("separator")
	if separator != "" {
		return runSplitTransform(ctx, cmd, client, searchRoot, separator, format)
	}

	t, err := transform.ResolveTransformer(cmd.StringArg("transform_name"), cmd.String("exec"))
	if err != nil {
		return err
	}

	opts := transform.Options{
		Transformer: t,
		Fields:      transform.DetermineFields(cmd.Bool("name"), cmd.Bool("note")),
		DryRun:      cmd.Bool("dry-run"),
		Interactive: cmd.Bool("interactive"),
		Depth:       int(cmd.Int("depth")),
	}

	var results []transform.Result
	transform.CollectTransformations(searchRoot, opts, 0, &results)

	if len(results) == 0 {
		if format == "json" {
			fmt.Println("[]")
		} else {
			fmt.Println("No transformations to apply")
		}
		return nil
	}

	if !opts.DryRun {
		if opts.Interactive {
			applyResultsInteractively(ctx, client, results)
		} else {
			transform.ApplyResults(ctx, client, results)
		}
	}

	return printTransformResults(results, format, opts.DryRun)
}

func runSplitTransform(ctx context.Context, cmd *cli.Command, client workflowy.Client, searchRoot []*workflowy.Item, separator, format string) error {
	separator = unescapeSeparator(separator)

	fields := transform.DetermineFields(cmd.Bool("name"), cmd.Bool("note"))
	dryRun := cmd.Bool("dry-run")
	depth := int(cmd.Int("depth"))

	var results []transform.SplitResult
	transform.CollectSplits(searchRoot, separator, fields, true, 0, depth, &results)

	if len(results) == 0 {
		if format == "json" {
			fmt.Println("[]")
		} else {
			fmt.Println("No nodes to split")
		}
		return nil
	}

	if !dryRun {
		transform.ApplySplitResults(ctx, client, results)
	}

	return printSplitResults(results, format, dryRun)
}

func unescapeSeparator(s string) string {
	s = strings.ReplaceAll(s, "\\n", "\n")
	s = strings.ReplaceAll(s, "\\t", "\t")
	s = strings.ReplaceAll(s, "\\r", "\r")
	return s
}

func printSplitResults(results []transform.SplitResult, format string, dryRun bool) error {
	if format == "json" {
		printJSON(results)
		return nil
	}

	appliedCount := 0
	skippedCount := 0
	totalChildren := 0
	for _, result := range results {
		fmt.Println(result.String())
		totalChildren += len(result.Parts)
		if result.Applied {
			appliedCount++
		}
		if result.Skipped {
			skippedCount++
		}
	}

	if dryRun {
		fmt.Printf("\nDry run: %d node(s) would be split into %d children\n", len(results), totalChildren)
		return nil
	}

	fmt.Printf("\nSplit %d node(s) into %d children", appliedCount, totalChildren)
	if skippedCount > 0 {
		fmt.Printf(", skipped %d", skippedCount)
	}
	fmt.Println()
	return nil
}

func applyResultsInteractively(ctx context.Context, client workflowy.Client, results []transform.Result) {
	reader := bufio.NewReader(os.Stdin)

	for i := range results {
		result := &results[i]
		if result.Skipped {
			continue
		}

		fmt.Printf("Transform %s (%s): \"%s\" → \"%s\"? [y/N/q] ",
			result.ID, result.Field, result.Original, result.New)
		response, err := reader.ReadString('\n')
		if err != nil {
			result.Skipped = true
			result.SkipReason = fmt.Sprintf("read error: %v", err)
			continue
		}

		response = strings.TrimSpace(strings.ToLower(response))
		if response == "q" || response == "quit" {
			result.Skipped = true
			result.SkipReason = "user quit"
			for j := i + 1; j < len(results); j++ {
				results[j].Skipped = true
				results[j].SkipReason = "user quit"
			}
			break
		}

		if response != "y" && response != "yes" {
			result.Skipped = true
			result.SkipReason = "user declined"
			continue
		}

		req := transform.BuildUpdateRequest(result)
		if _, err := client.UpdateNode(ctx, result.ID, req); err != nil {
			result.Skipped = true
			result.SkipReason = fmt.Sprintf("update failed: %v", err)
			continue
		}
		result.Applied = true
	}
}

func printTransformResults(results []transform.Result, format string, dryRun bool) error {
	if format == "json" {
		printJSON(results)
		return nil
	}

	appliedCount := 0
	skippedCount := 0
	for _, result := range results {
		fmt.Println(result.String())
		if result.Applied {
			appliedCount++
		}
		if result.Skipped {
			skippedCount++
		}
	}

	if dryRun {
		fmt.Printf("\nDry run: %d transformation(s) would be applied\n", len(results))
	} else {
		fmt.Printf("\nApplied %d transformation(s)", appliedCount)
		if skippedCount > 0 {
			fmt.Printf(", skipped %d", skippedCount)
		}
		fmt.Println()
	}
	return nil
}
