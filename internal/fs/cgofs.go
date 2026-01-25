package fs

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	"github.com/winfsp/cgofuse/fuse"

	"github.com/caffeinum/mcpfs/internal/config"
	"github.com/caffeinum/mcpfs/internal/mcp"
	"github.com/caffeinum/mcpfs/internal/pool"
)

type CgoFS struct {
	fuse.FileSystemBase
	cfg    *config.Config
	pool   *pool.Pool
	mu     sync.RWMutex
	results map[string]*mcp.ToolResult // path -> result cache
}

func NewCgoFS(cfg *config.Config, p *pool.Pool) *CgoFS {
	return &CgoFS{
		cfg:     cfg,
		pool:    p,
		results: make(map[string]*mcp.ToolResult),
	}
}

func (fs *CgoFS) Getattr(path string, stat *fuse.Stat_t, fh uint64) int {
	parts := splitPath(path)

	switch len(parts) {
	case 0: // root
		stat.Mode = fuse.S_IFDIR | 0755
		return 0

	case 1: // scope or .config
		name := parts[0]
		if name == ".config" || fs.hasScope(name) {
			stat.Mode = fuse.S_IFDIR | 0755
			return 0
		}

	case 2: // server dir or config files
		if parts[0] == ".config" {
			if parts[1] == "servers.json" {
				data, _ := config.MarshalServers(fs.cfg.Servers)
				stat.Mode = fuse.S_IFREG | 0644
				stat.Size = int64(len(data))
				return 0
			}
		} else {
			serverName := parts[0] + "/" + parts[1]
			if _, ok := fs.cfg.Servers[serverName]; ok {
				stat.Mode = fuse.S_IFDIR | 0755
				return 0
			}
		}

	case 3: // .status, .schema, or tool dir
		serverName := parts[0] + "/" + parts[1]
		if _, ok := fs.cfg.Servers[serverName]; !ok {
			return -fuse.ENOENT
		}

		name := parts[2]
		if name == ".status" || name == ".schema" {
			stat.Mode = fuse.S_IFREG | 0444
			return 0
		}

		// check if it's a tool
		conn, err := fs.pool.GetConnection(context.Background(), serverName)
		if err != nil {
			return -fuse.EIO
		}
		for _, tool := range conn.GetTools() {
			if tool.Name == name {
				stat.Mode = fuse.S_IFDIR | 0755
				return 0
			}
		}

	case 4: // tool files: .schema, .call, .result
		serverName := parts[0] + "/" + parts[1]
		toolName := parts[2]
		fileName := parts[3]

		if fileName == ".schema" || fileName == ".result" {
			stat.Mode = fuse.S_IFREG | 0444
			return 0
		}
		if fileName == ".call" {
			stat.Mode = fuse.S_IFREG | 0666
			return 0
		}
		_ = serverName
		_ = toolName
	}

	return -fuse.ENOENT
}

func (fs *CgoFS) Readdir(path string, fill func(name string, stat *fuse.Stat_t, ofst int64) bool, ofst int64, fh uint64) int {
	fill(".", nil, 0)
	fill("..", nil, 0)

	parts := splitPath(path)

	switch len(parts) {
	case 0: // root
		fill(".config", nil, 0)
		scopes := make(map[string]bool)
		for name := range fs.cfg.Servers {
			scope, _ := config.ParseServerName(name)
			if scope != "" && !scopes[scope] {
				scopes[scope] = true
				fill(scope, nil, 0)
			}
		}
		// also add servers without scope
		for name := range fs.cfg.Servers {
			scope, _ := config.ParseServerName(name)
			if scope == "" {
				fill(name, nil, 0)
			}
		}

	case 1: // scope or .config
		if parts[0] == ".config" {
			fill("servers.json", nil, 0)
		} else {
			scope := parts[0]
			for name := range fs.cfg.Servers {
				s, server := config.ParseServerName(name)
				if s == scope {
					fill(server, nil, 0)
				}
			}
		}

	case 2: // server dir
		if parts[0] == ".config" {
			return 0
		}
		serverName := parts[0] + "/" + parts[1]
		fill(".status", nil, 0)
		fill(".schema", nil, 0)

		conn, err := fs.pool.GetConnection(context.Background(), serverName)
		if err == nil {
			for _, tool := range conn.GetTools() {
				fill(tool.Name, nil, 0)
			}
		}

	case 3: // tool dir
		fill(".schema", nil, 0)
		fill(".call", nil, 0)
		fill(".result", nil, 0)
	}

	return 0
}

