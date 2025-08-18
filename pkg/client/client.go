package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	http    *http.Client
	auth    func(r *http.Request) // injects auth headers
}

// SetAuth allows setting the auth function after client creation
func (c *Client) SetAuth(authFunc func(r *http.Request)) {
	c.auth = authFunc
}

type Option func(*Client)

func New(base string, opts ...Option) *Client {
	c := &Client{
		baseURL: strings.TrimRight(base, "/"),
		http:    &http.Client{Timeout: 10 * time.Second}, // always set timeouts
		auth:    func(*http.Request) {},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *Client) Do(ctx context.Context, method, path string, in any, out any) error {
	u := c.baseURL + path

	var body io.ReadWriter
	if in != nil {
		buf := new(bytes.Buffer)
		if err := json.NewEncoder(buf).Encode(in); err != nil {
			return fmt.Errorf("encode: %w", err)
		}
		body = buf
	}
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	c.auth(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
		// include Retry-After for backoff decisions
		return &APIError{Status: resp.StatusCode, Body: string(b), RetryAfter: resp.Header.Get("Retry-After")}
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

type APIError struct {
	Status     int
	Body       string
	RetryAfter string
}

func (e *APIError) Error() string { return fmt.Sprintf("api %d: %s", e.Status, e.Body) }
