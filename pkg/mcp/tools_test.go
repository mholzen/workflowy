package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	mcptypes "github.com/mark3labs/mcp-go/mcp"
	"github.com/mholzen/workflowy/pkg/workflowy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	backupRootID  = "11111111-1111-1111-1111-aaaaaaaaaaaa"
	backupChildID = "22222222-2222-2222-2222-bbbbbbbbbbbb"
)

type countingBackupProvider struct {
	items          []*workflowy.Item
	fileReads      int
	latestReads    int
	lastBackupFile string
}

type treeClient struct {
	recordingClient
	nodes   []workflowy.ExportNode
	targets map[string]string
}

func (client *treeClient) GetItem(_ context.Context, itemID string) (*workflowy.Item, error) {
	client.record()
	if resolvedID, ok := client.targets[itemID]; ok {
		return &workflowy.Item{ID: resolvedID, Name: itemID}, nil
	}
	return &workflowy.Item{ID: itemID, Name: itemID}, nil
}

func (client *treeClient) ExportNodesWithCache(context.Context, bool) (*workflowy.ExportNodesResponse, error) {
	client.record()
	return &workflowy.ExportNodesResponse{Nodes: client.nodes}, nil
}

func (client *treeClient) ListTargets(context.Context) (*workflowy.ListTargetsResponse, error) {
	client.record()
	response := &workflowy.ListTargetsResponse{}
	for key := range client.targets {
		response.Targets = append(response.Targets, workflowy.Target{Key: key})
	}
	return response, nil
}

func (provider *countingBackupProvider) ReadBackupFile(filename string) ([]*workflowy.Item, error) {
	provider.fileReads++
	provider.lastBackupFile = filename
	return provider.items, nil
}

func (provider *countingBackupProvider) ReadLatestBackup() ([]*workflowy.Item, error) {
	provider.latestReads++
	return provider.items, nil
}

func backupTestTree() []*workflowy.Item {
	return []*workflowy.Item{
		{
			ID:   backupRootID,
			Name: "Root branch",
			Children: []*workflowy.Item{
				{ID: backupChildID, Name: "Child"},
			},
		},
	}
}

func newBackupTestBuilder(readRootID, writeRootID, backupFile string) (ToolBuilder, *recordingClient, *recordingClient, *countingBackupProvider) {
	productionClient := &recordingClient{}
	betaClient := &recordingClient{}
	backupProvider := &countingBackupProvider{items: backupTestTree()}
	builder := NewToolBuilder(
		map[workflowy.APIDeployment]workflowy.Client{
			workflowy.ProductionAPI: productionClient,
			workflowy.BetaAPI:       betaClient,
		},
		workflowy.ProductionAPI,
		writeRootID,
		readRootID,
		"",
		backupFile,
	)
	builder.backupProvider = backupProvider
	return builder, productionClient, betaClient, backupProvider
}

