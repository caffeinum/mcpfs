package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

type HTTPClient struct {
	baseClient
	url     string
	headers map[string]string
	client  *http.Client
	mu      sync.Mutex
}

type HTTPConfig struct {
	URL     string
	Headers map[string]string
	Timeout time.Duration
}

func NewHTTPClient(cfg HTTPConfig) *HTTPClient {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &HTTPClient{
		url:     cfg.URL,
		headers: cfg.Headers,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *HTTPClient) send(ctx context.Context, req *jsonRPCRequest) (*jsonRPCResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	for k, v := range c.headers {
		httpReq.Header.Set(k, v)
	}

	httpResp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("http %d: %s", httpResp.StatusCode, string(body))
	}

	respData, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var resp jsonRPCResponse
	if err := json.Unmarshal(respData, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if resp.Error != nil {
		return nil, resp.Error
	}

	return &resp, nil
}

func (c *HTTPClient) Initialize(ctx context.Context) error {
	req := c.makeRequest("initialize", c.initParams())
	resp, err := c.send(ctx, req)
	if err != nil {
		return fmt.Errorf("initialize: %w", err)
	}

	var result initializeResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse initialize result: %w", err)
	}

	return nil
}

func (c *HTTPClient) ListTools(ctx context.Context) ([]Tool, error) {
	req := c.makeRequest("tools/list", nil)
	resp, err := c.send(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("list tools: %w", err)
	}

	var result listToolsResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("parse tools list: %w", err)
	}

	return result.Tools, nil
}

func (c *HTTPClient) CallTool(ctx context.Context, name string, args map[string]any) (*ToolResult, error) {
	params := callToolParams{
		Name:      name,
		Arguments: args,
	}
	req := c.makeRequest("tools/call", params)
	resp, err := c.send(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("call tool: %w", err)
	}

	var result ToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("parse tool result: %w", err)
	}

	return &result, nil
}

func (c *HTTPClient) Close() error {
	return nil
}
