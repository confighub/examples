# kubescape Validation Example

This example demonstrates a custom ConfigHub function that validates Kubernetes resources against security controls using [kubescape](https://kubescape.io/). It uses the `kubescape` CLI for scanning, avoiding heavy build dependencies.

## Prerequisites

- The `kubescape` CLI must be installed and available in `PATH`. See [kubescape installation](https://github.com/kubescape/kubescape/blob/master/docs/getting-started.md).
- A running ConfigHub server (for the worker mode).

## Quick Start

Build the kubescape CLI (if not already installed):

    # From a kubescape source checkout:
    go build -o /usr/local/bin/kubescape .

    # Or install from release:
    # See https://github.com/kubescape/kubescape#install

Build the example worker:

    go build

### Running locally

Create the Worker, put its credentials in your shell, name the server, and start the executable:

    cub worker create --space $SPACE my-kubescape-worker
    eval "$(cub worker get-envs --space $SPACE my-kubescape-worker)"
    export CONFIGHUB_URL=https://hub.confighub.com
    ./kubescape

The executable reads `CONFIGHUB_WORKER_ID`, `CONFIGHUB_WORKER_SECRET` and `CONFIGHUB_URL` from its environment and connects to ConfigHub as that Worker. The `kubescape` CLI must be in PATH.

### Running in a Kubernetes cluster

Build and push a container image:

    docker build -f Dockerfile -t my-registry/kubescape-worker:latest .
    docker push my-registry/kubescape-worker:latest

Then give the cluster the Worker's credentials as a Secret and run the image with a Deployment that reads it. [`deploy-worker.sh`](../deploy-worker.sh) does both, and waits for the rollout:

    IMAGE_PULL_POLICY=IfNotPresent ../deploy-worker.sh $SPACE my-kubescape-worker confighub my-registry/kubescape-worker:latest

[External Functions](https://docs.confighub.com/guide/external-functions/#in-kubernetes) in the ConfigHub docs shows the Secret and the Deployment it applies, if you would rather write them yourself or keep the Deployment in a Unit.

The worker connects to ConfigHub and registers the `vet-kubescape` function.

## Usage

The `vet-kubescape` function takes no parameters and runs all default kubescape security controls.

    cub function do vet-kubescape --where "Slug='my-unit'" --worker "my-space/my-worker"

If any security controls fail, validation fails.

## Severity Mapping

Kubescape severity levels map directly to ConfigHub scores:

| kubescape Severity | ConfigHub Score |
|--------------------|-----------------|
| Critical           | Critical        |
| High               | High            |
| Medium             | Medium          |
| Low                | Low             |

## How It Works

1. The function writes the resource YAML to a temporary file.
2. It executes `kubescape scan <file> --format json --output <output-file>`.
3. It parses the JSON output, which includes summary details and per-resource control results.
4. Each failed control is mapped to a `FailedAttribute` with the control ID as the issue identifier. Where available, failed paths from the control rules are used to attribute findings to specific YAML paths.
5. It returns a `ValidationResult` with `Passed: false` if any controls fail.

## Running Tests

Unit tests (require `kubescape` CLI in PATH):

    go test -v ./...

Tests will skip automatically if the kubescape CLI is not found. JSON parsing and helper tests run without the binary.
