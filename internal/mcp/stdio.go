package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
)

type StdioClient struct {
	baseClient
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	mu     sync.Mutex
	closed bool
}

type StdioConfig struct {
	Command string
	Args    []string
	Env     []string
}

func NewStdioClient(cfg StdioConfig) (*StdioClient, error) {
	cmd := exec.Command(cfg.Command, cfg.Args...)
	if len(cfg.Env) > 0 {
		cmd.Env = cfg.Env
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		stdin.Close()
		return nil, fmt.Errorf("start process: %w", err)
	}

	return &StdioClient{
		cmd:    cmd,
		stdin:  stdin,
		stdout: bufio.NewReader(stdout),
	}, nil
}

func (c *StdioClient) send(req *jsonRPCRequest) (*jsonRPCResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil, fmt.Errorf("client closed")
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	if _, err := c.stdin.Write(append(data, '\n')); err != nil {
		return nil, fmt.Errorf("write request: %w", err)
	}

	line, err := c.stdout.ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var resp jsonRPCResponse
	if err := json.Unmarshal(line, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if resp.Error != nil {
		return nil, resp.Error
	}

	return &resp, nil
}

func (c *StdioClient) Initialize(ctx context.Context) error {
	req := c.makeRequest("initialize", c.initParams())
	resp, err := c.send(req)
	if err != nil {
		return fmt.Errorf("initialize: %w", err)
	}

	var result initializeResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse initialize result: %w", err)
	}

	notify := map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}
	data, _ := json.Marshal(notify)
	c.mu.Lock()
	c.stdin.Write(append(data, '\n'))
	c.mu.Unlock()

	return nil
}

func (c *StdioClient) ListTools(ctx context.Context) ([]Tool, error) {
	req := c.makeRequest("tools/list", nil)
	resp, err := c.send(req)
	if err != nil {
		return nil, fmt.Errorf("list tools: %w", err)
	}

	var result listToolsResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("parse tools list: %w", err)
	}

	return result.Tools, nil
}

func (c *StdioClient) CallTool(ctx context.Context, name string, args map[string]any) (*ToolResult, error) {
	params := callToolParams{
		Name:      name,
		Arguments: args,
	}
	req := c.makeRequest("tools/call", params)
	resp, err := c.send(req)
	if err != nil {
		return nil, fmt.Errorf("call tool: %w", err)
	}

	var result ToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("parse tool result: %w", err)
	}

	return &result, nil
}

func (c *StdioClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true

	c.stdin.Close()
	return c.cmd.Wait()
}
