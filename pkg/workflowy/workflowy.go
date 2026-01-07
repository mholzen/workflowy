package workflowy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mholzen/workflowy/pkg/cache"
	"github.com/mholzen/workflowy/pkg/client"
	"github.com/mholzen/workflowy/pkg/counter"
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
func WithAPIKeyFromFile(filename string) (client.Option, error) {
	slog.Debug("loading API key", "file", filename)
	apiKeyBytes, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("cannot read API key file -- get an API key from https://workflowy.com/api-key/ (file='%s', error='%w')", filename, err)
	}
	apiKey := strings.TrimSpace(string(apiKeyBytes))

	return func(c *client.Client) {
		c.SetAuth(func(r *http.Request) {
			r.Header.Set("Authorization", "Bearer "+apiKey)
		})
	}, nil
}

// SanitizeNodeID strips the Workflowy URL prefix if present, then removes any
// character that is not hexadecimal or a dash from a node ID
func SanitizeNodeID(id string) string {
	if id == "" || id == "None" {
		return id
	}
	id = strings.TrimPrefix(id, "https://workflowy.com/#/")
	var result strings.Builder
	for _, r := range id {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') || r == '-' {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// IsShortID returns true if the ID is exactly 12 hexadecimal characters
func IsShortID(id string) bool {
	if len(id) != 12 {
		return false
	}
	for _, r := range id {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

// IsTargetKey checks if the given ID matches a known target key
func IsTargetKey(ctx context.Context, client Client, id string) (bool, error) {
	targets, err := client.ListTargets(ctx)
	if err != nil {
		return false, err
	}
	for _, target := range targets.Targets {
		if target.Key == id {
			return true, nil
		}
	}
	return false, nil
}

// ResolveShortID resolves a short ID to its full UUID by searching all nodes
func ResolveShortID(ctx context.Context, client Client, shortID string) (string, error) {
	slog.Info("resolving short ID, loading export tree", "short_id", shortID)

	resp, err := client.ExportNodesWithCache(ctx, false)
	if err != nil {
		return "", fmt.Errorf("cannot load export tree: %w", err)
	}

	var matches []string
	for _, node := range resp.Nodes {
		if strings.HasSuffix(node.ID, shortID) {
			matches = append(matches, node.ID)
		}
	}

	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no node found with short ID: %s", shortID)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("multiple nodes match short ID %s: %v", shortID, matches)
	}
}

// ResolveNodeID resolves target keys, short IDs, and sanitizes full IDs.
// If client is nil, only sanitization is performed.
func ResolveNodeID(ctx context.Context, client Client, id string) (string, error) {
	if id == "" || id == "None" {
		return id, nil
	}

	if client == nil {
		return SanitizeNodeID(id), nil
	}

	isTarget, err := IsTargetKey(ctx, client, id)
	if err != nil {
		return "", fmt.Errorf("cannot check target: %w", err)
	}
	if isTarget {
		return id, nil
	}

	sanitized := SanitizeNodeID(id)

	if IsShortID(sanitized) {
		return ResolveShortID(ctx, client, sanitized)
	}

	return sanitized, nil
}

// ExpandTilde expands a leading ~ to the user's home directory
func ExpandTilde(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[1:])
}

// ResolveAPIKey resolves the API key with precedence:
// 1. Explicit file path (if different from default)
// 2. WORKFLOWY_API_KEY environment variable
// 3. Default file path
func ResolveAPIKey(apiKeyFile, defaultAPIKeyFile string) (client.Option, error) {
	apiKeyFile = ExpandTilde(apiKeyFile)
	defaultAPIKeyFile = ExpandTilde(defaultAPIKeyFile)

	if apiKeyFile != defaultAPIKeyFile {
		slog.Debug("using API key from explicit file", "file", apiKeyFile)
		return WithAPIKeyFromFile(apiKeyFile)
	}

	if envKey := os.Getenv("WORKFLOWY_API_KEY"); envKey != "" {
		slog.Debug("using API key from WORKFLOWY_API_KEY environment variable")
		return WithAPIKey(strings.TrimSpace(envKey)), nil
	}

	slog.Debug("using API key from default file", "file", defaultAPIKeyFile)
	return WithAPIKeyFromFile(defaultAPIKeyFile)
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

// UpdateNodeRequest represents the request body for nodes-update API
type UpdateNodeRequest struct {
	Name       *string `json:"name,omitempty"`
	Note       *string `json:"note,omitempty"`
	LayoutMode *string `json:"layoutMode,omitempty"`
}

// UpdateNodeResponse represents the response from nodes-update API
type UpdateNodeResponse struct {
	Status string `json:"status"`
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

// UpdateNode updates an existing node in Workflowy
func (wc *WorkflowyClient) UpdateNode(ctx context.Context, itemID string, req *UpdateNodeRequest) (*UpdateNodeResponse, error) {
	var resp UpdateNodeResponse
	path := fmt.Sprintf("/nodes/%s", itemID)
	err := wc.Do(ctx, "POST", path, req, &resp)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

func (wc *WorkflowyClient) CompleteNode(ctx context.Context, itemID string) (*UpdateNodeResponse, error) {
	var resp UpdateNodeResponse
	path := fmt.Sprintf("/nodes/%s/complete", itemID)
	err := wc.Do(ctx, "POST", path, nil, &resp)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

func (wc *WorkflowyClient) UncompleteNode(ctx context.Context, itemID string) (*UpdateNodeResponse, error) {
	var resp UpdateNodeResponse
	path := fmt.Sprintf("/nodes/%s/uncomplete", itemID)
	err := wc.Do(ctx, "POST", path, nil, &resp)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

func (wc *WorkflowyClient) DeleteNode(ctx context.Context, itemID string) (*UpdateNodeResponse, error) {
	var resp UpdateNodeResponse
	path := fmt.Sprintf("/nodes/%s", itemID)
	err := wc.Do(ctx, "DELETE", path, nil, &resp)
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

// Target represents a Workflowy target (shortcuts or system targets)
type Target struct {
	Key  string  `json:"key"`
	Type string  `json:"type"`
	Name *string `json:"name"`
}

// ListTargetsResponse represents the response from GET /targets
type ListTargetsResponse struct {
	Targets []Target `json:"targets"`
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

// ListTargets retrieves all available targets (shortcuts and system targets)
func (wc *WorkflowyClient) ListTargets(ctx context.Context) (*ListTargetsResponse, error) {
	var resp ListTargetsResponse
	err := wc.Do(ctx, "GET", "/targets", nil, &resp)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

// BackupNode represents a node from a Workflowy backup file
// Backup files use different field names than the API (nm, ch, ct, lm)
type BackupNode struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"nm"`
	Note       *string                `json:"no,omitempty"`
	Children   []BackupNode           `json:"ch,omitempty"`
	CreatedAt  int64                  `json:"ct,omitempty"`
	ModifiedAt int64                  `json:"lm,omitempty"`
	Completed  *int64                 `json:"cp,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// BackupNodeToItem converts a BackupNode to an Item recursively
func BackupNodeToItem(node BackupNode) *Item {
	item := &Item{
		ID:         node.ID,
		Name:       node.Name,
		Note:       node.Note,
		Priority:   0, // Backup files don't have priority, use position in children array
		Data:       node.Metadata,
		CreatedAt:  node.CreatedAt,
		ModifiedAt: node.ModifiedAt,
		Children:   make([]*Item, len(node.Children)),
	}

	if node.Completed != nil {
		item.CompletedAt = node.Completed
	}

	// Recursively convert children
	for i, child := range node.Children {
		item.Children[i] = BackupNodeToItem(child)
	}

	return item
}

// ReadBackupFile reads and parses a Workflowy backup file
func ReadBackupFile(filename string) ([]*Item, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("cannot open backup file: %w", err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("cannot read backup file: %w", err)
	}

	var backupNodes []BackupNode
	if err := json.Unmarshal(data, &backupNodes); err != nil {
		return nil, fmt.Errorf("cannot parse backup file: %w", err)
	}

	// Convert backup nodes to Items
	items := make([]*Item, len(backupNodes))
	for i, node := range backupNodes {
		items[i] = BackupNodeToItem(node)
	}

	return items, nil
}

// ReadLatestBackup reads the most recent backup file from Dropbox folder
func ReadLatestBackup() ([]*Item, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("could not get home directory: %w", err)
	}

	dropboxPath := filepath.Join(homeDir, "Dropbox", "Apps", "Workflowy", "Data")

	files, err := filepath.Glob(filepath.Join(dropboxPath, "*.workflowy.backup"))
	if err != nil {
		return nil, fmt.Errorf("cannot search for backup files: %w", err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no backup files found in %s", dropboxPath)
	}

	// Find the most recent file
	var latest string
	var latestTime int64
	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}
		if info.ModTime().Unix() > latestTime {
			latestTime = info.ModTime().Unix()
			latest = file
		}
	}

	slog.Debug("reading latest backup file", "file", filepath.Base(latest))
	return ReadBackupFile(latest)
}

// ExportNodeToItem converts an ExportNode to an Item
func ExportNodeToItem(node ExportNode) *Item {
	return &Item{
		ID:          node.ID,
		Name:        node.Name,
		Note:        node.Note,
		Priority:    node.Priority,
		Data:        node.Data,
		CreatedAt:   node.CreatedAt,
		ModifiedAt:  node.ModifiedAt,
		CompletedAt: node.CompletedAt,
		Children:    nil, // Will be populated during tree building
	}
}

// BuildTreeFromExport reconstructs a tree structure from flat export nodes
// Returns a root Item containing all top-level nodes as children
func BuildTreeFromExport(nodes []ExportNode) *Item {
	// Create a map of ID -> Item for quick lookup
	itemMap := make(map[string]*Item)

	// First pass: convert all nodes to Items
	for _, node := range nodes {
		item := ExportNodeToItem(node)
		itemMap[node.ID] = item
	}

	// Create a virtual root node to hold top-level items
	root := &Item{
		ID:       "root",
		Name:     "Root",
		Children: []*Item{},
	}

	// Second pass: build parent-child relationships
	for _, node := range nodes {
		item := itemMap[node.ID]

		if node.ParentID == nil {
			// Top-level node (no parent)
			root.Children = append(root.Children, item)
		} else {
			// Has a parent - add to parent's children
			parent, exists := itemMap[*node.ParentID]
			if exists {
				if parent.Children == nil {
					parent.Children = []*Item{}
				}
				parent.Children = append(parent.Children, item)
			} else {
				// Parent not found, treat as top-level
				slog.Warn("parent not found, treating as top-level", "node_id", node.ID, "parent_id", *node.ParentID)
				root.Children = append(root.Children, item)
			}
		}
	}

	// Third pass: sort all children by priority
	sortItemsByPriorityRecursive(root)

	return root
}

// sortItemsByPriorityRecursive sorts children by priority recursively
func sortItemsByPriorityRecursive(item *Item) {
	if len(item.Children) == 0 {
		return
	}

	// Sort immediate children
	sort.Slice(item.Children, func(i, j int) bool {
		return item.Children[i].Priority < item.Children[j].Priority
	})

	// Recursively sort grandchildren
	for _, child := range item.Children {
		sortItemsByPriorityRecursive(child)
	}
}

// ExportNodesWithCache retrieves all nodes using cache when valid
// forceRefresh bypasses cache and fetches fresh data
func (wc *WorkflowyClient) ExportNodesWithCache(ctx context.Context, forceRefresh bool) (*ExportNodesResponse, error) {
	// Try to read cache first
	cachedData, err := cache.ReadExportCache()
	if err != nil {
		slog.Warn("cannot read cache, will fetch from API", "error", err)
	}

	// Use cache if valid and not forcing refresh
	if !forceRefresh && cache.IsCacheValid(cachedData) {
		age := cache.GetCacheAge(cachedData)
		slog.Info("using cached export data", "age_seconds", int(age.Seconds()))

		// Unmarshal cached data
		var resp ExportNodesResponse
		if err := json.Unmarshal(cachedData.Data, &resp); err != nil {
			slog.Warn("cannot unmarshal cached data, will fetch from API", "error", err)
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
		return nil, fmt.Errorf("cannot fetch export data: %w", err)
	}

	// Write to cache
	if err := cache.WriteExportCache(resp); err != nil {
		slog.Warn("cannot write cache (continuing anyway)", "error", err)
	}

	return resp, nil
}

// ItemNode wraps Item to implement counter.TreeProvider interface
type ItemNode struct {
	item     *Item
	children []*ItemNode
}

// NewItemNode creates an ItemNode from an Item recursively
func NewItemNode(item *Item) *ItemNode {
	node := &ItemNode{
		item:     item,
		children: make([]*ItemNode, len(item.Children)),
	}
	for i, child := range item.Children {
		node.children[i] = NewItemNode(child)
	}
	return node
}

// Node implements counter.TreeProvider
func (n *ItemNode) Node() *ItemNode {
	return n
}

// Item returns the underlying Item
func (n *ItemNode) Item() *Item {
	return n.item
}

// Name returns the item's name
func (n *ItemNode) Name() string {
	return n.item.Name
}

// Children implements counter.TreeProvider
func (n *ItemNode) Children() iter.Seq[counter.TreeProvider[*ItemNode]] {
	return iter.Seq[counter.TreeProvider[*ItemNode]](func(yield func(counter.TreeProvider[*ItemNode]) bool) {
		for _, child := range n.children {
			if !yield(child) {
				break
			}
		}
	})
}

// String implements fmt.Stringer
func (n *ItemNode) String() string {
	url := n.ExternalURL()
	if url == "" {
		return n.item.Name
	}
	return fmt.Sprintf("[%s](%s)", n.item.Name, url)
}

// ExternalURL returns the external Workflowy URL for this item
func (n *ItemNode) ExternalURL() string {
	if n.item.ID == "" {
		return ""
	}
	return fmt.Sprintf("https://workflowy.com/#/%s", n.item.ID)
}

// InternalURL returns the internal Workflowy URL for this item
func (n *ItemNode) InternalURL() string {
	if n.item.ID == "" {
		return ""
	}
	return fmt.Sprintf("workflowy://#/%s", n.item.ID)
}

// MarshalJSON implements json.Marshaler
func (n *ItemNode) MarshalJSON() ([]byte, error) {
	type Alias ItemNode
	aux := struct {
		Node string `json:"node"`
		*Alias
	}{
		Node:  n.String(),
		Alias: (*Alias)(n),
	}
	return json.Marshal(aux)
}

// Descendants is a type alias for descendant tree count
type Descendants = *counter.DescendantTreeCount[**ItemNode]

// CountDescendants counts descendants in a tree and returns filtered, sorted results
func CountDescendants(item *Item, threshold float64) Descendants {
	node := NewItemNode(item)
	descendantTreeCount := counter.CountDescendantTree(node)
	counter.CalculateRatioToRoot(descendantTreeCount)
	descendantTreeCount = counter.FilterDescendantTree(descendantTreeCount, threshold)
	descendantTreeCount = counter.SortDescendantTree(descendantTreeCount)
	return descendantTreeCount
}

// NodeWithTimestamps combines a descendant count node with timestamp information
type NodeWithTimestamps struct {
	Count      Descendants
	Item       *Item
	CreatedAt  int64
	ModifiedAt int64
}

// CollectNodesWithTimestamps traverses the tree and collects all nodes with their timestamps
func CollectNodesWithTimestamps(root Descendants) []*NodeWithTimestamps {
	allNodes := counter.CollectAllNodes(root)
	result := make([]*NodeWithTimestamps, len(allNodes))

	for i, node := range allNodes {
		nodeValue := node.NodeValue()
		item := (**nodeValue).Item()
		result[i] = &NodeWithTimestamps{
			Count:      node,
			Item:       item,
			CreatedAt:  item.CreatedAt,
			ModifiedAt: item.ModifiedAt,
		}
	}

	return result
}

// ChildrenCountRankable implements ranking by children count
type ChildrenCountRankable struct {
	Node *NodeWithTimestamps
}

func (r *ChildrenCountRankable) GetValue() fmt.Stringer {
	nodeValue := r.Node.Count.NodeValue()
	return *nodeValue
}

func (r *ChildrenCountRankable) GetRankingValue() int {
	return r.Node.Count.ChildrenCount
}

func (r ChildrenCountRankable) String() string {
	nodeValue := r.Node.Count.NodeValue()
	return fmt.Sprintf("%d: %v", r.Node.Count.ChildrenCount, *nodeValue)
}

// RankByChildrenCount ranks nodes by their immediate children count
func RankByChildrenCount(nodes []*NodeWithTimestamps, topN int) []ChildrenCountRankable {
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Count.ChildrenCount > nodes[j].Count.ChildrenCount
	})

	limit := len(nodes)
	if topN > 0 && topN < limit {
		limit = topN
	}

	result := make([]ChildrenCountRankable, limit)
	for i := 0; i < limit; i++ {
		result[i] = ChildrenCountRankable{Node: nodes[i]}
	}

	return result
}

// TimestampRankable implements ranking by timestamp with formatting
type TimestampRankable struct {
	Node        *NodeWithTimestamps
	UseModified bool
}

func (r *TimestampRankable) GetValue() fmt.Stringer {
	nodeValue := r.Node.Count.NodeValue()
	return *nodeValue
}

func (r *TimestampRankable) GetRankingValue() int {
	if r.UseModified {
		return int(r.Node.ModifiedAt)
	}
	return int(r.Node.CreatedAt)
}

func (r TimestampRankable) String() string {
	nodeValue := r.Node.Count.NodeValue()
	var timestamp int64
	if r.UseModified {
		timestamp = r.Node.ModifiedAt
	} else {
		timestamp = r.Node.CreatedAt
	}
	if timestamp == 0 {
		return fmt.Sprintf("(no date): %v", *nodeValue)
	}
	date := formatTimestamp(timestamp)
	return fmt.Sprintf("%s: %v", date, *nodeValue)
}

// RankByCreated ranks nodes by creation date (oldest first)
func RankByCreated(nodes []*NodeWithTimestamps, topN int) []TimestampRankable {
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].CreatedAt < nodes[j].CreatedAt
	})

	limit := len(nodes)
	if topN > 0 && topN < limit {
		limit = topN
	}

	result := make([]TimestampRankable, limit)
	for i := 0; i < limit; i++ {
		result[i] = TimestampRankable{Node: nodes[i], UseModified: false}
	}

	return result
}

// RankByModified ranks nodes by modification date (oldest first)
func RankByModified(nodes []*NodeWithTimestamps, topN int) []TimestampRankable {
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].ModifiedAt < nodes[j].ModifiedAt
	})

	limit := len(nodes)
	if topN > 0 && topN < limit {
		limit = topN
	}

	result := make([]TimestampRankable, limit)
	for i := 0; i < limit; i++ {
		result[i] = TimestampRankable{Node: nodes[i], UseModified: true}
	}

	return result
}

func formatTimestamp(timestamp int64) string {
	if timestamp == 0 {
		return "no date"
	}
	t := time.Unix(timestamp, 0)
	return t.Format("2006-01-02 15:04:05")
}
