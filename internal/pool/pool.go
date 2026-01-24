package pool

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/caffeinum/mcpfs/internal/config"
	"github.com/caffeinum/mcpfs/internal/mcp"
)

type Pool struct {
	cfg         *config.Config
	connections map[string]*Connection
	mu          sync.RWMutex
	idleTimeout time.Duration
	stopChan    chan struct{}
	wg          sync.WaitGroup
}

type Connection struct {
	Name       string
	Client     mcp.Client
	Tools      []mcp.Tool
	LastAccess time.Time
	Status     ConnectionStatus
	Error      error
	mu         sync.RWMutex
}

type ConnectionStatus int

const (
	StatusDisconnected ConnectionStatus = iota
	StatusConnecting
	StatusConnected
	StatusError
)

func (s ConnectionStatus) String() string {
	switch s {
	case StatusDisconnected:
		return "disconnected"
	case StatusConnecting:
		return "connecting"
	case StatusConnected:
		return "connected"
	case StatusError:
		return "error"
	default:
		return "unknown"
	}
}

type PoolConfig struct {
	Config      *config.Config
	IdleTimeout time.Duration
}

func New(pcfg PoolConfig) *Pool {
	if pcfg.IdleTimeout == 0 {
		pcfg.IdleTimeout = 5 * time.Minute
	}

	p := &Pool{
		cfg:         pcfg.Config,
		connections: make(map[string]*Connection),
		idleTimeout: pcfg.IdleTimeout,
		stopChan:    make(chan struct{}),
	}

	p.wg.Add(1)
	go p.idleReaper()

	return p
}

func (p *Pool) GetConnection(ctx context.Context, serverName string) (*Connection, error) {
	p.mu.Lock()
	conn, exists := p.connections[serverName]
	if !exists {
		conn = &Connection{
			Name:   serverName,
			Status: StatusDisconnected,
		}
		p.connections[serverName] = conn
	}
	p.mu.Unlock()

	conn.mu.Lock()
	defer conn.mu.Unlock()

	conn.LastAccess = time.Now()

	if conn.Status == StatusConnected && conn.Client != nil {
		return conn, nil
	}

	if conn.Status == StatusError {
		conn.Status = StatusDisconnected
		conn.Error = nil
	}

	conn.Status = StatusConnecting
	client, err := p.createClient(serverName)
	if err != nil {
		conn.Status = StatusError
		conn.Error = err
		return nil, err
	}

	if err := client.Initialize(ctx); err != nil {
		client.Close()
		conn.Status = StatusError
		conn.Error = err
		return nil, fmt.Errorf("initialize: %w", err)
	}

	tools, err := client.ListTools(ctx)
	if err != nil {
		client.Close()
		conn.Status = StatusError
		conn.Error = err
		return nil, fmt.Errorf("list tools: %w", err)
	}

	conn.Client = client
	conn.Tools = tools
	conn.Status = StatusConnected
	conn.Error = nil

	return conn, nil
}

func (p *Pool) createClient(serverName string) (mcp.Client, error) {
	srv, ok := p.cfg.GetServer(serverName)
	if !ok {
		return nil, fmt.Errorf("server not found: %s", serverName)
	}

	auth, _ := config.LoadAuth(p.cfg.Dir(), serverName)

	switch srv.Transport {
	case config.TransportStdio:
		env := os.Environ()
		for k, v := range srv.ResolveEnv(auth) {
			env = append(env, k+"="+v)
		}
		return mcp.NewStdioClient(mcp.StdioConfig{
			Command: srv.Command,
			Args:    srv.Args,
			Env:     env,
		})

	case config.TransportHTTP:
		return mcp.NewHTTPClient(mcp.HTTPConfig{
			URL:     srv.URL,
			Headers: srv.ResolveHeaders(auth),
		}), nil

	default:
		return nil, fmt.Errorf("unknown transport: %s", srv.Transport)
	}
}

func (p *Pool) GetStatus() map[string]*ConnectionInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make(map[string]*ConnectionInfo)
	for name, conn := range p.connections {
		conn.mu.RLock()
		info := &ConnectionInfo{
			Name:       name,
			Status:     conn.Status.String(),
			ToolCount:  len(conn.Tools),
			LastAccess: conn.LastAccess,
		}
		if conn.Error != nil {
			info.Error = conn.Error.Error()
		}
		conn.mu.RUnlock()
		result[name] = info
	}
	return result
}

type ConnectionInfo struct {
	Name       string
	Status     string
	ToolCount  int
	LastAccess time.Time
	Error      string
}

func (p *Pool) CloseConnection(serverName string) error {
	p.mu.Lock()
	conn, exists := p.connections[serverName]
	if exists {
		delete(p.connections, serverName)
	}
	p.mu.Unlock()

	if !exists {
		return nil
	}

	conn.mu.Lock()
	defer conn.mu.Unlock()

	if conn.Client != nil {
		conn.Client.Close()
		conn.Client = nil
	}
	conn.Status = StatusDisconnected
	return nil
}

func (p *Pool) Close() error {
	close(p.stopChan)
	p.wg.Wait()

	p.mu.Lock()
	defer p.mu.Unlock()

	for _, conn := range p.connections {
		conn.mu.Lock()
		if conn.Client != nil {
			conn.Client.Close()
		}
		conn.mu.Unlock()
	}
	p.connections = nil

	return nil
}

func (p *Pool) idleReaper() {
	defer p.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopChan:
			return
		case <-ticker.C:
			p.reapIdle()
		}
	}
}

func (p *Pool) reapIdle() {
	p.mu.Lock()
	var toClose []string
	now := time.Now()

	for name, conn := range p.connections {
		conn.mu.RLock()
		if conn.Status == StatusConnected && now.Sub(conn.LastAccess) > p.idleTimeout {
			toClose = append(toClose, name)
		}
		conn.mu.RUnlock()
	}
	p.mu.Unlock()

	for _, name := range toClose {
		p.CloseConnection(name)
	}
}

func (c *Connection) GetTools() []mcp.Tool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Tools
}

func (c *Connection) CallTool(ctx context.Context, name string, args map[string]any) (*mcp.ToolResult, error) {
	c.mu.Lock()
	c.LastAccess = time.Now()
	client := c.Client
	c.mu.Unlock()

	if client == nil {
		return nil, fmt.Errorf("not connected")
	}

	return client.CallTool(ctx, name, args)
}
