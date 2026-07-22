# Makefile for the cub-eks CLI.
#
# The output binary is `bin/cub-eks`. The cub plugin protocol routes the command
# token declared in the manifest ("eks"), so installed as a plugin it is invoked
# as `cub eks ...`, and standalone as `cub-eks ...`.

GO     ?= go
BIN    ?= bin/cub-eks
PKG    ?= ./cmd/cub-eks

.PHONY: all build release-build test vet fmt clean

all: build

build: $(BIN)

$(BIN): $(shell find . -name '*.go' -not -path './bin/*')
	$(GO) build -o $(BIN) $(PKG)

# Release-style build: cross-compile with version injection, used by the release
# workflow. Override GOOS, GOARCH, VERSION; OUT defaults to a per-arch binary
# name (cub-eks-<goos>-<goarch>).
#
# Example: make release-build VERSION=0.1.0 GOOS=darwin GOARCH=arm64
release-build:
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) \
	  $(GO) build -trimpath \
	    -ldflags "-s -w -X github.com/confighub/examples/eks-manager/internal/version.Version=$(VERSION)" \
	    -o $(or $(OUT),cub-eks-$(GOOS)-$(GOARCH)) $(PKG)

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

clean:
	rm -f bin/cub-eks
