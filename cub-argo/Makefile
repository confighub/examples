# Makefile for cub-argo, the `cub argo` plugin. The binary is bin/cub-argo.

GO  ?= go
BIN ?= bin/cub-argo

.PHONY: all build test vet fmt golden example install-plugin uninstall-plugin clean

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

# Install as a local cub plugin, so `cub argo` works. A plugin is a directory
# named for the repository, holding the binary and the manifest that names it.
# Install as a cub plugin through cub's own route: the binary's install hook
# writes cub-plugin.yaml, so the manifest cannot drift from the commands it
# implements, and a rebuild is picked up with `cub plugin upgrade argo`.
install-plugin: build
	cub plugin upgrade argo >/dev/null 2>&1 || cub plugin install ./$(BIN)
	@echo "Installed. Check it with: cub plugin list"

uninstall-plugin:
	cub plugin uninstall argo

clean:
	rm -rf bin
