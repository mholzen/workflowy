package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v3"
)

type FetchParameters struct {
	format string
	depth  int
	itemID string
}

func getMethodFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:  "method",
			Usage: "Access method: get, export or backup\n\tDefaults to 'get' for depth 1-3, 'export' for depth 4+, 'backup' if no api key provided",
		},
		getAPIKeyFlag(),
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
		getDepthFlag(2, "Recursion depth for get/list operations (positive integer)"),
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
		getAPIKeyFlag(),
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

	flags = append(flags, getIdFlag("ID to start from (default: root)"))

	flags = append(flags,
		&cli.BoolFlag{
			Name:  "upload",
			Usage: "Upload report to Workflowy instead of printing",
		},
		getParentIdFlag("Parent ID for uploaded report: UUID or target key (default: root)"),
		&cli.StringFlag{
			Name:  "position",
			Usage: "Position in parent: top or bottom",
		},
		&cli.BoolFlag{
			Name:  "preserve-tags",
			Usage: "Preserve HTML tags in list output (by default, HTML tags are stripped)",
		},
	)

	return flags
}

func getRankingReportFlags() []cli.Flag {
	reportFlags := getReportFlags()
	reportFlags = append(reportFlags,
		getIdFlag("ID to start from (default: root)"),
		&cli.IntFlag{
			Name:  "top-n",
			Value: 20,
			Usage: "Number of top results to show (0 for all)",
		},
	)
	return reportFlags
}

func getFetchArguments() []cli.Argument {
	return []cli.Argument{
		&cli.StringArg{
			Name:      "id",
			Value:     "None",
			UsageText: "<id> (default: root)",
		},
	}
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
	itemID := cmd.StringArg("id")
	return FetchParameters{format: format, depth: depth, itemID: itemID}, nil
}

func validateFormat(format string) error {
	if format != "list" && format != "json" && format != "markdown" {
		return fmt.Errorf("format must be 'list', 'json', or 'markdown'")
	}
	return nil
}

func getIgnoreCaseFlag() cli.Flag {
	return &cli.BoolFlag{
		Name:    "ignore-case",
		Aliases: []string{"i"},
		Usage:   "Case-insensitive matching",
	}
}

func getRegexpFlag() cli.Flag {
	return &cli.BoolFlag{
		Name:    "regexp",
		Aliases: []string{"E"},
		Usage:   "Treat pattern as regular expression",
	}
}

func getParentIdFlag(usage string) cli.Flag {
	return &cli.StringFlag{
		Name:  "parent-id",
		Value: "None",
		Usage: usage,
	}
}

func getIdFlag(usage string) cli.Flag {
	return &cli.StringFlag{
		Name:  "id",
		Value: "None",
		Usage: usage,
	}
}

func getDepthFlag(defaultValue int, usage string) cli.Flag {
	return &cli.IntFlag{
		Name:    "depth",
		Aliases: []string{"d"},
		Value:   defaultValue,
		Usage:   usage,
	}
}

func getSearchFlags() []cli.Flag {
	return []cli.Flag{
		getIgnoreCaseFlag(),
		getRegexpFlag(),
		getIdFlag("ID to search within (default: root)"),
	}
}

func getReplaceFlags() []cli.Flag {
	flags := []cli.Flag{
		getIgnoreCaseFlag(),
		getParentIdFlag("Parent ID to limit replacement scope: UUID or target key (default: root)"),
		getDepthFlag(-1, "Maximum depth to traverse (-1 for unlimited)"),
		&cli.BoolFlag{
			Name:  "interactive",
			Usage: "Prompt for confirmation before each replacement",
		},
		&cli.BoolFlag{
			Name:  "dry-run",
			Usage: "Show what would be replaced without making changes",
		},
	}
	flags = append(flags, getMethodFlags()...)
	return flags
}

var defaultAPIKeyFile string

func init() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("cannot get home directory: %v", err)
	}
	defaultAPIKeyFile = filepath.Join(homeDir, ".workflowy", "api.key")
}

func getAPIKeyFlag() *cli.StringFlag {
	return &cli.StringFlag{
		Name:  "api-key-file",
		Value: defaultAPIKeyFile,
		Usage: "Path to API key file (overrides WORKFLOWY_API_KEY env var)",
	}
}

func getParentID(cmd *cli.Command) string {
	return cmd.String("parent-id")
}

func getID(cmd *cli.Command) string {
	return cmd.String("id")
}

