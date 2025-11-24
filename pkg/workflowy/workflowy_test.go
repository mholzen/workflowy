package workflowy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mholzen/workflowy/pkg/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowyClient_GetItem(t *testing.T) {
	tests := []struct {
		name           string
		itemID         string
		serverResponse interface{}
		statusCode     int
		expectedItem   *Item
		expectError    bool
	}{
		{
			name:   "successful get item",
			itemID: "test-item-123",
			serverResponse: GetItemResponse{
				Node: Item{
					ID:          "test-item-123",
					Name:        "Test Item",
					Note:        stringPtr("This is a test note"),
					Priority:    1,
					Data:        map[string]interface{}{"custom": "data"},
					CreatedAt:   1667666409, // Unix timestamp in seconds (actual API format)
					ModifiedAt:  1756151109, // Unix timestamp in seconds (actual API format)
					CompletedAt: nil,
					Children:    nil,
				},
			},
			statusCode: http.StatusOK,
			expectedItem: &Item{
				ID:          "test-item-123",
				Name:        "Test Item",
				Note:        stringPtr("This is a test note"),
				Priority:    1,
				Data:        map[string]interface{}{"custom": "data"},
				CreatedAt:   1667666409,
				ModifiedAt:  1756151109,
				CompletedAt: nil,
				Children:    nil,
			},
			expectError: false,
		},
		{
			name:   "item with children",
			itemID: "parent-item",
			serverResponse: GetItemResponse{
				Node: Item{
					ID:       "parent-item",
					Name:     "Parent Item",
					Priority: 0,
					Children: []*Item{
						{
							ID:       "child-1",
							Name:     "Child 1",
							Priority: 0,
						},
						{
							ID:       "child-2",
							Name:     "Child 2",
							Priority: 1,
						},
					},
				},
			},
			statusCode: http.StatusOK,
			expectedItem: &Item{
				ID:       "parent-item",
				Name:     "Parent Item",
				Priority: 0,
				Children: []*Item{
					{
						ID:       "child-1",
						Name:     "Child 1",
						Priority: 0,
					},
					{
						ID:       "child-2",
						Name:     "Child 2",
						Priority: 1,
					},
				},
			},
			expectError: false,
		},
		{
			name:   "real API response format",
			itemID: "d6ef2ccc-8853-ede1-d6c8-f03667f91df9",
			serverResponse: GetItemResponse{
				Node: Item{
					ID:          "d6ef2ccc-8853-ede1-d6c8-f03667f91df9",
					Name:        "synthesize",
					Note:        nil,
					Priority:    400,
					Data:        map[string]interface{}{"layoutMode": "bullets"},
					CreatedAt:   1667666409,
					ModifiedAt:  1756151109,
					CompletedAt: nil,
					Children:    nil,
				},
			},
			statusCode: http.StatusOK,
			expectedItem: &Item{
				ID:          "d6ef2ccc-8853-ede1-d6c8-f03667f91df9",
				Name:        "synthesize",
				Note:        nil,
				Priority:    400,
				Data:        map[string]interface{}{"layoutMode": "bullets"},
				CreatedAt:   1667666409,
				ModifiedAt:  1756151109,
				CompletedAt: nil,
				Children:    nil,
			},
			expectError: false,
		},
		{
			name:           "server error",
			itemID:         "nonexistent-item",
			serverResponse: map[string]string{"error": "Item not found"},
			statusCode:     http.StatusNotFound,
			expectedItem:   nil,
			expectError:    true,
		},
		{
			name:           "server internal error",
			itemID:         "error-item",
			serverResponse: map[string]string{"error": "Internal server error"},
			statusCode:     http.StatusInternalServerError,
			expectedItem:   nil,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request method and path
				assert.Equal(t, "GET", r.Method)
				expectedPath := "/nodes/" + tt.itemID
				assert.Equal(t, expectedPath, r.URL.Path)
				assert.Equal(t, "application/json", r.Header.Get("Accept"))

				// Send response
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				json.NewEncoder(w).Encode(tt.serverResponse)
			}))
			defer server.Close()

			// Create client with test server URL
			client := &WorkflowyClient{
				Client: client.New(server.URL),
			}

			// Execute the method under test
			ctx := context.Background()
			result, err := client.GetItem(ctx, tt.itemID)

			// Assert results
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, tt.expectedItem, result)
			}
		})
	}
}

