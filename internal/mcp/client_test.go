package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPClient(t *testing.T) {
	tools := []Tool{
		{Name: "search", Description: "search stuff"},
		{Name: "get", Description: "get stuff"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req jsonRPCRequest
		json.NewDecoder(r.Body).Decode(&req)

		var result any
		switch req.Method {
		case "initialize":
			result = initializeResult{
				ProtocolVersion: "2024-11-05",
				ServerInfo:      serverInfo{Name: "test-server", Version: "1.0"},
			}
		case "tools/list":
			result = listToolsResult{Tools: tools}
		case "tools/call":
			result = ToolResult{
				Content: []ContentBlock{{Type: "text", Text: "hello world"}},
			}
		default:
			http.Error(w, "unknown method", http.StatusBadRequest)
			return
		}

		resp := jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
		}
		resp.Result, _ = json.Marshal(result)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewHTTPClient(HTTPConfig{
		URL: server.URL,
		Headers: map[string]string{
			"Authorization": "Bearer test",
		},
	})
	defer client.Close()

	ctx := context.Background()

	if err := client.Initialize(ctx); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	gotTools, err := client.ListTools(ctx)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	if len(gotTools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(gotTools))
	}
	if gotTools[0].Name != "search" {
		t.Errorf("expected search, got %s", gotTools[0].Name)
	}

	result, err := client.CallTool(ctx, "search", map[string]any{"query": "test"})
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	if len(result.Content) != 1 || result.Content[0].Text != "hello world" {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestHTTPClientError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req jsonRPCRequest
		json.NewDecoder(r.Body).Decode(&req)

		resp := jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &jsonRPCError{
				Code:    -32600,
				Message: "invalid request",
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewHTTPClient(HTTPConfig{URL: server.URL})
	ctx := context.Background()

	err := client.Initialize(ctx)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "initialize: jsonrpc error -32600: invalid request" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBaseClientRequestID(t *testing.T) {
	var c baseClient
	id1 := c.nextID()
	id2 := c.nextID()
	id3 := c.nextID()

	if id1 != 1 || id2 != 2 || id3 != 3 {
		t.Errorf("expected sequential IDs, got %d, %d, %d", id1, id2, id3)
	}
}
