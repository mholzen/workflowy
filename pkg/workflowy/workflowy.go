package workflowy

import (
	"context"
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
type GetItemResponse map[string]interface{}

// GetItem retrieves an item by ID from Workflowy
func (wc *WorkflowyClient) GetItem(ctx context.Context, itemID string) (*GetItemResponse, error) {
	req := GetItemRequest{ItemID: itemID}
	var resp GetItemResponse

	err := wc.Do(ctx, "POST", "/get-item/", req, &resp)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}
