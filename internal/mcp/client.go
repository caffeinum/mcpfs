package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
)

type Client interface {
	Initialize(ctx context.Context) error
	ListTools(ctx context.Context) ([]Tool, error)
	CallTool(ctx context.Context, name string, args map[string]any) (*ToolResult, error)
	Close() error
}

type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
}

type ToolResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type jsonRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *jsonRPCError) Error() string {
	return fmt.Sprintf("jsonrpc error %d: %s", e.Code, e.Message)
}

type initializeParams struct {
	ProtocolVersion string           `json:"protocolVersion"`
	Capabilities    clientCaps       `json:"capabilities"`
	ClientInfo      clientInfo       `json:"clientInfo"`
}

type clientCaps struct{}

type clientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type initializeResult struct {
	ProtocolVersion string     `json:"protocolVersion"`
	ServerInfo      serverInfo `json:"serverInfo"`
}

type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

type listToolsResult struct {
	Tools []Tool `json:"tools"`
}

type callToolParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

type baseClient struct {
	reqID atomic.Int64
}

func (c *baseClient) nextID() int64 {
	return c.reqID.Add(1)
}

func (c *baseClient) makeRequest(method string, params any) *jsonRPCRequest {
	return &jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      c.nextID(),
		Method:  method,
		Params:  params,
	}
}

func (c *baseClient) initParams() *initializeParams {
	return &initializeParams{
		ProtocolVersion: "2024-11-05",
		Capabilities:    clientCaps{},
		ClientInfo: clientInfo{
			Name:    "mcpfs",
			Version: "0.1.0",
		},
	}
}
