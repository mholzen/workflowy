package mcp

import (
	"context"
	"fmt"
	"strings"

	mcptypes "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/mholzen/workflowy/pkg/workflowy"
)

// Config controls MCP server startup.
type Config struct {
	APIKeyFile string
	Expose     string
	Version    string
}

// RunServer starts the MCP stdio server with the requested tool set.
func RunServer(ctx context.Context, cfg Config) error {
	expose := strings.TrimSpace(cfg.Expose)
	if expose == "" {
		expose = "read"
	}

	toolsToEnable, err := ParseExposeList(expose)
	if err != nil {
		return err
	}

	option, err := workflowy.WithAPIKeyFromFile(cfg.APIKeyFile)
	if err != nil {
		return fmt.Errorf("cannot load API key: %w", err)
	}

	client := workflowy.NewWorkflowyClient(option)

	builder := NewToolBuilder(client)
	serverTools, err := builder.BuildTools(toolsToEnable)
	if err != nil {
		return err
	}

	server := mcpserver.NewMCPServer(
		"workflowy",
		cfg.Version,
		mcpserver.WithToolCapabilities(true),
		mcpserver.WithLogging(),
	)

	for _, tool := range serverTools {
		server.AddTool(tool.Tool, tool.Handler)
	}

	return mcpserver.ServeStdio(server, mcpserver.WithStdioContextFunc(func(_ context.Context) context.Context {
		return ctx
	}))
}

// ParseExposeList converts the --expose flag into a deduplicated, ordered tool list.
// Supports groups: all, read, write. Individual tools can be referenced either by
// their short name (e.g., "get") or full MCP name (e.g., "workflowy_get").
func ParseExposeList(raw string) ([]string, error) {
	tokenList := strings.Split(raw, ",")

	var tokens []string
	for _, t := range tokenList {
		token := strings.TrimSpace(strings.ToLower(t))
		if token == "" {
			continue
		}
		tokens = append(tokens, token)
	}

	if len(tokens) == 0 {
		tokens = []string{"read"}
	}

	result := make([]string, 0, len(allTools))
	seen := make(map[string]struct{})

	addSet := func(names []string) {
		for _, name := range names {
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			result = append(result, name)
		}
	}

	for _, token := range tokens {
		if group, ok := groupMap[token]; ok {
			addSet(group)
			continue
		}

		if alias, ok := aliasMap[token]; ok {
			addSet([]string{alias})
			continue
		}

		// Accept the fully qualified tool name if provided.
		if _, ok := aliasMapFull[token]; ok {
			addSet([]string{token})
			continue
		}

		return nil, fmt.Errorf("unknown tool or group in --expose: %s", token)
	}

	return result, nil
}

var (
	allTools = []string{
		ToolGet,
		ToolList,
		ToolSearch,
		ToolTargets,
		ToolCreate,
		ToolUpdate,
		ToolDelete,
		ToolComplete,
		ToolUncomplete,
		ToolReportCount,
		ToolReportChildren,
		ToolReportCreated,
		ToolReportModified,
		ToolReplace,
	}

	readTools = []string{
		ToolGet,
		ToolList,
		ToolSearch,
		ToolTargets,
		ToolReportCount,
		ToolReportChildren,
		ToolReportCreated,
		ToolReportModified,
	}

	writeTools = []string{
		ToolCreate,
		ToolUpdate,
		ToolDelete,
		ToolComplete,
		ToolUncomplete,
		ToolReplace,
	}

	groupMap = map[string][]string{
		"all":   allTools,
		"read":  readTools,
		"write": writeTools,
	}

	aliasMap = map[string]string{
		"get":             ToolGet,
		"list":            ToolList,
		"search":          ToolSearch,
		"targets":         ToolTargets,
		"create":          ToolCreate,
		"update":          ToolUpdate,
		"delete":          ToolDelete,
		"complete":        ToolComplete,
		"uncomplete":      ToolUncomplete,
		"report_count":    ToolReportCount,
		"report_children": ToolReportChildren,
		"report_created":  ToolReportCreated,
		"report_modified": ToolReportModified,
		"replace":         ToolReplace,
	}

	aliasMapFull = func() map[string]string {
		out := make(map[string]string, len(aliasMap))
		for _, fullName := range allTools {
			out[fullName] = fullName
		}
		return out
	}()
)

// Ensure MCP types are imported even if only referenced from handlers.
var _ = mcptypes.CallToolRequest{}
