package client

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientDo_LogsHTTPRequestsAtDebugLevel(t *testing.T) {
	output := captureLogs(t, slog.LevelDebug)
	c := newTestClient(http.StatusOK, `{}`)

	for _, method := range []string{"GET", "POST", "DELETE"} {
		output.Reset()

		err := c.Do(context.Background(), method, "/nodes/test-id", nil, nil)

		require.NoError(t, err)
		assert.Contains(t, output.String(), "level=DEBUG")
		assert.Contains(t, output.String(), "msg=\"http request\"")
		assert.Contains(t, output.String(), "method="+method)
		assert.Contains(t, output.String(), "path=/nodes/test-id")
	}
}

func TestClientDo_LogsConfiguredRequestAttributes(t *testing.T) {
	output := captureLogs(t, slog.LevelDebug)
	c := newTestClientWithOptions(http.StatusOK, `{}`, WithLogAttributes(slog.String("api", "beta")))

	err := c.Do(context.Background(), "GET", "/nodes/test-id", nil, nil)

	require.NoError(t, err)
	assert.Contains(t, output.String(), "api=beta")
	assert.Contains(t, output.String(), "path=/nodes/test-id")
}

func TestClientDo_DoesNotLogHTTPRequestsAtInfoLevel(t *testing.T) {
	output := captureLogs(t, slog.LevelInfo)
	c := newTestClient(http.StatusOK, `{}`)

	for _, method := range []string{"GET", "POST", "DELETE"} {
		output.Reset()

		err := c.Do(context.Background(), method, "/nodes/test-id", nil, nil)

		require.NoError(t, err)
		assert.NotContains(t, strings.TrimSpace(output.String()), "msg=\"http request\"")
	}
}

func TestClientDo_LogsAndDecodesSuccessfulResponse(t *testing.T) {
	output := captureLogs(t, slog.LevelDebug)
	c := newTestClient(http.StatusOK, `{"status":"ok"}`)
	var response struct {
		Status string `json:"status"`
	}

	err := c.Do(context.Background(), "GET", "/nodes/test-id", nil, &response)

	require.NoError(t, err)
	assert.Equal(t, "ok", response.Status)
	assert.Contains(t, output.String(), "msg=\"http request\"")
}

func TestClientDo_LogsRequestReturningAPIError(t *testing.T) {
	output := captureLogs(t, slog.LevelDebug)
	c := newTestClient(http.StatusBadRequest, `{"error":"invalid request"}`)

	err := c.Do(context.Background(), "POST", "/nodes/test-id", nil, nil)

	var apiError *APIError
	require.ErrorAs(t, err, &apiError)
	assert.Equal(t, http.StatusBadRequest, apiError.Status)
	assert.Equal(t, `{"error":"invalid request"}`, apiError.Body)
	assert.Contains(t, output.String(), "msg=\"http request\"")
}

func captureLogs(t *testing.T, level slog.Level) *bytes.Buffer {
	t.Helper()

	var output bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&output, &slog.HandlerOptions{Level: level}))
	previous := slog.Default()
	slog.SetDefault(logger)
	t.Cleanup(func() {
		slog.SetDefault(previous)
	})
	return &output
}

func newTestClient(statusCode int, responseBody string) *Client {
	return newTestClientWithOptions(statusCode, responseBody)
}

func newTestClientWithOptions(statusCode int, responseBody string, options ...Option) *Client {
	c := New("https://api.example.com", options...)
	c.http = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: statusCode,
			Body:       io.NopCloser(strings.NewReader(responseBody)),
			Header:     make(http.Header),
		}, nil
	})}
	return c
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
