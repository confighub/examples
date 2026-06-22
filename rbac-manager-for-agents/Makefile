# Makefile for the cub-rbac CLI.
#
# The output binary is `bin/cub-rbac`. The cub plugin protocol routes the
# command token declared in the manifest ("rbac"), so installed as a plugin it
# is invoked as `cub rbac ...`, and standalone as `cub-rbac ...`.

GO     ?= go
BIN    ?= bin/cub-rbac
PKG    ?= ./cmd/cub-rbac

.PHONY: all build release-build test vet fmt clean

all: build

build: $(BIN)

$(BIN): $(shell find . -name '*.go' -not -path './bin/*')
	$(GO) build -o $(BIN) $(PKG)

# Release-style build: cross-compile with version injection, used by the release
# workflow. Override GOOS, GOARCH, VERSION; OUT defaults to a per-arch binary
# name (cub-rbac-<goos>-<goarch>).
#
# Example: make release-build VERSION=0.1.0 GOOS=darwin GOARCH=arm64
release-build:
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) \
	  $(GO) build -trimpath \
	    -ldflags "-s -w -X github.com/confighub/examples/rbac-manager-for-agents/internal/version.Version=$(VERSION)" \
	    -o $(or $(OUT),cub-rbac-$(GOOS)-$(GOARCH)) $(PKG)

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

clean:
	rm -f bin/cub-rbac