func callTool(t *testing.T, builder ToolBuilder, toolName string, arguments map[string]any) *mcptypes.CallToolResult {
	t.Helper()
	tools, err := builder.BuildTools([]string{toolName})
	require.NoError(t, err)
	result, err := tools[0].Handler(context.Background(), mcptypes.CallToolRequest{
		Params: mcptypes.CallToolParams{Name: toolName, Arguments: arguments},
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	return result
}

func toolResultText(t *testing.T, result *mcptypes.CallToolResult) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	require.NoError(t, err)
	return string(encoded)
}

func TestBackupInvocationResolvesRootsAndIDsOffline(t *testing.T) {
	builder, productionClient, betaClient, backupProvider := newBackupTestBuilder(backupRootID, "None", "fixture.backup")

	result := callTool(t, builder, ToolGet, map[string]any{
		"api":    "beta",
		"method": "backup",
		"id":     "bbbbbbbbbbbb",
		"depth":  float64(0),
	})

	require.False(t, result.IsError, toolResultText(t, result))
	assert.Equal(t, 1, backupProvider.fileReads)
	assert.Equal(t, "fixture.backup", backupProvider.lastBackupFile)
	assert.Zero(t, backupProvider.latestReads)
	assert.Zero(t, productionClient.calls.Load())
	assert.Zero(t, betaClient.calls.Load())
}

func TestBackupValidationWritesThroughSelectedAPI(t *testing.T) {
	builder, productionClient, betaClient, backupProvider := newBackupTestBuilder("None", "aaaaaaaaaaaa", "fixture.backup")

	result := callTool(t, builder, ToolCreate, map[string]any{
		"api":       "beta",
		"method":    "backup",
		"name":      "Created from backup validation",
		"parent_id": "bbbbbbbbbbbb",
	})

	require.False(t, result.IsError, toolResultText(t, result))
	assert.Equal(t, 1, backupProvider.fileReads)
	assert.Zero(t, productionClient.calls.Load())
	assert.Equal(t, int64(1), betaClient.calls.Load())
}

func TestBackupRestrictionRootRejectsTargetKeyWithoutAPI(t *testing.T) {
	tests := []struct {
		name       string
		backupFile string
		wantLabel  string
	}{
		{name: "configured file", backupFile: "fixture.backup", wantLabel: "fixture.backup"},
		{name: "latest backup", wantLabel: "latest Workflowy backup"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			builder, productionClient, betaClient, backupProvider := newBackupTestBuilder("inbox", "None", test.backupFile)
			result := callTool(t, builder, ToolGet, map[string]any{"method": "backup", "depth": float64(0)})

			require.True(t, result.IsError)
			assert.Contains(t, toolResultText(t, result), `Cannot resolve read root \"inbox\" from backup \"`+test.wantLabel+`\": target keys require an API; configure read_root_id with a full or short node ID`)
			assert.Equal(t, 1, backupProvider.fileReads+backupProvider.latestReads)
			assert.Zero(t, productionClient.calls.Load())
			assert.Zero(t, betaClient.calls.Load())
		})
	}
}

func TestMirrorReportUsesConfiguredBackupAndReadRoot(t *testing.T) {
	insideMirrorID := "33333333-3333-3333-3333-cccccccccccc"
	outsideRootID := "44444444-4444-4444-4444-dddddddddddd"
	outsideMirrorID := "55555555-5555-5555-5555-eeeeeeeeeeee"

	builder, productionClient, betaClient, backupProvider := newBackupTestBuilder("aaaaaaaaaaaa", "None", "mirrors.backup")
	backupProvider.items = []*workflowy.Item{
		{
			ID: backupRootID, Name: "Included branch", Children: []*workflowy.Item{
				{ID: insideMirrorID, Name: "Included mirror", Data: map[string]any{"mirror": map[string]any{"mirrorRootIds": map[string]any{"copy-a": true}}}},
			},
		},
		{
			ID: outsideRootID, Name: "Excluded branch", Children: []*workflowy.Item{
				{ID: outsideMirrorID, Name: "Excluded mirror", Data: map[string]any{"mirror": map[string]any{"mirrorRootIds": map[string]any{"copy-b": true}}}},
			},
		},
	}

	result := callTool(t, builder, ToolReportMirrors, map[string]any{"top_n": float64(20)})

	require.False(t, result.IsError, toolResultText(t, result))
	resultText := toolResultText(t, result)
	assert.True(t, strings.Contains(resultText, "Included mirror"))
	assert.False(t, strings.Contains(resultText, "Excluded mirror"))
	assert.Equal(t, 1, backupProvider.fileReads)
	assert.Equal(t, "mirrors.backup", backupProvider.lastBackupFile)
	assert.Zero(t, productionClient.calls.Load())
	assert.Zero(t, betaClient.calls.Load())
}

func TestMirrorReportAcceptsFullReadRoot(t *testing.T) {
	builder, productionClient, betaClient, backupProvider := newBackupTestBuilder(backupRootID, "None", "mirrors.backup")

	result := callTool(t, builder, ToolReportMirrors, map[string]any{})

	require.False(t, result.IsError, toolResultText(t, result))
	assert.Equal(t, 1, backupProvider.fileReads)
	assert.Zero(t, productionClient.calls.Load())
	assert.Zero(t, betaClient.calls.Load())
}

