package workflowy

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	workflowyclient "github.com/mholzen/workflowy/pkg/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecursiveFetchDisabledDoesNotListMirrorChildren(t *testing.T) {
	mirror := testMirror("mirror", "origin")
	client, requests := newRecursiveFetchTestClient(t, func(t *testing.T, request *http.Request) interface{} {
		t.Fatalf("unexpected HTTP request: %s", request.URL.RequestURI())
		return nil
	})

	result, err := client.ListChildrenRecursiveWithOptions(context.Background(), mirror.ID, RecursiveFetchOptions{
		Depth:          3,
		ResolveMirrors: false,
		Operation:      "get",
		RootItem:       mirror,
	})
	require.NoError(t, err)
	assert.Empty(t, result.Response.Items)
	assert.Empty(t, requests.snapshot())
}

func TestRecursiveFetchNilRootRetrievesMirrorBeforeDisabledTraversal(t *testing.T) {
	mirror := testMirror("mirror", "origin")
	client, requests := newRecursiveFetchTestClient(t, func(t *testing.T, request *http.Request) interface{} {
		require.Equal(t, "/nodes/mirror", request.URL.RequestURI())
		return GetItemResponse{Node: *mirror}
	})

	result, err := client.ListChildrenRecursiveWithOptions(context.Background(), mirror.ID, RecursiveFetchOptions{
		Depth:          3,
		ResolveMirrors: false,
		Operation:      "get",
	})
	require.NoError(t, err)
	assert.Empty(t, result.Response.Items)
	assert.Equal(t, []string{"/nodes/mirror"}, requests.snapshot())
}

func TestRecursiveFetchRejectsMismatchedRootItem(t *testing.T) {
	client, requests := newRecursiveFetchTestClient(t, func(t *testing.T, request *http.Request) interface{} {
		t.Fatalf("unexpected HTTP request: %s", request.URL.RequestURI())
		return nil
	})

	result, err := client.ListChildrenRecursiveWithOptions(context.Background(), "requested", RecursiveFetchOptions{
		Depth:          2,
		ResolveMirrors: true,
		Operation:      "get",
		RootItem:       testItem("supplied"),
	})

	assert.Nil(t, result)
	require.EqualError(t, err, `Cannot recursively fetch Workflowy node "requested": supplied root item has ID "supplied"`)
	assert.Empty(t, requests.snapshot())
}

func TestRecursiveFetchDisabledStillListsOrdinaryChildren(t *testing.T) {
	root := testItem("root")
	mirror := testMirror("mirror", "origin")
	client, requests := newRecursiveFetchTestClient(t, func(t *testing.T, request *http.Request) interface{} {
		switch request.URL.RequestURI() {
		case "/nodes?parent_id=root":
			return ListChildrenResponse{Items: []*Item{mirror}}
		default:
			t.Fatalf("unexpected HTTP request: %s", request.URL.RequestURI())
			return nil
		}
	})

	result, err := client.ListChildrenRecursiveWithOptions(context.Background(), root.ID, RecursiveFetchOptions{
		Depth:          3,
		ResolveMirrors: false,
		Operation:      "get",
		RootItem:       root,
	})
	require.NoError(t, err)
	require.Len(t, result.Response.Items, 1)
	assert.Empty(t, result.Response.Items[0].Children)
	assert.Equal(t, []string{"/nodes?parent_id=root"}, requests.snapshot())
}

func TestRecursiveFetchEnabledUsesServerMirrorChildren(t *testing.T) {
	mirror := testMirror("mirror", "origin")
	client, requests := newRecursiveFetchTestClient(t, func(t *testing.T, request *http.Request) interface{} {
		require.Equal(t, "/nodes?parent_id=mirror", request.URL.RequestURI())
		return ListChildrenResponse{Items: []*Item{testItem("origin-child")}}
	})

	result, err := client.ListChildrenRecursiveWithOptions(context.Background(), mirror.ID, RecursiveFetchOptions{
		Depth:          1,
		ResolveMirrors: true,
		Operation:      "get",
		RootItem:       mirror,
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"origin-child"}, itemIDs(result.Response.Items))
	assert.Equal(t, 1, result.Summary.Resolved)
	assert.Equal(t, []string{"/nodes?parent_id=mirror"}, requests.snapshot())
}

func TestRecursiveFetchStopsMirrorCycle(t *testing.T) {
	root := testItem("origin")
	cycle := testMirror("cycle", root.ID)
	client, requests := newRecursiveFetchTestClient(t, func(t *testing.T, request *http.Request) interface{} {
		require.Equal(t, "/nodes?parent_id=origin", request.URL.RequestURI())
		return ListChildrenResponse{Items: []*Item{cycle}}
	})

	result, err := client.ListChildrenRecursiveWithOptions(context.Background(), root.ID, RecursiveFetchOptions{
		Depth:          3,
		ResolveMirrors: true,
		Operation:      "get",
		RootItem:       root,
	})
	require.NoError(t, err)
	require.Len(t, result.Response.Items, 1)
	assert.Empty(t, result.Response.Items[0].Children)
	assert.Equal(t, 1, result.Summary.Cycles)
	assert.Equal(t, []string{"/nodes?parent_id=origin"}, requests.snapshot())
}

