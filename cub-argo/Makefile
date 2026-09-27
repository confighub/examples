# Makefile for cub-argo, the `cub argo` plugin. The binary is bin/cub-argo.

GO  ?= go
BIN ?= bin/cub-argo

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
	$(BIN) plan ../gitops/argo/expert-app-of-apps --stage-label rollout-phase --stages canary,secondary,primary

# Install the binary as a local cub plugin, so `cub argo` works.
install-plugin:
	mkdir -p "$${CUB_CONFIG:-$$HOME/.confighub}/plugins/argo"
	$(GO) build -o "$${CUB_CONFIG:-$$HOME/.confighub}/plugins/argo/main" .

clean:
	rm -rf bin
