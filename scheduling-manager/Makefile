# Makefile for the cub-scheduling CLI.
#
# The output binary is `bin/cub-scheduling`. Installed as a cub plugin it is
# invoked as `cub scheduling ...`, and standalone as `cub-scheduling ...`.

GO     ?= go
BIN    ?= bin/cub-scheduling
PKG    ?= ./cmd/cub-scheduling

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
	    -ldflags "-s -w -X github.com/confighub/examples/scheduling-manager/internal/version.Version=$(VERSION)" \
	    -o $(or $(OUT),cub-scheduling-$(GOOS)-$(GOARCH)) $(PKG)

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

clean:
	rm -f bin/cub-scheduling