func TestRecursiveFetchInternalRootSeedsCyclePath(t *testing.T) {
	root := testItem("origin")
	cycle := testMirror("cycle", root.ID)
	client, requests := newRecursiveFetchTestClient(t, func(t *testing.T, request *http.Request) interface{} {
		switch request.URL.RequestURI() {
		case "/nodes/origin":
			return GetItemResponse{Node: *root}
		case "/nodes?parent_id=origin":
			return ListChildrenResponse{Items: []*Item{cycle}}
		default:
			t.Fatalf("unexpected HTTP request: %s", request.URL.RequestURI())
			return nil
		}
	})

	result, err := client.ListChildrenRecursiveWithOptions(context.Background(), root.ID, RecursiveFetchOptions{
		Depth:          3,
		ResolveMirrors: true,
		Operation:      "get",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Summary.Cycles)
	assert.Equal(t, []string{"/nodes/origin", "/nodes?parent_id=origin"}, requests.snapshot())
}

func TestRecursiveFetchUsesMetadataForBothDeployments(t *testing.T) {
	tests := []struct {
		name   string
		mirror *Item
	}{
		{name: "beta", mirror: testMirror("mirror", "origin")},
		{name: "production backup shape", mirror: &Item{ID: "mirror", Data: map[string]interface{}{
			"mirror": map[string]interface{}{"originalId": "origin"},
		}}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, requests := newRecursiveFetchTestClient(t, func(t *testing.T, request *http.Request) interface{} {
				t.Fatalf("unexpected HTTP request: %s", request.URL.RequestURI())
				return nil
			})

			result, err := client.ListChildrenRecursiveWithOptions(context.Background(), test.mirror.ID, RecursiveFetchOptions{
				Depth:          1,
				ResolveMirrors: false,
				Operation:      "get",
				RootItem:       test.mirror,
			})
			require.NoError(t, err)
			assert.Empty(t, result.Response.Items)
			assert.Empty(t, requests.snapshot())
		})
	}
}

func TestRecursiveFetchReturnsResolutionSummaryAndLogsOneTraversalSummary(t *testing.T) {
	logs := captureResolvedTreeLogs(t, slog.LevelDebug)
	success := testMirror("success", "origin")
	malformed := &Item{ID: "malformed", Data: map[string]interface{}{
		"mirror": map[string]interface{}{"origin_id": false},
	}}
	client, _ := newRecursiveFetchTestClient(t, func(t *testing.T, request *http.Request) interface{} {
		switch request.URL.Query().Get("parent_id") {
		case "None":
			return ListChildrenResponse{Items: []*Item{success, malformed}}
		case "success", "malformed":
			return ListChildrenResponse{}
		default:
			t.Fatalf("unexpected HTTP request: %s", request.URL.RequestURI())
			return nil
		}
	})

	result, err := client.ListChildrenRecursiveWithOptions(context.Background(), "None", RecursiveFetchOptions{
		Depth:          2,
		ResolveMirrors: true,
		Operation:      "list",
	})
	require.NoError(t, err)
	assert.Equal(t, MirrorResolutionSummary{Resolved: 1, MalformedMetadata: 1}, result.Summary)
	assert.Len(t, logs.withMessage("Workflowy recursive mirror traversal completed"), 1)
	malformedLog := logs.requireOne(t, "Workflowy mirror metadata is malformed; using API children")
	assert.Equal(t, "malformed", malformedLog.attrs["mirror_id"])
	assert.Equal(t, "list", malformedLog.attrs["operation"])
	assert.Len(t, logs.withMessage(result.Summary.ThresholdWarning("list", "Workflowy API")), 1)
}

func TestRecursiveFetchLogsContextualCycle(t *testing.T) {
	logs := captureResolvedTreeLogs(t, slog.LevelDebug)
	root := testMirror("entry", "origin")
	client, _ := newRecursiveFetchTestClient(t, func(t *testing.T, request *http.Request) interface{} {
		return ListChildrenResponse{Items: []*Item{testMirror("cycle", "origin")}}
	})

	_, err := client.ListChildrenRecursiveWithOptions(context.Background(), root.ID, RecursiveFetchOptions{
		Depth:          2,
		ResolveMirrors: true,
		Operation:      "get",
		RootItem:       root,
	})
	require.NoError(t, err)
	cycleLog := logs.requireOne(t, "Workflowy mirror cycle stopped")
	assert.Equal(t, "cycle", cycleLog.attrs["mirror_id"])
	assert.Equal(t, "origin", cycleLog.attrs["origin_id"])
	assert.Equal(t, "entry/cycle", cycleLog.attrs["path"])
}

type synchronizedRequests struct {
	mu    sync.Mutex
	paths []string
}

func (requests *synchronizedRequests) append(path string) {
	requests.mu.Lock()
	defer requests.mu.Unlock()
	requests.paths = append(requests.paths, path)
}

func (requests *synchronizedRequests) snapshot() []string {
	requests.mu.Lock()
	defer requests.mu.Unlock()
	return append([]string(nil), requests.paths...)
}

func newRecursiveFetchTestClient(
	t *testing.T,
	response func(*testing.T, *http.Request) interface{},
) (*WorkflowyClient, *synchronizedRequests) {
	t.Helper()
	requests := &synchronizedRequests{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests.append(request.URL.RequestURI())
		writer.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(writer).Encode(response(t, request)))
	}))
	t.Cleanup(server.Close)
	return &WorkflowyClient{Client: workflowyclient.New(server.URL)}, requests
}
