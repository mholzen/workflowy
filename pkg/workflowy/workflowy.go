package workflowy

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/mholzen/workflowy/pkg/cache"
	"github.com/mholzen/workflowy/pkg/client"
)

// WithAPIKey sets up Bearer token authentication
func WithAPIKey(apiKey string) client.Option {
	return func(c *client.Client) {
		c.SetAuth(func(r *http.Request) {
			r.Header.Set("Authorization", "Bearer "+apiKey)
		})
	}
}

// WithAPIKeyFromFile reads API key from file and sets up Bearer token authentication
func WithAPIKeyFromFile(filename string) client.Option {
	return func(c *client.Client) {
		c.SetAuth(func(r *http.Request) {
			apiKeyBytes, err := os.ReadFile(filename)
			if err != nil {
				return // fail silently, let the API call fail with auth error
			}
			apiKey := strings.TrimSpace(string(apiKeyBytes))
			r.Header.Set("Authorization", "Bearer "+apiKey)
		})
	}
}

// WorkflowyClient wraps the generic Client with Workflowy-specific methods
type WorkflowyClient struct {
	*client.Client
	opts []client.Option
}

// NewWorkflowyClient creates a new Workflowy API client
func NewWorkflowyClient(opts ...client.Option) *WorkflowyClient {
	c := client.New("https://workflowy.com/api/v1", opts...)
	return &WorkflowyClient{Client: c, opts: opts}
}

// Item represents a Workflowy item with all its properties
type Item struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Note        *string                `json:"note"`
	Priority    int                    `json:"priority"`
	Data        map[string]interface{} `json:"data"`
	CreatedAt   int64                  `json:"createdAt"`
	ModifiedAt  int64                  `json:"modifiedAt"`
	CompletedAt *int64                 `json:"completedAt"`
	Children    []*Item                `json:"children,omitempty"`
}

// ListChildrenResponse represents the response from list nodes API
type ListChildrenResponse struct {
	Items []*Item `json:"nodes"` // v1 API uses "nodes" field
}

// CreateNodeRequest represents the request payload for nodes-create API
type CreateNodeRequest struct {
	ParentID   string  `json:"parent_id"`
	Name       string  `json:"name"`
	Note       *string `json:"note,omitempty"`
	LayoutMode *string `json:"layoutMode,omitempty"`
	Position   *string `json:"position,omitempty"`
}

// CreateNodeResponse represents the response from nodes-create API
type CreateNodeResponse struct {
	ItemID string `json:"item_id"`
}

// GetItemResponse represents the response from GET /nodes/:id
type GetItemResponse struct {
	Node Item `json:"node"`
}

// GetItem retrieves an item by ID from Workflowy
func (wc *WorkflowyClient) GetItem(ctx context.Context, itemID string) (*Item, error) {
	var resp GetItemResponse
	path := fmt.Sprintf("/nodes/%s", itemID)

	err := wc.Do(ctx, "GET", path, nil, &resp)
	if err != nil {
		return nil, err
	}

	return &resp.Node, nil
}

