# mcpfs

mount mcp servers as a fuse filesystem. mcp tools become files and directories.

## install

```bash
go install github.com/caffeinum/mcpfs/cmd/mcpfs@latest
```

requires macos with macfuse or linux with fuse.

## usage

### add an mcp server

```bash
# stdio server (spawns a process)
mcpfs add @notion/mcp -- npx -y @notionhq/notion-mcp-server

# http server
mcpfs add @github/mcp --url https://mcp.github.com
```

### set auth token

```bash
mcpfs auth @notion/mcp "secret-token-here"
```

tokens are stored in `~/.mcp/.config/auth/` with 0600 permissions.

### mount the filesystem

```bash
mkdir -p ~/mcp
mcpfs mount ~/mcp
```

press ctrl+c to unmount.

### browse and use tools

```bash
# list servers
ls ~/mcp/
# @notion  @github  .config

# list tools for a server
ls ~/mcp/@notion/mcp/
# search  get-page  create-page  .status  .schema

# view tool schema
cat ~/mcp/@notion/mcp/search/.schema

# call a tool (read executes with no args)
cat ~/mcp/@notion/mcp/search/.call

# call a tool with args
echo '{"query": "meeting notes"}' > ~/mcp/@notion/mcp/search/.call

# read cached result
cat ~/mcp/@notion/mcp/search/.result
```

## filesystem structure

```
~/mcp/
├── .config/
│   └── servers.json        # mcp server definitions
├── @{scope}/
│   └── {server}/
│       ├── .status         # connection status
│       ├── .schema         # all tool schemas
│       └── {tool}/
│           ├── .schema     # tool input/output schema
│           ├── .call       # write json → execute tool
│           └── .result     # last call result
```

## config format

`~/.mcp/.config/servers.json`:

```json
{
  "@notion/mcp": {
    "transport": "stdio",
    "command": "npx",
    "args": ["-y", "@notionhq/notion-mcp-server"],
    "env": {"NOTION_TOKEN": "${auth.token}"}
  },
  "@github/mcp": {
    "transport": "http",
    "url": "https://mcp.github.com",
    "headers": {"Authorization": "Bearer ${auth.token}"}
  }
}
```

`${auth.token}` is replaced with the value from the auth file.

## commands

```bash
mcpfs mount <mountpoint>    # mount the filesystem
mcpfs umount <mountpoint>   # unmount
mcpfs add <name> -- <cmd>   # add stdio server
mcpfs add <name> --url <u>  # add http server
mcpfs auth <server> <token> # store auth token
mcpfs list                  # list configured servers
mcpfs status                # show connection status
```

## features

- lazy connection: servers only connect when their directory is accessed
- idle timeout: connections close after 5 minutes of inactivity
- graceful shutdown: unmount cleans up all child processes

## license

mit