func TestMirrorReportRejectsTargetKeyRootOffline(t *testing.T) {
	builder, productionClient, betaClient, backupProvider := newBackupTestBuilder("inbox", "None", "mirrors.backup")

	result := callTool(t, builder, ToolReportMirrors, map[string]any{})

	require.True(t, result.IsError)
	assert.Contains(t, toolResultText(t, result), `Cannot resolve read root \"inbox\" from backup \"mirrors.backup\": target keys require an API`)
	assert.Equal(t, 1, backupProvider.fileReads)
	assert.Zero(t, productionClient.calls.Load())
	assert.Zero(t, betaClient.calls.Load())
}

func TestNetworkResolutionUsesOnlySelectedDeployment(t *testing.T) {
	parentID := backupRootID
	productionClient := &treeClient{nodes: []workflowy.ExportNode{{ID: "99999999-9999-9999-9999-999999999999"}}}
	betaClient := &treeClient{
		targets: map[string]string{"inbox": backupRootID},
		nodes: []workflowy.ExportNode{
			{ID: backupRootID, Name: "Beta root"},
			{ID: backupChildID, Name: "Beta child", ParentID: &parentID},
		},
	}
	builder := NewToolBuilder(
		map[workflowy.APIDeployment]workflowy.Client{
			workflowy.ProductionAPI: productionClient,
			workflowy.BetaAPI:       betaClient,
		},
		workflowy.ProductionAPI,
		"None",
		"inbox",
		"",
		"",
	)

	result := callTool(t, builder, ToolGet, map[string]any{
		"api":    "beta",
		"method": "export",
		"id":     "bbbbbbbbbbbb",
		"depth":  float64(0),
	})

	require.False(t, result.IsError, toolResultText(t, result))
	assert.Zero(t, productionClient.calls.Load())
	assert.Positive(t, betaClient.calls.Load())
}

func TestFailedNetworkRootResolutionDoesNotLeakAcrossInvocations(t *testing.T) {
	productionClient := &treeClient{nodes: []workflowy.ExportNode{{ID: backupRootID, Name: "Production root"}}}
	betaClient := &treeClient{nodes: []workflowy.ExportNode{{ID: backupChildID, Name: "Beta node"}}}
	builder := NewToolBuilder(
		map[workflowy.APIDeployment]workflowy.Client{
			workflowy.ProductionAPI: productionClient,
			workflowy.BetaAPI:       betaClient,
		},
		workflowy.ProductionAPI,
		"None",
		"aaaaaaaaaaaa",
		"",
		"",
	)

	betaResult := callTool(t, builder, ToolGet, map[string]any{"api": "beta", "method": "export", "depth": float64(0)})
	require.True(t, betaResult.IsError)
	assert.Contains(t, toolResultText(t, betaResult), `Cannot resolve read root \"aaaaaaaaaaaa\" using Workflowy API \"beta\"`)

	productionResult := callTool(t, builder, ToolGet, map[string]any{"api": "production", "method": "export", "depth": float64(0)})
	require.False(t, productionResult.IsError, toolResultText(t, productionResult))
	assert.Positive(t, productionClient.calls.Load())
	assert.Positive(t, betaClient.calls.Load())
}

func TestNetworkRestrictionRootMustExistInSelectedExport(t *testing.T) {
	productionClient := &treeClient{nodes: []workflowy.ExportNode{{ID: backupRootID, Name: "Production root"}}}
	betaClient := &treeClient{nodes: []workflowy.ExportNode{{ID: backupChildID, Name: "Different beta node"}}}
	builder := NewToolBuilder(
		map[workflowy.APIDeployment]workflowy.Client{
			workflowy.ProductionAPI: productionClient,
			workflowy.BetaAPI:       betaClient,
		},
		workflowy.ProductionAPI,
		"None",
		backupRootID,
		"",
		"",
	)

	result := callTool(t, builder, ToolGet, map[string]any{"api": "beta", "method": "get", "depth": float64(0)})

	require.True(t, result.IsError)
	assert.Contains(t, toolResultText(t, result), `Cannot resolve read root \"`+backupRootID+`\" using Workflowy API \"beta\": node was not found in export`)
	assert.Zero(t, productionClient.calls.Load())
	assert.Positive(t, betaClient.calls.Load())
}

