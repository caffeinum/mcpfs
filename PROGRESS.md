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

- [ ] **stage 3: mcp client**
  - [ ] json-rpc 2.0 base
  - [ ] stdio transport
  - [ ] http transport
  - [ ] test with mock server
  - [ ] commit

- [ ] **stage 4: connection pool**
  - [ ] lazy connection
  - [ ] idle timeout
  - [ ] graceful shutdown
  - [ ] commit

- [ ] **stage 5: fuse filesystem**
  - [ ] basic mount/unmount
  - [ ] root directory listing
  - [ ] server directories
  - [ ] tool directories
  - [ ] .schema files
  - [ ] .call files (read/write)
  - [ ] .result files
  - [ ] commit

- [ ] **stage 6: cli commands**
  - [ ] mount/umount
  - [ ] add server
  - [ ] auth management
  - [ ] status/list
  - [ ] commit

- [ ] **stage 7: error handling & polish**
  - [ ] proper errno mapping
  - [ ] integration tests
  - [ ] readme
  - [ ] final commit

## notes

- using bazil.org/fuse for fuse bindings
- go 1.25.5 confirmed
