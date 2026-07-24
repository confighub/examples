# Makefile for the k8s-to-score CLI.
#
# The output binary is `bin/k8s-to-score`.

GO     ?= go
BIN    ?= bin/k8s-to-score
PKG    ?= ./cmd/k8s-to-score

.PHONY: all build release-build test vet fmt sample clean

all: build

build: $(BIN)

$(BIN): $(shell find . -name '*.go' -not -path './bin/*')
	$(GO) build -o $(BIN) $(PKG)

# Release-style build: cross-compile with version injection. Override GOOS,
# GOARCH, VERSION; OUT defaults to a per-arch binary name.
#
# Example: make release-build VERSION=0.1.0 GOOS=darwin GOARCH=arm64
release-build:
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) \
	  $(GO) build -trimpath \
	    -ldflags "-s -w -X github.com/confighub/examples/k8s-to-score/internal/version.Version=$(VERSION)" \
	    -o $(or $(OUT),k8s-to-score-$(GOOS)-$(GOARCH)) $(PKG)

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

# Convert the bundled fixtures — no ConfigHub session needed. Useful for seeing
# the output shape before pointing the tool at a real Space.
sample: build
	$(BIN) --from-dir testdata/sample --out-dir /tmp/k8s-to-score-sample

clean:
	rm -f $(BIN)
