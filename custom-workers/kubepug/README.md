# kubepug Validation Example

This example demonstrates a custom ConfigHub function that checks Kubernetes resources for deprecated and deleted APIs using [kubepug](https://github.com/kubepug/kubepug). It uses the `kubepug` CLI for scanning, avoiding heavy build dependencies.

## Prerequisites

- The `kubepug` CLI must be installed and available in `PATH`. See [kubepug installation](https://github.com/kubepug/kubepug#installation).
- A running ConfigHub server (for the worker mode).

## Quick Start

Build the kubepug CLI (if not already installed):

    # From a kubepug source checkout:
    go build -o /usr/local/bin/kubepug .

    # Or install from release:
    # See https://github.com/kubepug/kubepug#installation

Build the example function executor:

    go build

### Running locally

Create a Worker, put its credentials in your shell, name the server, and start the function executor:

    cub worker create --space $SPACE my-kubepug-worker
    eval "$(cub worker get-envs --space $SPACE my-kubepug-worker)"
    export CONFIGHUB_URL=https://hub.confighub.com
    ./kubepug

The function executor reads `CONFIGHUB_WORKER_ID`, `CONFIGHUB_WORKER_SECRET` and `CONFIGHUB_URL` from its environment and connects to ConfigHub as that Worker. The `kubepug` CLI must be in PATH.

### Running in a Kubernetes cluster

Build and push a container image:

    docker build -f Dockerfile -t my-registry/kubepug-worker:latest .
    docker push my-registry/kubepug-worker:latest

Then give the cluster the Worker's credentials as a Secret and run the image with a Deployment that reads it. [`deploy-worker.sh`](../deploy-worker.sh) does both, and waits for the rollout:

    IMAGE_PULL_POLICY=IfNotPresent ../deploy-worker.sh $SPACE my-kubepug-worker confighub my-registry/kubepug-worker:latest

[External Functions](https://docs.confighub.com/guide/external-functions/#in-kubernetes) in the ConfigHub docs shows the Secret and the Deployment it applies, if you would rather write them yourself or keep the Deployment in a Unit.

The function executor connects to ConfigHub and registers the `vet-kubepug` function with the Worker.

## Usage

The `vet-kubepug` function takes a single parameter: the target Kubernetes version to check against.

    cub function do vet-kubepug 'v1.25' --where "Slug='my-unit'" --worker "my-space/my-worker"

If any deprecated or deleted APIs are found for the target version, validation fails.

## Severity Mapping

| API Status  | ConfigHub Score |
|-------------|-----------------|
| Deleted     | Critical        |
| Deprecated  | High            |

## How It Works

1. The function writes the resource YAML to a temporary file.
2. It executes `kubepug --input-file=<file> --k8s-version=<version> --format=json --error-on-deprecated --error-on-deleted`.
3. It parses the JSON output and maps deleted APIs to Critical severity and deprecated APIs to High severity.
4. It returns a `ValidationResult` with `Passed: false` if any deprecated or deleted APIs are found.
5. Each finding is attributed to the `apiVersion` path of the affected resource.

## Running Tests

Unit tests (require `kubepug` CLI in PATH):

    go test -v ./...

Tests will skip automatically if the kubepug CLI is not found. JSON parsing tests run without the binary.
