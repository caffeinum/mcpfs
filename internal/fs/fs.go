package fs

import (
	"context"
	"encoding/json"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	"github.com/caffeinum/mcpfs/internal/config"
	"github.com/caffeinum/mcpfs/internal/mcp"
	"github.com/caffeinum/mcpfs/internal/pool"
)

type MCPFS struct {
	cfg  *config.Config
	pool *pool.Pool
}

func New(cfg *config.Config, p *pool.Pool) *MCPFS {
	return &MCPFS{
		cfg:  cfg,
		pool: p,
	}
}

type RootNode struct {
	fs.Inode
	mcpfs *MCPFS
}

var _ fs.InodeEmbedder = (*RootNode)(nil)
var _ fs.NodeReaddirer = (*RootNode)(nil)
var _ fs.NodeLookuper = (*RootNode)(nil)

func (n *RootNode) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	var entries []fuse.DirEntry

	entries = append(entries, fuse.DirEntry{
		Name: ".config",
		Mode: fuse.S_IFDIR,
	})

	scopes := make(map[string]bool)
	for name := range n.mcpfs.cfg.Servers {
		scope, _ := config.ParseServerName(name)
		if scope != "" {
			scopes[scope] = true
		}
	}

	for scope := range scopes {
		entries = append(entries, fuse.DirEntry{
			Name: scope,
			Mode: fuse.S_IFDIR,
		})
	}

	return fs.NewListDirStream(entries), 0
}

func (n *RootNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	if name == ".config" {
		child := &ConfigNode{mcpfs: n.mcpfs}
		return n.NewInode(ctx, child, fs.StableAttr{Mode: fuse.S_IFDIR}), 0
	}

	if len(name) > 0 && name[0] == '@' {
		for serverName := range n.mcpfs.cfg.Servers {
			scope, _ := config.ParseServerName(serverName)
			if scope == name {
				child := &ScopeNode{mcpfs: n.mcpfs, scope: name}
				return n.NewInode(ctx, child, fs.StableAttr{Mode: fuse.S_IFDIR}), 0
			}
		}
	}

	return nil, syscall.ENOENT
}

type ConfigNode struct {
	fs.Inode
	mcpfs *MCPFS
}

var _ fs.InodeEmbedder = (*ConfigNode)(nil)
var _ fs.NodeReaddirer = (*ConfigNode)(nil)
var _ fs.NodeLookuper = (*ConfigNode)(nil)

func (n *ConfigNode) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	return fs.NewListDirStream([]fuse.DirEntry{
		{Name: "servers.json", Mode: fuse.S_IFREG},
	}), 0
}

func (n *ConfigNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	if name == "servers.json" {
		child := &ServersFile{mcpfs: n.mcpfs}
		return n.NewInode(ctx, child, fs.StableAttr{Mode: fuse.S_IFREG}), 0
	}
	return nil, syscall.ENOENT
}

type ServersFile struct {
	fs.Inode
	mcpfs *MCPFS
}

var _ fs.InodeEmbedder = (*ServersFile)(nil)
var _ fs.NodeOpener = (*ServersFile)(nil)
var _ fs.NodeGetattrer = (*ServersFile)(nil)

func (f *ServersFile) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	data, _ := config.MarshalServers(f.mcpfs.cfg.Servers)
	out.Size = uint64(len(data))
	out.Mode = 0644
	return 0
}

func (f *ServersFile) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	data, _ := config.MarshalServers(f.mcpfs.cfg.Servers)
	return &bytesFileHandle{data: data}, fuse.FOPEN_DIRECT_IO, 0
}

type bytesFileHandle struct {
	data []byte
}

var _ fs.FileReader = (*bytesFileHandle)(nil)

func (fh *bytesFileHandle) Read(ctx context.Context, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	end := int(off) + len(dest)
	if end > len(fh.data) {
		end = len(fh.data)
	}
	if int(off) >= len(fh.data) {
		return fuse.ReadResultData(nil), 0
	}
	return fuse.ReadResultData(fh.data[off:end]), 0
}

type ScopeNode struct {
	fs.Inode
	mcpfs *MCPFS
	scope string
}

var _ fs.InodeEmbedder = (*ScopeNode)(nil)
var _ fs.NodeReaddirer = (*ScopeNode)(nil)
var _ fs.NodeLookuper = (*ScopeNode)(nil)

func (n *ScopeNode) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	var entries []fuse.DirEntry
	for name := range n.mcpfs.cfg.Servers {
		scope, server := config.ParseServerName(name)
		if scope == n.scope {
			entries = append(entries, fuse.DirEntry{
				Name: server,
				Mode: fuse.S_IFDIR,
			})
		}
	}
	return fs.NewListDirStream(entries), 0
}

