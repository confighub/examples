# Makefile for cub-flux, the `cub flux` plugin. The binary is bin/cub-flux.

GO  ?= go
BIN ?= bin/cub-flux

.PHONY: all build test vet fmt golden example install-plugin clean

all: build

build:
	$(GO) build -o $(BIN) .

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

# Rewrite the golden plan after an intended change, then review the diff.
golden:
	$(GO) test ./internal/argo -run Golden -update

# Plan the expert Argo CD example in this repo. Offline; changes nothing.
example: build
	$(BIN) plan ../gitops/flux/expert-fleet

# Install the binary as a local cub plugin, so `cub flux` works.
install-plugin:
	mkdir -p "$${CUB_CONFIG:-$$HOME/.confighub}/plugins/flux"
	$(GO) build -o "$${CUB_CONFIG:-$$HOME/.confighub}/plugins/flux/main" .

clean:
	rm -rf bin
