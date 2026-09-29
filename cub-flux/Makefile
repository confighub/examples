# Makefile for cub-flux, the `cub flux` plugin. The binary is bin/cub-flux.

GO  ?= go
BIN ?= bin/cub-flux

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
	$(GO) test ./internal/flux -run Golden -update

# Plan the expert Argo CD example in this repo. Offline; changes nothing.
example: build
	$(BIN) plan ../gitops/flux/expert-fleet

# Install as a local cub plugin, so `cub flux` works. A plugin is a directory
# named for the repository, holding the binary and the manifest that names it.
PLUGIN_DIR ?= $(or $(CUB_CONFIG),$(HOME)/.confighub)/plugins/cub-flux

install-plugin:
	mkdir -p "$(PLUGIN_DIR)"
	$(GO) build -o "$(PLUGIN_DIR)/cub-flux" .
	printf 'name: flux\nversion: 0.1.0\ncommands:\n    - name: flux\n      summary: Read a Flux fleet repository and plan it into ConfigHub, one variant per cluster\n      entrypoint: cub-flux\n' > "$(PLUGIN_DIR)/cub-plugin.yaml"
	@echo "Installed. Check it with: cub plugin list"

uninstall-plugin:
	rm -rf "$(PLUGIN_DIR)"

clean:
	rm -rf bin
