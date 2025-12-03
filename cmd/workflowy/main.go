package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/urfave/cli/v3"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

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
		Commands: getCommands(),
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		slog.Error("cannot run command", "error", err)
	}
}
