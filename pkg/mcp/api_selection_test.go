package mcp

import (
	"context"
	"sync/atomic"
	"testing"

	mcptypes "github.com/mark3labs/mcp-go/mcp"
	"github.com/mholzen/workflowy/pkg/workflowy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingClient struct {
	calls atomic.Int64
}

func (client *recordingClient) record() {
	client.calls.Add(1)
}

func (client *recordingClient) GetItem(context.Context, string) (*workflowy.Item, error) {
	client.record()
	return &workflowy.Item{ID: "item", Name: "Item"}, nil
}

func (client *recordingClient) ListChildren(context.Context, string) (*workflowy.ListChildrenResponse, error) {
	client.record()
	return &workflowy.ListChildrenResponse{}, nil
}

func (client *recordingClient) ListChildrenRecursive(context.Context, string) (*workflowy.ListChildrenResponse, error) {
	client.record()
	return &workflowy.ListChildrenResponse{}, nil
}

func (client *recordingClient) ListChildrenRecursiveWithDepth(context.Context, string, int) (*workflowy.ListChildrenResponse, error) {
	client.record()
	return &workflowy.ListChildrenResponse{Items: []*workflowy.Item{}}, nil
}

func (client *recordingClient) CreateNode(context.Context, *workflowy.CreateNodeRequest) (*workflowy.CreateNodeResponse, error) {
	client.record()
	return &workflowy.CreateNodeResponse{ItemID: "created"}, nil
}

func (client *recordingClient) UpdateNode(context.Context, string, *workflowy.UpdateNodeRequest) (*workflowy.UpdateNodeResponse, error) {
	client.record()
	return &workflowy.UpdateNodeResponse{Status: "ok"}, nil
}

func (client *recordingClient) MoveNode(context.Context, string, *workflowy.MoveNodeRequest) (*workflowy.MoveNodeResponse, error) {
	client.record()
	return &workflowy.MoveNodeResponse{Status: "ok"}, nil
}

func (client *recordingClient) CompleteNode(context.Context, string) (*workflowy.UpdateNodeResponse, error) {
	client.record()
	return &workflowy.UpdateNodeResponse{Status: "ok"}, nil
}

func (client *recordingClient) UncompleteNode(context.Context, string) (*workflowy.UpdateNodeResponse, error) {
	client.record()
	return &workflowy.UpdateNodeResponse{Status: "ok"}, nil
}

func (client *recordingClient) DeleteNode(context.Context, string) (*workflowy.UpdateNodeResponse, error) {
	client.record()
	return &workflowy.UpdateNodeResponse{Status: "ok"}, nil
}

func (client *recordingClient) ExportNodesWithCache(context.Context, bool) (*workflowy.ExportNodesResponse, error) {
	client.record()
	return &workflowy.ExportNodesResponse{}, nil
}

func (client *recordingClient) ListTargets(context.Context) (*workflowy.ListTargetsResponse, error) {
	client.record()
	return &workflowy.ListTargetsResponse{}, nil
}

func TestNetworkToolsExposeAndDispatchAPISelection(t *testing.T) {
	productionClient := &recordingClient{}
	betaClient := &recordingClient{}
	builder := NewToolBuilder(
		map[workflowy.APIDeployment]workflowy.Client{
			workflowy.ProductionAPI: productionClient,
			workflowy.BetaAPI:       betaClient,
		},
		workflowy.ProductionAPI,
		"None",
		"None",
		"",
		"",
	)

	tools, err := builder.BuildTools(allTools)
	require.NoError(t, err)
	require.Len(t, tools, len(allTools))

	networkToolNames := map[string]struct{}{
		ToolGet: {}, ToolList: {}, ToolSearch: {}, ToolTargets: {}, ToolID: {},
		ToolCreate: {}, ToolUpdate: {}, ToolMove: {}, ToolDelete: {}, ToolComplete: {},
		ToolUncomplete: {}, ToolReportCount: {}, ToolReportChildren: {}, ToolReportCreated: {},
		ToolReportModified: {}, ToolReplace: {}, ToolTransform: {},
	}
	require.Len(t, networkToolNames, 17)

	for _, tool := range tools {
		_, isNetworkTool := networkToolNames[tool.Tool.Name]
		property, hasAPIProperty := tool.Tool.InputSchema.Properties["api"]
		if !isNetworkTool {
			assert.Equal(t, ToolReportMirrors, tool.Tool.Name)
			assert.False(t, hasAPIProperty)
			continue
		}

		require.True(t, hasAPIProperty, "%s must expose api", tool.Tool.Name)
		schema := property.(map[string]any)
		assert.Equal(t, "string", schema["type"])
		assert.ElementsMatch(t, []string{"production", "beta"}, schema["enum"])

		result, handlerErr := tool.Handler(context.Background(), mcptypes.CallToolRequest{
			Params: mcptypes.CallToolParams{
				Name:      tool.Tool.Name,
				Arguments: map[string]any{"api": "staging"},
			},
		})
		require.NoError(t, handlerErr)
		require.True(t, result.IsError, "%s must reject invalid api before its own handler", tool.Tool.Name)
	}

	assert.Zero(t, productionClient.calls.Load())
	assert.Zero(t, betaClient.calls.Load())
}

func TestAPISelectionDispatch(t *testing.T) {
	tests := []struct {
		name              string
		defaultDeployment workflowy.APIDeployment
		arguments         map[string]any
		wantProduction    int64
		wantBeta          int64
	}{
		{
			name:              "explicit beta overrides production default",
			defaultDeployment: workflowy.ProductionAPI,
			arguments:         map[string]any{"api": "beta", "method": "get", "depth": float64(0)},
			wantBeta:          1,
		},
		{
			name:              "omission uses beta server default",
			defaultDeployment: workflowy.BetaAPI,
			arguments:         map[string]any{"method": "get", "depth": float64(0)},
			wantBeta:          1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			productionClient := &recordingClient{}
			betaClient := &recordingClient{}
			builder := NewToolBuilder(
				map[workflowy.APIDeployment]workflowy.Client{
					workflowy.ProductionAPI: productionClient,
					workflowy.BetaAPI:       betaClient,
				},
				test.defaultDeployment,
				"None",
				"None",
				"",
				"",
			)
			tools, err := builder.BuildTools([]string{ToolGet})
			require.NoError(t, err)

			result, handlerErr := tools[0].Handler(context.Background(), mcptypes.CallToolRequest{
				Params: mcptypes.CallToolParams{Name: ToolGet, Arguments: test.arguments},
			})
			require.NoError(t, handlerErr)
			require.False(t, result.IsError)
			assert.Equal(t, test.wantProduction, productionClient.calls.Load())
			assert.Equal(t, test.wantBeta, betaClient.calls.Load())
		})
	}
}