func TestBackupRequestIDsResolveOfflineAcrossHandlers(t *testing.T) {
	tests := []struct {
		name         string
		toolName     string
		arguments    map[string]any
		wantAPICalls int64
	}{
		{name: "get id", toolName: ToolGet, arguments: map[string]any{"id": "bbbbbbbbbbbb", "depth": float64(0)}},
		{name: "get to_ancestor", toolName: ToolGet, arguments: map[string]any{"id": "bbbbbbbbbbbb", "to_ancestor": "aaaaaaaaaaaa", "depth": float64(0)}},
		{name: "list id", toolName: ToolList, arguments: map[string]any{"id": "aaaaaaaaaaaa", "depth": float64(1)}},
		{name: "search id", toolName: ToolSearch, arguments: map[string]any{"id": "aaaaaaaaaaaa", "pattern": "Child"}},
		{name: "id tool id", toolName: ToolID, arguments: map[string]any{"id": "bbbbbbbbbbbb"}},
		{name: "create parent_id", toolName: ToolCreate, arguments: map[string]any{"name": "New", "parent_id": "aaaaaaaaaaaa"}, wantAPICalls: 1},
		{name: "update id", toolName: ToolUpdate, arguments: map[string]any{"id": "bbbbbbbbbbbb", "name": "Updated"}, wantAPICalls: 1},
		{name: "move id and parent_id", toolName: ToolMove, arguments: map[string]any{"id": "bbbbbbbbbbbb", "parent_id": "aaaaaaaaaaaa"}, wantAPICalls: 1},
		{name: "delete id", toolName: ToolDelete, arguments: map[string]any{"id": "bbbbbbbbbbbb"}, wantAPICalls: 1},
		{name: "complete id", toolName: ToolComplete, arguments: map[string]any{"id": "bbbbbbbbbbbb"}, wantAPICalls: 1},
		{name: "uncomplete id", toolName: ToolUncomplete, arguments: map[string]any{"id": "bbbbbbbbbbbb"}, wantAPICalls: 1},
		{name: "count report id", toolName: ToolReportCount, arguments: map[string]any{"id": "aaaaaaaaaaaa", "threshold": float64(0)}},
		{name: "children report id", toolName: ToolReportChildren, arguments: map[string]any{"id": "aaaaaaaaaaaa", "top_n": float64(20)}},
		{name: "created report id", toolName: ToolReportCreated, arguments: map[string]any{"id": "aaaaaaaaaaaa", "top_n": float64(20)}},
		{name: "modified report id", toolName: ToolReportModified, arguments: map[string]any{"id": "aaaaaaaaaaaa", "top_n": float64(20)}},
		{name: "replace parent_id", toolName: ToolReplace, arguments: map[string]any{"parent_id": "aaaaaaaaaaaa", "pattern": "Child", "substitution": "Task", "dry_run": true}},
		{name: "transform id", toolName: ToolTransform, arguments: map[string]any{"id": "bbbbbbbbbbbb", "transform_name": "trim", "dry_run": true}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			builder, productionClient, betaClient, backupProvider := newBackupTestBuilder("None", "None", "fixture.backup")
			builder.method = "backup"
			test.arguments["api"] = "beta"

			result := callTool(t, builder, test.toolName, test.arguments)

			require.False(t, result.IsError, toolResultText(t, result))
			assert.Equal(t, 1, backupProvider.fileReads)
			assert.Zero(t, productionClient.calls.Load())
			assert.Equal(t, test.wantAPICalls, betaClient.calls.Load())
		})
	}
}
