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
		Name:      "transform",
		Usage:     "Transform node names and/or notes using built-in or shell transformations",
		UsageText: "workflowy transform <id> [<transform-name>] [options]",
		Description: `Apply transformations to node names and/or notes.

Built-in transforms: ` + strings.Join(transform.ListBuiltins(), ", ") + `, split

By default, transforms are applied to names. Use --note to transform notes,
or both --name and --note to transform both fields.

Examples:
  workflowy transform 1a2b3c lowercase
  workflowy transform 1a2b3c uppercase --note
  workflowy transform 1a2b3c trim --name --note
  workflowy transform 1a2b3c split                 # Split by "," (default)
  workflowy transform 1a2b3c split -s "\n"         # Split by newline
  workflowy transform 1a2b3c -x 'echo {} | tr a-z A-Z'
  workflowy transform 1a2b3c uppercase --as-child  # Insert as child, keep original
  workflowy transform 1a2b3c uppercase --dry-run --depth 2`,
		Arguments: []cli.Argument{
			&cli.StringArg{
				Name:      "id",
				UsageText: "<id>",
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
			Value:   ",",
			Usage:   "Separator for split transform (default: \",\")",
		},
		&cli.BoolFlag{
			Name:  "name",
			Usage: "Transform node names (default if neither --name nor --note specified)",
		},
		&cli.BoolFlag{
			Name:  "note",
			Usage: "Transform node notes",
		},
		&cli.BoolFlag{
			Name:  "as-child",
			Usage: "Insert result as child of source node instead of replacing",
		},
	)
}

func runTransform(ctx context.Context, cmd *cli.Command, client workflowy.Client) error {
	format := cmd.String("format")
	if err := validateFormat(format); err != nil {
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

	transformName := cmd.StringArg("transform_name")
	execCmd := cmd.String("exec")

	// Handle split transform
	if transformName == "split" {
		separator := cmd.String("separator")
		return runSplitTransform(ctx, cmd, client, searchRoot, separator, format)
	}

	// Handle exec (no transform_name required)
	if execCmd != "" {
		if transformName != "" {
			return fmt.Errorf("cannot use both transform name and --exec")
		}
	} else if transformName == "" {
		return fmt.Errorf("transform name required (use a built-in, 'split', or --exec)")
	}

	t, err := transform.ResolveTransformer(transformName, execCmd)
	if err != nil {
		return err
	}

	opts := transform.Options{
		Transformer: t,
		Fields:      transform.DetermineFields(cmd.Bool("name"), cmd.Bool("note")),
		DryRun:      cmd.Bool("dry-run"),
		Interactive: cmd.Bool("interactive"),
		Depth:       int(cmd.Int("depth")),
		AsChild:     cmd.Bool("as-child"),
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
			applyResultsInteractively(ctx, client, results, opts.AsChild)
		} else {
			transform.ApplyResultsWithOptions(ctx, client, results, opts.AsChild)
		}
	}

	return printTransformResults(results, format, opts.DryRun)
}

func runSplitTransform(ctx context.Context, cmd *cli.Command, client workflowy.Client, searchRoot []*workflowy.Item, separator, format string) error {
	separator = transform.UnescapeSeparator(separator)

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

func applyResultsInteractively(ctx context.Context, client workflowy.Client, results []transform.Result, asChild bool) {
	reader := bufio.NewReader(os.Stdin)

	action := "Transform"
	if asChild {
		action = "Create child from"
	}

	for i := range results {
		result := &results[i]
		if result.Skipped {
			continue
		}

		fmt.Printf("%s %s (%s): \"%s\" → \"%s\"? [y/N/q] ",
			action, result.ID, result.Field, result.Original, result.New)
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

		if asChild {
			position := "top"
			req := &workflowy.CreateNodeRequest{
				ParentID: result.ID,
				Position: &position,
			}
			if result.Field == "name" {
				req.Name = result.New
			} else if result.Field == "note" {
				req.Note = &result.New
			}
			resp, err := client.CreateNode(ctx, req)
			if err != nil {
				result.Skipped = true
				result.SkipReason = fmt.Sprintf("create child failed: %v", err)
				continue
			}
			result.CreatedID = resp.ItemID
			result.Applied = true
		} else {
			req := transform.BuildUpdateRequest(result)
			if _, err := client.UpdateNode(ctx, result.ID, req); err != nil {
				result.Skipped = true
				result.SkipReason = fmt.Sprintf("update failed: %v", err)
				continue
			}
			result.Applied = true
		}
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