func (n *ScopeNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	fullName := n.scope + "/" + name
	if _, ok := n.mcpfs.cfg.Servers[fullName]; ok {
		child := &ServerNode{mcpfs: n.mcpfs, name: fullName}
		return n.NewInode(ctx, child, fs.StableAttr{Mode: fuse.S_IFDIR}), 0
	}
	return nil, syscall.ENOENT
}

type ServerNode struct {
	fs.Inode
	mcpfs *MCPFS
	name  string
}

var _ fs.InodeEmbedder = (*ServerNode)(nil)
var _ fs.NodeReaddirer = (*ServerNode)(nil)
var _ fs.NodeLookuper = (*ServerNode)(nil)

func (n *ServerNode) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	entries := []fuse.DirEntry{
		{Name: ".status", Mode: fuse.S_IFREG},
		{Name: ".schema", Mode: fuse.S_IFREG},
	}

	conn, err := n.mcpfs.pool.GetConnection(ctx, n.name)
	if err != nil {
		return fs.NewListDirStream(entries), 0
	}

	for _, tool := range conn.GetTools() {
		entries = append(entries, fuse.DirEntry{
			Name: tool.Name,
			Mode: fuse.S_IFDIR,
		})
	}

	return fs.NewListDirStream(entries), 0
}

func (n *ServerNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	switch name {
	case ".status":
		child := &StatusNode{mcpfs: n.mcpfs, server: n.name}
		return n.NewInode(ctx, child, fs.StableAttr{Mode: fuse.S_IFREG}), 0
	case ".schema":
		child := &SchemaNode{mcpfs: n.mcpfs, server: n.name}
		return n.NewInode(ctx, child, fs.StableAttr{Mode: fuse.S_IFREG}), 0
	}

	conn, err := n.mcpfs.pool.GetConnection(ctx, n.name)
	if err != nil {
		return nil, syscall.EIO
	}

	for _, tool := range conn.GetTools() {
		if tool.Name == name {
			child := &ToolNode{mcpfs: n.mcpfs, server: n.name, tool: tool}
			return n.NewInode(ctx, child, fs.StableAttr{Mode: fuse.S_IFDIR}), 0
		}
	}

	return nil, syscall.ENOENT
}

type StatusNode struct {
	fs.Inode
	mcpfs  *MCPFS
	server string
}

var _ fs.InodeEmbedder = (*StatusNode)(nil)
var _ fs.NodeOpener = (*StatusNode)(nil)
var _ fs.NodeGetattrer = (*StatusNode)(nil)

func (n *StatusNode) content() []byte {
	status := n.mcpfs.pool.GetStatus()
	info, ok := status[n.server]
	if !ok {
		return []byte("disconnected\n")
	}

	result := "status: " + info.Status + "\n"
	if info.ToolCount > 0 {
		result += "tools: " + string(rune(info.ToolCount+'0')) + "\n"
	}
	if info.Error != "" {
		result += "error: " + info.Error + "\n"
	}
	return []byte(result)
}

func (n *StatusNode) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Size = uint64(len(n.content()))
	out.Mode = 0444
	return 0
}

func (n *StatusNode) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	return &bytesFileHandle{data: n.content()}, fuse.FOPEN_DIRECT_IO, 0
}

type SchemaNode struct {
	fs.Inode
	mcpfs  *MCPFS
	server string
}

var _ fs.InodeEmbedder = (*SchemaNode)(nil)
var _ fs.NodeOpener = (*SchemaNode)(nil)
var _ fs.NodeGetattrer = (*SchemaNode)(nil)

func (n *SchemaNode) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = 0444
	return 0
}

func (n *SchemaNode) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	conn, err := n.mcpfs.pool.GetConnection(ctx, n.server)
	if err != nil {
		return nil, 0, syscall.EIO
	}

	tools := conn.GetTools()
	schema := make([]map[string]any, 0, len(tools))

	for _, tool := range tools {
		item := map[string]any{
			"name":        tool.Name,
			"description": tool.Description,
		}
		if tool.InputSchema != nil {
			var input any
			json.Unmarshal(tool.InputSchema, &input)
			item["inputSchema"] = input
		}
		schema = append(schema, item)
	}

	data, _ := json.MarshalIndent(schema, "", "  ")
	return &bytesFileHandle{data: append(data, '\n')}, fuse.FOPEN_DIRECT_IO, 0
}

type ToolNode struct {
	fs.Inode
	mcpfs  *MCPFS
	server string
	tool   mcp.Tool
	result *mcp.ToolResult
}

var _ fs.InodeEmbedder = (*ToolNode)(nil)
var _ fs.NodeReaddirer = (*ToolNode)(nil)
var _ fs.NodeLookuper = (*ToolNode)(nil)

func (n *ToolNode) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	return fs.NewListDirStream([]fuse.DirEntry{
		{Name: ".schema", Mode: fuse.S_IFREG},
		{Name: ".call", Mode: fuse.S_IFREG},
		{Name: ".result", Mode: fuse.S_IFREG},
	}), 0
}

