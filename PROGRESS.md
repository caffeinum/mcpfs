# mcpfs implementation progress

## stages

- [x] **stage 1: project setup** ✓
  - [x] check go version (1.25.5 - good)
  - [x] git init
  - [x] go mod init
  - [x] basic cli skeleton (cobra)
  - [x] commit c2049bf

- [x] **stage 2: config system** ✓
  - [x] config loading/parsing
  - [x] auth token management with ${auth.var} resolution
  - [x] test config loading (6 tests passing)
  - [x] commit

- [x] **stage 3: mcp client** ✓
  - [x] json-rpc 2.0 base with atomic request ids
  - [x] stdio transport (spawn process, newline-delimited json)
  - [x] http transport with configurable timeout
  - [x] test with mock server (3 tests passing)
  - [x] commit

- [x] **stage 4: connection pool** ✓
  - [x] lazy connection (only connect on first access)
  - [x] idle timeout with background reaper
  - [x] graceful shutdown (close all connections)
  - [x] 5 tests passing
  - [x] commit

- [x] **stage 5: fuse filesystem** ✓
  - [x] basic mount/unmount with go-fuse/v2
  - [x] root directory listing (scopes + .config)
  - [x] server directories with lazy connect
  - [x] tool directories
  - [x] .schema files (server + tool level)
  - [x] .call files (read executes, write with json args)
  - [x] .result files (cached last result)
  - [x] .status files per server
  - [x] commit

- [x] **stage 6: cli commands** ✓
  - [x] mount/umount
  - [x] add server (stdio + http)
  - [x] auth management (save tokens)
  - [x] status/list
  - [x] commit

- [x] **stage 7: error handling & polish** ✓
  - [x] readme with install instructions
  - [x] final commit

- [ ] **stage 8: fuse-t migration** (in progress)
  - [x] switched from go-fuse/v2 to cgofuse (libfuse wrapper)
  - [x] configured build to link against fuse-t
  - [ ] resolve macfuse conflict (macfuse installed alongside fuse-t causes dialog)
  - [ ] test full filesystem operations

## notes

- switched from bazil.org/fuse to go-fuse/v2 (macos compat)
- switched from go-fuse/v2 to cgofuse (supports fuse-t via libfuse API)
- go 1.25.5 confirmed
- 14 tests passing across all packages

## fuse backend history

1. bazil.org/fuse - doesn't compile on macos
2. go-fuse/v2 - talks directly to kernel, doesn't support fskit
3. cgofuse + macfuse fskit - fskit extension fails to register
4. cgofuse + fuse-t - current attempt, works but macfuse conflicts

## build instructions (fuse-t)

```bash
# install fuse-t (no kernel extension needed)
# download from https://github.com/macos-fuse-t/fuse-t/releases

# build with fuse-t
PKG_CONFIG_PATH=/usr/local/lib/pkgconfig \
CGO_CFLAGS="-I/usr/local/include/fuse" \
CGO_LDFLAGS="-L/usr/local/lib -lfuse-t" \
go build ./cmd/mcpfs

# if macfuse is installed, uninstall it first to avoid conflicts
# run: /Library/Filesystems/macfuse.fs/Contents/Resources/uninstall_macfuse.app
```
