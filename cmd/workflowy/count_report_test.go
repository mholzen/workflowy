package main

import (
	"bytes"
	"context"
	"testing"

	"github.com/mholzen/workflowy/pkg/workflowy"
	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v3"
)

type MockBackupProvider struct {
	Items []*workflowy.Item
}

func (m *MockBackupProvider) ReadBackupFile(filename string) ([]*workflowy.Item, error) {
	return m.Items, nil
}

func (m *MockBackupProvider) ReadLatestBackup() ([]*workflowy.Item, error) {
	return m.Items, nil
}

func TestCountReportCommand_PrintOutputContainsLinks(t *testing.T) {
	testItems := []*workflowy.Item{
		{
			ID:   "abc123",
			Name: "Parent Item",
			Children: []*workflowy.Item{
				{
					ID:   "def456",
					Name: "Child Item 1",
				},
				{
					ID:   "ghi789",
					Name: "Child Item 2",
				},
			},
		},
	}

	mockBackup := &MockBackupProvider{Items: testItems}
	var output bytes.Buffer

	deps := ReportDeps{
		BackupProvider: mockBackup,
		Output:         &output,
	}

	cmd := getCountReportCommandWithDeps(deps, withOptionalClient)
	err := cmd.Run(context.Background(), []string{"count", "--method=backup", "--threshold=0"})
	assert.NoError(t, err)

	outputStr := output.String()
	t.Logf("Print Output:\n%s", outputStr)

	assert.Contains(t, outputStr, "workflowy.com/#/abc123", "print output should contain link to parent item")
	assert.Contains(t, outputStr, "workflowy.com/#/def456", "print output should contain link to child item 1")
	assert.Contains(t, outputStr, "workflowy.com/#/ghi789", "print output should contain link to child item 2")
}

type MockClient struct {
	CreatedNodes []*workflowy.CreateNodeRequest
}

func (m *MockClient) CreateNode(ctx context.Context, req *workflowy.CreateNodeRequest) (*workflowy.CreateNodeResponse, error) {
	m.CreatedNodes = append(m.CreatedNodes, req)
	return &workflowy.CreateNodeResponse{ItemID: "mock-id"}, nil
}

func (m *MockClient) GetItem(ctx context.Context, itemID string) (*workflowy.Item, error) {
	return nil, nil
}

func (m *MockClient) ListChildren(ctx context.Context, itemID string) (*workflowy.ListChildrenResponse, error) {
	return nil, nil
}

func (m *MockClient) ListChildrenRecursive(ctx context.Context, itemID string) (*workflowy.ListChildrenResponse, error) {
	return nil, nil
}

func (m *MockClient) ListChildrenRecursiveWithDepth(ctx context.Context, itemID string, depth int) (*workflowy.ListChildrenResponse, error) {
	return nil, nil
}

func (m *MockClient) UpdateNode(ctx context.Context, itemID string, req *workflowy.UpdateNodeRequest) (*workflowy.UpdateNodeResponse, error) {
	return nil, nil
}

func (m *MockClient) CompleteNode(ctx context.Context, itemID string) (*workflowy.UpdateNodeResponse, error) {
	return nil, nil
}

func (m *MockClient) UncompleteNode(ctx context.Context, itemID string) (*workflowy.UpdateNodeResponse, error) {
	return nil, nil
}

func (m *MockClient) ExportNodesWithCache(ctx context.Context, forceRefresh bool) (*workflowy.ExportNodesResponse, error) {
	return nil, nil
}

func withMockClient(client workflowy.Client) ClientProvider {
	return func(fn ClientActionFunc) cli.ActionFunc {
		return func(ctx context.Context, cmd *cli.Command) error {
			return fn(ctx, cmd, client)
		}
	}
}

func TestCountReportCommand_UploadContainsLinks(t *testing.T) {
	testItems := []*workflowy.Item{
		{
			ID:   "abc123",
			Name: "Parent Item",
			Children: []*workflowy.Item{
				{
					ID:   "def456",
					Name: "Child Item 1",
				},
				{
					ID:   "ghi789",
					Name: "Child Item 2",
				},
			},
		},
	}

	mockBackup := &MockBackupProvider{Items: testItems}
	mockClient := &MockClient{}
	var output bytes.Buffer

	deps := ReportDeps{
		BackupProvider: mockBackup,
		Output:         &output,
	}

	cmd := getCountReportCommandWithDeps(deps, withMockClient(mockClient))
	err := cmd.Run(context.Background(), []string{"count", "--method=backup", "--upload", "--threshold=0"})
	assert.NoError(t, err)

	t.Logf("Created nodes:\n")
	for _, req := range mockClient.CreatedNodes {
		t.Logf("  - %s\n", req.Name)
	}

	foundLinks := false
	for _, req := range mockClient.CreatedNodes {
		if containsLink(req.Name) {
			foundLinks = true
			break
		}
	}

	assert.True(t, foundLinks, "uploaded nodes should contain links to items (e.g., workflowy.com/#/abc123)")
}

func containsLink(s string) bool {
	return len(s) > 0 && (contains(s, "workflowy.com/#/") || contains(s, "](https://"))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
