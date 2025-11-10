package workflowy

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strings"

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
}

// NewWorkflowyClient creates a new Workflowy API client
func NewWorkflowyClient(opts ...client.Option) *WorkflowyClient {
	c := client.New("https://beta.workflowy.com/api/beta", opts...)
	return &WorkflowyClient{Client: c}
}

// GetItemRequest represents the request payload for get-item API
type GetItemRequest struct {
	ItemID string `json:"item_id"`
}

// GetItemResponse represents the response from get-item API
type GetItemResponse struct {
	Item Item `json:"item"`
}

// ListChildrenRequest represents the request payload for list-children API
type ListChildrenRequest struct {
	ItemID string `json:"item_id"`
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

// ListChildrenResponse represents the response from list-children API
type ListChildrenResponse struct {
	Items []*Item `json:"items"`
}

// GetItem retrieves an item by ID from Workflowy
func (wc *WorkflowyClient) GetItem(ctx context.Context, itemID string) (*Item, error) {
	req := GetItemRequest{ItemID: itemID}
	var resp GetItemResponse

	err := wc.Do(ctx, "POST", "/get-item/", req, &resp)
	if err != nil {
		return nil, err
	}

	return &resp.Item, nil
}

// ListChildren retrieves direct children of an item from Workflowy
// Use itemID "None" to get root level items
func (wc *WorkflowyClient) ListChildren(ctx context.Context, itemID string) (*ListChildrenResponse, error) {
	req := ListChildrenRequest{ItemID: itemID}
	var resp ListChildrenResponse

	err := wc.Do(ctx, "POST", "/list-children/", req, &resp)
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