func TestWorkflowyClient_GetItem_ContextCancellation(t *testing.T) {
	// Create a server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(GetItemResponse{
			Node: Item{ID: "test", Name: "Test"},
		})
	}))
	defer server.Close()

	client := &WorkflowyClient{
		Client: client.New(server.URL),
	}

	// Create context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	result, err := client.GetItem(ctx, "test-item")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestWorkflowyClient_GetItem_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json response"))
	}))
	defer server.Close()

	client := &WorkflowyClient{
		Client: client.New(server.URL),
	}

	ctx := context.Background()
	result, err := client.GetItem(ctx, "test-item")

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestWorkflowyClient_ListChildrenRecursiveWithDepth(t *testing.T) {
	// Create a test server that simulates a deep hierarchy
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify it's a GET request
		assert.Equal(t, "GET", r.Method)

		// Extract parent_id from query string
		parentID := r.URL.Query().Get("parent_id")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Simulate a 4-level deep hierarchy: root -> level1 -> level2 -> level3 -> level4
		switch parentID {
		case "root":
			json.NewEncoder(w).Encode(ListChildrenResponse{
				Items: []*Item{
					{ID: "level1", Name: "Level 1"},
				},
			})
		case "level1":
			json.NewEncoder(w).Encode(ListChildrenResponse{
				Items: []*Item{
					{ID: "level2", Name: "Level 2"},
				},
			})
		case "level2":
			json.NewEncoder(w).Encode(ListChildrenResponse{
				Items: []*Item{
					{ID: "level3", Name: "Level 3"},
				},
			})
		case "level3":
			json.NewEncoder(w).Encode(ListChildrenResponse{
				Items: []*Item{
					{ID: "level4", Name: "Level 4"},
				},
			})
		case "level4":
			// Deepest level has no children
			json.NewEncoder(w).Encode(ListChildrenResponse{Items: []*Item{}})
		default:
			json.NewEncoder(w).Encode(ListChildrenResponse{Items: []*Item{}})
		}
	}))
	defer server.Close()

	client := &WorkflowyClient{
		Client: client.New(server.URL),
	}
	ctx := context.Background()

	tests := []struct {
		name          string
		depth         int
		expectedDepth int // How many levels should be populated
	}{
		{
			name:          "depth 0 - no children",
			depth:         0,
			expectedDepth: 0,
		},
		{
			name:          "depth 1 - direct children only",
			depth:         1,
			expectedDepth: 1,
		},
		{
			name:          "depth 2 - two levels",
			depth:         2,
			expectedDepth: 2,
		},
		{
			name:          "depth 3 - three levels",
			depth:         3,
			expectedDepth: 3,
		},
		{
			name:          "depth 10 - beyond available depth",
			depth:         10,
			expectedDepth: 4, // Only 4 levels exist in our test data
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := client.ListChildrenRecursiveWithDepth(ctx, "root", tt.depth)
			require.NoError(t, err)
			require.NotNil(t, result)

			// Check that we got the expected depth
			actualDepth := calculateDepth(result.Items)
			assert.Equal(t, tt.expectedDepth, actualDepth, "Expected depth %d but got %d", tt.expectedDepth, actualDepth)
		})
	}
}

func TestWorkflowyClient_ListChildrenRecursive_DefaultDepth(t *testing.T) {
	// Test that the default method uses depth 10
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(ListChildrenResponse{
			Items: []*Item{
				{ID: "child1", Name: "Child 1"},
			},
		})
	}))
	defer server.Close()

	client := &WorkflowyClient{
		Client: client.New(server.URL),
	}
	ctx := context.Background()

	// This should work the same as calling ListChildrenRecursiveWithDepth with depth 10
	result, err := client.ListChildrenRecursive(ctx, "root")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Items, 1)
	assert.Equal(t, "child1", result.Items[0].ID)
}

// Helper function to calculate the actual depth of a tree
func calculateDepth(items []*Item) int {
	if len(items) == 0 {
		return 0
	}

	maxChildDepth := 0
	for _, item := range items {
		childDepth := calculateDepth(item.Children)
		if childDepth > maxChildDepth {
			maxChildDepth = childDepth
		}
	}

	return maxChildDepth + 1
}

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}