// ListChildren retrieves direct children of an item from Workflowy
// Use itemID "None" to get root level items
func (wc *WorkflowyClient) ListChildren(ctx context.Context, itemID string) (*ListChildrenResponse, error) {
	path := fmt.Sprintf("/nodes?parent_id=%s", itemID)
	var resp ListChildrenResponse

	err := wc.Do(ctx, "GET", path, nil, &resp)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

// ListChildrenRecursive retrieves children recursively, building a complete tree
// Use itemID "None" to get the entire outline tree
// Uses default depth of 5 levels
func (wc *WorkflowyClient) ListChildrenRecursive(ctx context.Context, itemID string) (*ListChildrenResponse, error) {
	return wc.ListChildrenRecursiveWithDepth(ctx, itemID, 5)
}

// ListChildrenRecursiveWithDepth retrieves children recursively up to specified depth
// Use itemID "None" to get the entire outline tree
// depth parameter controls how many levels deep to fetch (0 = no children, 1 = direct children only, etc.)
func (wc *WorkflowyClient) ListChildrenRecursiveWithDepth(ctx context.Context, itemID string, depth int) (*ListChildrenResponse, error) {
	// If depth is 0, return empty response without making any API calls
	if depth <= 0 {
		return &ListChildrenResponse{Items: []*Item{}}, nil
	}

	resp, err := wc.ListChildren(ctx, itemID)
	if err != nil {
		return nil, err
	}

	// If depth > 1, recursively fetch children for each item
	if depth > 1 {
		for _, item := range resp.Items {
			err := wc.fetchChildrenRecursively(ctx, item, depth-1)
			if err != nil {
				return nil, err
			}
		}
	}

	return resp, nil
}

// fetchChildrenRecursively is a helper function to recursively populate children
// depth parameter controls how many more levels deep to fetch
func (wc *WorkflowyClient) fetchChildrenRecursively(ctx context.Context, item *Item, depth int) error {
	slog.Debug("fetching children recursively", "item_id", item.ID, "depth", depth)

	// Stop recursion if depth is 0 or negative
	if depth <= 0 {
		return nil
	}

	childrenResp, err := wc.ListChildren(ctx, item.ID)
	if err != nil {
		return err
	}

	if len(childrenResp.Items) > 0 {
		item.Children = childrenResp.Items

		// Recursively fetch children for each child, reducing depth by 1
		for _, child := range item.Children {
			err := wc.fetchChildrenRecursively(ctx, child, depth-1)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// CreateNode creates a new node in Workflowy
func (wc *WorkflowyClient) CreateNode(ctx context.Context, req *CreateNodeRequest) (*CreateNodeResponse, error) {
	var resp CreateNodeResponse
	err := wc.Do(ctx, "POST", "/nodes", req, &resp)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

// ExportNode represents a node from the export API with parent_id for tree reconstruction
type ExportNode struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Note        *string                `json:"note"`
	ParentID    *string                `json:"parent_id"`
	Priority    int                    `json:"priority"`
	Completed   bool                   `json:"completed"`
	Data        map[string]interface{} `json:"data"`
	CreatedAt   int64                  `json:"createdAt"`
	ModifiedAt  int64                  `json:"modifiedAt"`
	CompletedAt *int64                 `json:"completedAt"`
}

// ExportNodesResponse represents the response from GET /nodes-export
type ExportNodesResponse struct {
	Nodes []ExportNode `json:"nodes"`
}

// ExportNodes retrieves all nodes from Workflowy (rate limited to 1 req/min)
func (wc *WorkflowyClient) ExportNodes(ctx context.Context) (*ExportNodesResponse, error) {
	var resp ExportNodesResponse
	err := wc.Do(ctx, "GET", "/nodes-export", nil, &resp)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

// ExportNodesWithCache retrieves all nodes using cache when valid
// forceRefresh bypasses cache and fetches fresh data
func (wc *WorkflowyClient) ExportNodesWithCache(ctx context.Context, forceRefresh bool) (*ExportNodesResponse, error) {
	// Try to read cache first
	cachedData, err := cache.ReadExportCache()
	if err != nil {
		slog.Warn("error reading cache, will fetch from API", "error", err)
	}

	// Use cache if valid and not forcing refresh
	if !forceRefresh && cache.IsCacheValid(cachedData) {
		age := cache.GetCacheAge(cachedData)
		slog.Info("using cached export data", "age_seconds", int(age.Seconds()))

		// Unmarshal cached data
		var resp ExportNodesResponse
		if err := json.Unmarshal(cachedData.Data, &resp); err != nil {
			slog.Warn("error unmarshaling cached data, will fetch from API", "error", err)
		} else {
			return &resp, nil
		}
	}

	// Check if cache exists but is too recent (enforce rate limit)
	if cachedData != nil && !forceRefresh {
		age := cache.GetCacheAge(cachedData)
		if age < cache.CacheExpiryDuration {
			remaining := cache.CacheExpiryDuration - age
			return nil, fmt.Errorf("rate limit: must wait %d seconds before next export (use cache or wait)", int(remaining.Seconds()))
		}
	}

	// Fetch fresh data from API
	slog.Info("fetching fresh export data from API")
	resp, err := wc.ExportNodes(ctx)
	if err != nil {
		// If API call fails, try to use stale cache as fallback
		if cachedData != nil {
			age := cache.GetCacheAge(cachedData)
			slog.Warn("API call failed, using stale cache", "age_seconds", int(age.Seconds()))

			var fallbackResp ExportNodesResponse
			if unmarshalErr := json.Unmarshal(cachedData.Data, &fallbackResp); unmarshalErr == nil {
				return &fallbackResp, nil
			}
		}
		return nil, fmt.Errorf("error fetching export data: %w", err)
	}

	// Write to cache
	if err := cache.WriteExportCache(resp); err != nil {
		slog.Warn("error writing cache (continuing anyway)", "error", err)
	}

	return resp, nil
}
