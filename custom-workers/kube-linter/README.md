# kube-linter Validation Example

This example demonstrates a custom ConfigHub function that validates Kubernetes resources using [kube-linter](https://github.com/stackrox/kube-linter) best-practice checks. It uses the `kube-linter` CLI for linting, avoiding heavy build dependencies.

## Prerequisites

- The `kube-linter` CLI must be installed and available in `PATH`. See [kube-linter installation](https://github.com/stackrox/kube-linter#installing-kubelinter).
- A running ConfigHub server (for the worker mode).

## Quick Start

Build the kube-linter CLI (if not already installed):

    # From a kube-linter source checkout:
    go build -o /usr/local/bin/kube-linter ./cmd/kube-linter/

    # Or install from release:
    # See https://github.com/stackrox/kube-linter#installing-kubelinter

Build the example function executor:

    go build

### Running locally

Create a Worker, put its credentials in your shell, name the server, and start the function executor:

    cub worker create --space $SPACE my-kube-linter-worker
    eval "$(cub worker get-envs --space $SPACE my-kube-linter-worker)"
    export CONFIGHUB_URL=https://hub.confighub.com
    ./kube-linter

The function executor reads `CONFIGHUB_WORKER_ID`, `CONFIGHUB_WORKER_SECRET` and `CONFIGHUB_URL` from its environment and connects to ConfigHub as that Worker. The `kube-linter` CLI must be in PATH.

### Running in a Kubernetes cluster

Build and push a container image:

    docker build -f Dockerfile -t my-registry/kube-linter-worker:latest .
    docker push my-registry/kube-linter-worker:latest

Then give the cluster the Worker's credentials as a Secret and run the image with a Deployment that reads it. [`deploy-worker.sh`](../deploy-worker.sh) does both, and waits for the rollout:

    IMAGE_PULL_POLICY=IfNotPresent ../deploy-worker.sh $SPACE my-kube-linter-worker confighub my-registry/kube-linter-worker:latest

[External Functions](https://docs.confighub.com/guide/external-functions/#in-kubernetes) in the ConfigHub docs shows the Secret and the Deployment it applies, if you would rather write them yourself or keep the Deployment in a Unit.

The function executor connects to ConfigHub and registers the `vet-kube-linter` function with the Worker.

## Usage

The `vet-kube-linter` function takes no parameters and runs all default kube-linter checks.

    cub function do vet-kube-linter --where "Slug='my-unit'" --worker "my-space/my-worker"

If any lint violations are found, validation fails.

## How It Works

1. The function writes the resource YAML to a temporary file.
2. It executes `kube-linter lint --format json <resources>`.
3. It parses the JSON output, which includes `Checks`, `Reports`, and `Summary`.
4. Each report is mapped to a `FailedAttribute` with the check name as the issue identifier.
5. It returns a `ValidationResult` with `Passed: false` if any lint reports are found.

## Running Tests

Unit tests (require `kube-linter` CLI in PATH):

    go test -v ./...

Tests will skip automatically if the kube-linter CLI is not found. JSON parsing tests run without the binary.
