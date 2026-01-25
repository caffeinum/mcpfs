.PHONY: build build-linux build-macos build-windows clean install

BINARY := mcpfs
BUILD_DIR := ./cmd/mcpfs
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

# detect OS
UNAME := $(shell uname -s)

# macos fuse-t flags
MACOS_CGO_CFLAGS := -I/Library/Frameworks/fuse_t.framework/Versions/A/Headers
MACOS_CGO_LDFLAGS := -L/usr/local/lib -lfuse-t -Wl,-rpath,/usr/local/lib

build:
ifeq ($(UNAME),Darwin)
	@$(MAKE) build-macos
else ifeq ($(UNAME),Linux)
	@$(MAKE) build-linux
else
	@echo "unsupported platform: $(UNAME)"
	@exit 1
endif

build-macos:
	CGO_ENABLED=1 \
	CGO_CFLAGS="$(MACOS_CGO_CFLAGS)" \
	CGO_LDFLAGS="$(MACOS_CGO_LDFLAGS)" \
	go build $(LDFLAGS) -o $(BINARY) $(BUILD_DIR)

build-linux:
	CGO_ENABLED=1 go build $(LDFLAGS) -o $(BINARY) $(BUILD_DIR)

build-windows:
	CGO_ENABLED=1 GOOS=windows go build $(LDFLAGS) -o $(BINARY).exe $(BUILD_DIR)

clean:
	rm -f $(BINARY) $(BINARY).exe

PREFIX := $(HOME)/.local

install: build
	mkdir -p $(PREFIX)/bin
	install -m 755 $(BINARY) $(PREFIX)/bin/

test:
	go test ./...
