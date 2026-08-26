# Makefile for the cub-observability CLI.
#
# The output binary is `bin/cub-observability`. Installed as a cub plugin it is
# invoked as `cub observability ...`, and standalone as `cub-observability ...`.

GO     ?= go
BIN    ?= bin/cub-observability
PKG    ?= ./cmd/cub-observability

.PHONY: all build release-build test vet fmt clean

all: build

build: $(BIN)

# managerkit is a local module wired in with a replace directive, so its
# sources are prerequisites too: without them make serves a stale binary
# after a library change, and a stale binary lies.
$(BIN): $(shell find . ../managerkit -name '*.go' -not -path './bin/*')
	$(GO) build -o $(BIN) $(PKG)

release-build:
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) \
	  $(GO) build -trimpath \
	    -ldflags "-s -w -X github.com/confighub/examples/observability-manager/internal/version.Version=$(VERSION)" \
	    -o $(or $(OUT),cub-observability-$(GOOS)-$(GOARCH)) $(PKG)

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

clean:
	rm -f bin/cub-observability
