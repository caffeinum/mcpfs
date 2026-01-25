```
                        ___     
  _ __ ___   ___ _ __  / __\___
 | '_ ` _ \ / __| '_ \/ _\/ __|
 | | | | | | (__| |_) / / \__ \
 |_| |_| |_|\___| .__/\/  |___/
                |_|            
```

mcp servers as a filesystem. because config files are where tools go to die.

## why

i got tired of this loop:

1. add mcp to claude config
2. restart claude
3. mcp fails (token expired, package updated, who knows)
4. debug for 20 minutes
5. give up, remove mcp
6. repeat next week

also: some mcps return 50k tokens. good luck fitting that in context.

mcpfs fixes this:

- **no restarts** - add servers while claude is running
- **lazy loading** - servers only spawn when you actually use them  
- **unix pipes** - `cat .result | jq '.data[0]'` - filter before it hits your context
- **separate process** - mcp crashes don't take down your session

## install

**macos (homebrew)**:

```bash
brew install --cask macos-fuse-t/cask/fuse-t
brew install caffeinum/tap/mcpfs
```

**macos (manual)**:

```bash
brew install --cask macos-fuse-t/cask/fuse-t
curl -L https://github.com/caffeinum/mcpfs/releases/latest/download/mcpfs_darwin_arm64.tar.gz | tar xz
mv mcpfs ~/.local/bin/
```

**linux**:

```bash
sudo apt install libfuse-dev  # or: dnf install fuse-devel
curl -L https://github.com/caffeinum/mcpfs/releases/latest/download/mcpfs_linux_amd64.tar.gz | tar xz
mv mcpfs ~/.local/bin/
```

**build from source**:

```bash
git clone https://github.com/caffeinum/mcpfs
cd mcpfs
make install
```

## quick start

```bash
# mount
mkdir -p ~/mcp
mcpfs mount ~/mcp

# add a server (in another terminal)
mcpfs add @github/mcp -- npx -y @modelcontextprotocol/server-github
mcpfs auth @github/mcp "ghp_yourtoken"

# use it
ls ~/mcp/@github/mcp/
cat ~/mcp/@github/mcp/search_repositories/.schema
echo '{"query":"mcpfs language:go"}' > ~/mcp/@github/mcp/search_repositories/.call
cat ~/mcp/@github/mcp/search_repositories/.result | jq '.items[0].html_url'
```

## how it works

**server lifecycle**: servers spawn on first access, stay alive for reuse, auto-close after 5 min idle. not per-request - way faster for repeated calls.

**caching**: `.result` is cached in memory until the next `.call` write. read it multiple times, pipe it, grep it - no re-execution. `.schema` fetches fresh each time (tools might change).

**separate process**: mcpfs runs independently. add/remove servers without restarting your claude session. if an mcp crashes, just access it again - it respawns.

## for claude code

tell claude:

```
mcp servers are mounted at ~/mcp
to call a tool: echo '{"arg":"value"}' > ~/mcp/@server/mcp/tool/.call
to read result: cat ~/mcp/@server/mcp/tool/.result
to add new mcp: mcpfs add @name/mcp -- npx -y @package/name
```

claude can now add and use mcps on the fly. no config editing. no restarts.

## filesystem layout

```
~/mcp/
├── .config/servers.json     # server definitions (auto-managed)
├── @github/mcp/
│   ├── .schema              # all tools (fetched on read)
│   ├── .status              # connection state
│   └── search_repositories/
│       ├── .schema          # input schema for this tool
│       ├── .call            # write json here to execute
│       └── .result          # cached result from last call
```

## commands

```bash
mcpfs mount <path>          # mount filesystem
mcpfs add @name -- <cmd>    # add stdio server
mcpfs add @name --url <u>   # add http server  
mcpfs auth @name <token>    # save auth token
mcpfs list                  # show servers
```

## license

mit
