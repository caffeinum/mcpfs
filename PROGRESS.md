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

- [ ] **stage 7: error handling & polish**
  - [ ] readme
  - [ ] final commit

## notes

- switched from bazil.org/fuse to go-fuse/v2 (macos compat)
- go 1.25.5 confirmed
- 14 tests passing across all packages