func (fs *CgoFS) Open(path string, flags int) (int, uint64) {
	parts := splitPath(path)
	if len(parts) < 2 {
		return -fuse.ENOENT, 0
	}
	return 0, 0
}

func (fs *CgoFS) Read(path string, buff []byte, ofst int64, fh uint64) int {
	data := fs.getFileContent(path)
	if data == nil {
		return -fuse.ENOENT
	}

	if ofst >= int64(len(data)) {
		return 0
	}

	n := copy(buff, data[ofst:])
	return n
}

func (fs *CgoFS) Write(path string, buff []byte, ofst int64, fh uint64) int {
	parts := splitPath(path)
	if len(parts) != 4 || parts[3] != ".call" {
		return -fuse.EACCES
	}

	serverName := parts[0] + "/" + parts[1]
	toolName := parts[2]

	var args map[string]any
	if err := json.Unmarshal(buff, &args); err != nil {
		return -fuse.EINVAL
	}

	conn, err := fs.pool.GetConnection(context.Background(), serverName)
	if err != nil {
		return -fuse.EIO
	}

	result, err := conn.CallTool(context.Background(), toolName, args)
	if err != nil {
		return -fuse.EIO
	}

	fs.mu.Lock()
	fs.results[path] = result
	fs.mu.Unlock()

	return len(buff)
}

func (fs *CgoFS) Truncate(path string, size int64, fh uint64) int {
	return 0
}

func (fs *CgoFS) getFileContent(path string) []byte {
	parts := splitPath(path)

	switch len(parts) {
	case 2: // .config/servers.json
		if parts[0] == ".config" && parts[1] == "servers.json" {
			data, _ := config.MarshalServers(fs.cfg.Servers)
			return data
		}

	case 3: // .status or .schema
		serverName := parts[0] + "/" + parts[1]
		fileName := parts[2]

		if fileName == ".status" {
			status := fs.pool.GetStatus()
			info, ok := status[serverName]
			if !ok {
				return []byte("disconnected\n")
			}
			return []byte("status: " + info.Status + "\n")
		}

		if fileName == ".schema" {
			conn, err := fs.pool.GetConnection(context.Background(), serverName)
			if err != nil {
				return []byte("error: " + err.Error() + "\n")
			}
			tools := conn.GetTools()
			data, _ := json.MarshalIndent(tools, "", "  ")
			return append(data, '\n')
		}

	case 4: // tool files
		serverName := parts[0] + "/" + parts[1]
		toolName := parts[2]
		fileName := parts[3]

		if fileName == ".schema" {
			conn, err := fs.pool.GetConnection(context.Background(), serverName)
			if err != nil {
				return []byte("error: " + err.Error() + "\n")
			}
			for _, tool := range conn.GetTools() {
				if tool.Name == toolName {
					data, _ := json.MarshalIndent(tool, "", "  ")
					return append(data, '\n')
				}
			}
		}

		if fileName == ".call" {
			conn, err := fs.pool.GetConnection(context.Background(), serverName)
			if err != nil {
				return []byte("error: " + err.Error() + "\n")
			}
			result, err := conn.CallTool(context.Background(), toolName, nil)
			if err != nil {
				return []byte("error: " + err.Error() + "\n")
			}
			fs.mu.Lock()
			fs.results[path] = result
			fs.mu.Unlock()
			return formatToolResult(result)
		}

		if fileName == ".result" {
			resultPath := strings.TrimSuffix(path, ".result") + ".call"
			fs.mu.RLock()
			result := fs.results[resultPath]
			fs.mu.RUnlock()
			if result == nil {
				return []byte("(no result yet)\n")
			}
			return formatToolResult(result)
		}
	}

	return nil
}

func (fs *CgoFS) hasScope(name string) bool {
	for serverName := range fs.cfg.Servers {
		scope, _ := config.ParseServerName(serverName)
		if scope == name {
			return true
		}
	}
	return false
}

func splitPath(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return nil
	}
	return strings.Split(path, "/")
}

func formatToolResult(result *mcp.ToolResult) []byte {
	for _, block := range result.Content {
		if block.Type == "text" {
			if result.IsError {
				return []byte("error: " + block.Text + "\n")
			}
			return []byte(block.Text + "\n")
		}
	}
	data, _ := json.MarshalIndent(result.Content, "", "  ")
	return append(data, '\n')
}