func (n *ToolNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	switch name {
	case ".schema":
		child := &ToolSchemaNode{tool: n.tool}
		return n.NewInode(ctx, child, fs.StableAttr{Mode: fuse.S_IFREG}), 0
	case ".call":
		child := &CallNode{mcpfs: n.mcpfs, server: n.server, toolNode: n}
		return n.NewInode(ctx, child, fs.StableAttr{Mode: fuse.S_IFREG}), 0
	case ".result":
		child := &ResultNode{toolNode: n}
		return n.NewInode(ctx, child, fs.StableAttr{Mode: fuse.S_IFREG}), 0
	}
	return nil, syscall.ENOENT
}

type ToolSchemaNode struct {
	fs.Inode
	tool mcp.Tool
}

var _ fs.InodeEmbedder = (*ToolSchemaNode)(nil)
var _ fs.NodeOpener = (*ToolSchemaNode)(nil)
var _ fs.NodeGetattrer = (*ToolSchemaNode)(nil)

func (n *ToolSchemaNode) content() []byte {
	schema := map[string]any{
		"name":        n.tool.Name,
		"description": n.tool.Description,
	}
	if n.tool.InputSchema != nil {
		var input any
		json.Unmarshal(n.tool.InputSchema, &input)
		schema["inputSchema"] = input
	}
	data, _ := json.MarshalIndent(schema, "", "  ")
	return append(data, '\n')
}

func (n *ToolSchemaNode) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Size = uint64(len(n.content()))
	out.Mode = 0444
	return 0
}

func (n *ToolSchemaNode) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	return &bytesFileHandle{data: n.content()}, fuse.FOPEN_DIRECT_IO, 0
}

type CallNode struct {
	fs.Inode
	mcpfs    *MCPFS
	server   string
	toolNode *ToolNode
}

var _ fs.InodeEmbedder = (*CallNode)(nil)
var _ fs.NodeOpener = (*CallNode)(nil)
var _ fs.NodeGetattrer = (*CallNode)(nil)

func (n *CallNode) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = 0666
	return 0
}

func (n *CallNode) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	return &callFileHandle{node: n}, fuse.FOPEN_DIRECT_IO, 0
}

type callFileHandle struct {
	node *CallNode
}

var _ fs.FileReader = (*callFileHandle)(nil)
var _ fs.FileWriter = (*callFileHandle)(nil)

func (fh *callFileHandle) Read(ctx context.Context, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	conn, err := fh.node.mcpfs.pool.GetConnection(ctx, fh.node.server)
	if err != nil {
		return fuse.ReadResultData([]byte("error: " + err.Error() + "\n")), 0
	}

	result, err := conn.CallTool(ctx, fh.node.toolNode.tool.Name, nil)
	if err != nil {
		return fuse.ReadResultData([]byte("error: " + err.Error() + "\n")), 0
	}

	fh.node.toolNode.result = result
	data := formatResult(result)

	if int(off) >= len(data) {
		return fuse.ReadResultData(nil), 0
	}
	return fuse.ReadResultData(data[off:]), 0
}

func (fh *callFileHandle) Write(ctx context.Context, data []byte, off int64) (uint32, syscall.Errno) {
	var args map[string]any
	if err := json.Unmarshal(data, &args); err != nil {
		return 0, syscall.EINVAL
	}

	conn, err := fh.node.mcpfs.pool.GetConnection(ctx, fh.node.server)
	if err != nil {
		return 0, syscall.EIO
	}

	result, err := conn.CallTool(ctx, fh.node.toolNode.tool.Name, args)
	if err != nil {
		return 0, syscall.EIO
	}

	fh.node.toolNode.result = result
	return uint32(len(data)), 0
}

type ResultNode struct {
	fs.Inode
	toolNode *ToolNode
}

var _ fs.InodeEmbedder = (*ResultNode)(nil)
var _ fs.NodeOpener = (*ResultNode)(nil)
var _ fs.NodeGetattrer = (*ResultNode)(nil)

func (n *ResultNode) content() []byte {
	if n.toolNode.result == nil {
		return []byte("(no result yet)\n")
	}
	return formatResult(n.toolNode.result)
}

func (n *ResultNode) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Size = uint64(len(n.content()))
	out.Mode = 0444
	return 0
}

func (n *ResultNode) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	return &bytesFileHandle{data: n.content()}, fuse.FOPEN_DIRECT_IO, 0
}

func formatResult(result *mcp.ToolResult) []byte {
	if result.IsError {
		return []byte("error: " + extractText(result) + "\n")
	}
	return []byte(extractText(result) + "\n")
}

func extractText(result *mcp.ToolResult) string {
	for _, block := range result.Content {
		if block.Type == "text" {
			return block.Text
		}
	}
	data, _ := json.MarshalIndent(result.Content, "", "  ")
	return string(data)
}
