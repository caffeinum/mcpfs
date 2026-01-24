package pool

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/caffeinum/mcpfs/internal/config"
)

func TestPoolLazyConnection(t *testing.T) {
	server := createMockServer(t)
	defer server.Close()

	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"@test/server": {
				Transport: config.TransportHTTP,
				URL:       server.URL,
			},
		},
	}

	pool := New(PoolConfig{
		Config:      cfg,
		IdleTimeout: 1 * time.Minute,
	})
	defer pool.Close()

	status := pool.GetStatus()
	if len(status) != 0 {
		t.Errorf("expected 0 connections before access, got %d", len(status))
	}

	ctx := context.Background()
	conn, err := pool.GetConnection(ctx, "@test/server")
	if err != nil {
		t.Fatalf("get connection: %v", err)
	}

	status = pool.GetStatus()
	if len(status) != 1 {
		t.Errorf("expected 1 connection after access, got %d", len(status))
	}

	info := status["@test/server"]
	if info.Status != "connected" {
		t.Errorf("expected connected, got %s", info.Status)
	}
	if info.ToolCount != 2 {
		t.Errorf("expected 2 tools, got %d", info.ToolCount)
	}

	tools := conn.GetTools()
	if len(tools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(tools))
	}
}

func TestPoolConnectionReuse(t *testing.T) {
	server := createMockServer(t)
	defer server.Close()

	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"@test/server": {
				Transport: config.TransportHTTP,
				URL:       server.URL,
			},
		},
	}

	pool := New(PoolConfig{Config: cfg})
	defer pool.Close()

	ctx := context.Background()
	conn1, _ := pool.GetConnection(ctx, "@test/server")
	conn2, _ := pool.GetConnection(ctx, "@test/server")

	if conn1 != conn2 {
		t.Error("expected same connection on reuse")
	}
}

func TestPoolServerNotFound(t *testing.T) {
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{},
	}

	pool := New(PoolConfig{Config: cfg})
	defer pool.Close()

	ctx := context.Background()
	_, err := pool.GetConnection(ctx, "@nonexistent/server")
	if err == nil {
		t.Fatal("expected error for nonexistent server")
	}
}

func TestPoolCloseConnection(t *testing.T) {
	server := createMockServer(t)
	defer server.Close()

	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"@test/server": {
				Transport: config.TransportHTTP,
				URL:       server.URL,
			},
		},
	}

	pool := New(PoolConfig{Config: cfg})
	defer pool.Close()

	ctx := context.Background()
	pool.GetConnection(ctx, "@test/server")

	if err := pool.CloseConnection("@test/server"); err != nil {
		t.Fatalf("close connection: %v", err)
	}

	status := pool.GetStatus()
	if len(status) != 0 {
		t.Errorf("expected 0 connections after close, got %d", len(status))
	}
}

func TestPoolCallTool(t *testing.T) {
	server := createMockServer(t)
	defer server.Close()

	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"@test/server": {
				Transport: config.TransportHTTP,
				URL:       server.URL,
			},
		},
	}

	pool := New(PoolConfig{Config: cfg})
	defer pool.Close()

	ctx := context.Background()
	conn, err := pool.GetConnection(ctx, "@test/server")
	if err != nil {
		t.Fatalf("get connection: %v", err)
	}

	result, err := conn.CallTool(ctx, "echo", map[string]any{"text": "hello"})
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}

	if len(result.Content) != 1 || result.Content[0].Text != "echo: hello" {
		t.Errorf("unexpected result: %+v", result)
	}
}

func createMockServer(t *testing.T) *httptest.Server {
	type jsonRPCRequest struct {
		JSONRPC string         `json:"jsonrpc"`
		ID      int64          `json:"id"`
		Method  string         `json:"method"`
		Params  map[string]any `json:"params,omitempty"`
	}
	type jsonRPCResponse struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      int64           `json:"id"`
		Result  json.RawMessage `json:"result,omitempty"`
	}

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req jsonRPCRequest
		json.NewDecoder(r.Body).Decode(&req)

		var result any
		switch req.Method {
		case "initialize":
			result = map[string]any{
				"protocolVersion": "2024-11-05",
				"serverInfo":      map[string]any{"name": "test"},
			}
		case "tools/list":
			result = map[string]any{
				"tools": []map[string]any{
					{"name": "echo", "description": "echoes text"},
					{"name": "ping", "description": "pongs back"},
				},
			}
		case "tools/call":
			args := req.Params["arguments"].(map[string]any)
			text := args["text"].(string)
			result = map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": "echo: " + text},
				},
			}
		}

		resp := jsonRPCResponse{JSONRPC: "2.0", ID: req.ID}
		resp.Result, _ = json.Marshal(result)
		json.NewEncoder(w).Encode(resp)
	}))
}
