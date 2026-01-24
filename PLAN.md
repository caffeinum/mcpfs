# mcpfs - MCP as a FUSE Filesystem

Build a FUSE filesystem in Go that exposes MCP servers as a Unix filesystem.

## Core Concept

MCP tools become files/directories. Reading files calls tools, writing files invokes mutations.

## Directory Structure
```
~/.mcp/
├── .config/
│   ├── servers.json        # MCP server definitions
│   └── auth/               # Per-server auth tokens (chmod 600)
│       └── {server}.json
├── @{scope}/
│   └── {server}/
│       ├── .status         # read: server connection status
│       ├── .schema         # read: JSON schema of all tools
│       └── {tool}/
│           ├── .schema     # read: tool's input/output schema
│           ├── .call       # write JSON here → execute tool, read result
│           └── .result     # read: last call result (cached)
```

## File Operations Mapping

| Operation | MCP Action |
|-----------|------------|
| `ls ~/.mcp/@notion/mcp/` | `listTools()` |
| `cat ~/.mcp/@notion/mcp/.schema` | Return tool schemas as JSON |
| `cat ~/.mcp/@notion/mcp/search/.schema` | Return single tool schema |
| `echo '{"query":"test"}' > ~/.mcp/@notion/mcp/search/.call` | `callTool("search", {query:"test"})` |
| `cat ~/.mcp/@notion/mcp/search/.result` | Return last cached result |
| `cat ~/.mcp/@notion/mcp/search/.call` | Execute with empty args, return result |

## Tech Stack

- **Language**: Go 1.21+
- **FUSE**: `bazil.org/fuse` (preferred) or `github.com/hanwen/go-fuse/v2`
- **MCP Protocol**: Implement JSON-RPC 2.0 over stdio (spawn process) and HTTP (remote servers)
- **Config**: JSON files in `~/.mcp/.config/`

## Implementation Requirements

1. **Lazy connection**: Only spawn/connect to MCP server when its directory is accessed
2. **Connection pooling**: Keep servers alive with configurable idle timeout (default 5min)
3. **Caching**: Cache `.schema` reads, invalidate on server reconnect
4. **Auth injection**: Load from `~/.mcp/.config/auth/{server}.json`, inject as env vars when spawning
5. **Graceful shutdown**: Clean up child processes on unmount

## Config Format (~/.mcp/.config/servers.json)
```json
{
  "@notion/mcp": {
    "transport": "stdio",
    "command": "npx",
    "args": ["-y", "@notionhq/notion-mcp-server"],
    "env": {"NOTION_TOKEN": "${auth.token}"}
  },
  "@github/mcp-server": {
    "transport": "http",
    "url": "https://mcp.github.com",
    "headers": {"Authorization": "Bearer ${auth.token}"}
  }
}
```

## MCP JSON-RPC Implementation

Implement minimal MCP client:
- `initialize` handshake
- `tools/list` → returns available tools
- `tools/call` → executes a tool with arguments

For stdio transport:
```go
cmd := exec.Command(config.Command, config.Args...)
cmd.Stdin = stdinPipe   // write JSON-RPC requests
cmd.Stdout = stdoutPipe // read JSON-RPC responses
```

For HTTP transport:
- POST to server URL with JSON-RPC body
- Handle SSE for streaming (stretch goal)

## CLI Commands
```bash
mcpfs mount <mountpoint>           # Mount filesystem (daemonizes)
mcpfs mount <mountpoint> -f        # Foreground mode (for debugging)
mcpfs umount <mountpoint>          # Unmount and cleanup
mcpfs add <name> -- <command>      # Add stdio server to config
mcpfs add <name> --url <url>       # Add HTTP server to config
mcpfs auth <server> <token>        # Store auth token securely
mcpfs status                       # List servers and connection state
mcpfs list                         # List configured servers
```

## Error Handling

Map errors to appropriate FUSE/errno:
- Tool/server not found → `ENOENT`
- Auth failure → `EACCES`
- Server timeout → `EIO`
- Invalid JSON input → `EINVAL`
- Server not connected → `ENOTCONN`

## Project Structure
```
mcpfs/
├── cmd/
│   └── mcpfs/
│       └── main.go           # CLI entry point
├── internal/
│   ├── fs/
│   │   ├── fs.go             # FUSE filesystem implementation
│   │   ├── server_dir.go     # @scope/server directory handling
│   │   └── tool_dir.go       # tool directory + .call/.schema files
│   ├── mcp/
│   │   ├── client.go         # MCP JSON-RPC client interface
│   │   ├── stdio.go          # stdio transport implementation
│   │   └── http.go           # HTTP transport implementation
│   ├── config/
│   │   ├── config.go         # Config loading/parsing
│   │   └── auth.go           # Auth token management
│   └── pool/
│       └── pool.go           # Server connection pool with idle timeout
├── go.mod
├── go.sum
└── README.md
```

## Testing

1. Unit tests for MCP client with mock server
2. Integration tests mounting real filesystem
3. Test cases:
   - `ls` returns tools correctly
   - `cat .schema` returns valid JSON
   - Write to `.call` executes tool and returns result
   - Concurrent access from multiple processes
   - Server crash recovery
   - Idle timeout disconnects server
   - Unmount cleans up all child processes

## Build & Install
```bash
go build -o mcpfs ./cmd/mcpfs
# or
go install github.com/yourname/mcpfs/cmd/mcpfs@latest
```

## Stretch Goals

- Streaming: `tail -f ~/.mcp/@slack/mcp/messages/.stream` for real-time data
- Resources: expose MCP resources as readable files
- Prompts: expose MCP prompts as template files
- File watching: `inotify` on `.result` to watch for updates
- macOS Keychain integration for auth storage
