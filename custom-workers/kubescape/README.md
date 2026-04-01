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

### Running locally with `cub worker run`

The simplest way to run the example is with `cub worker run`, which automatically creates the worker and sets up the environment:

    cub worker run --space $SPACE --executable ./kubescape my-kubescape-worker

This will create the worker if it doesn't exist, set the required environment variables (`CONFIGHUB_WORKER_ID`, `CONFIGHUB_WORKER_SECRET`, `CONFIGHUB_URL`), and start the executable. The `kubescape` CLI must be in PATH.

### Running directly with environment variables

Alternatively, you can set up the environment manually:

    eval "$(cub worker get-envs --space $SPACE my-kubescape-worker)"
    ./kubescape

### Installing in a Kubernetes cluster

To deploy the worker in a Kubernetes cluster, first build and push a container image:

    docker build -f Dockerfile -t my-registry/kubescape-worker:latest .
    docker push my-registry/kubescape-worker:latest

Then install using `cub worker install`:

    # Create the worker unit in ConfigHub
    cub worker install --space $SPACE \
      --unit kubescape-worker-unit \
      --target $TARGET \
      --image my-registry/kubescape-worker:latest \
      my-kubescape-worker

    # Apply the worker unit to the cluster
    cub unit apply --space $SPACE kubescape-worker-unit

    # Wait for the namespace and deployment, then install the secret
    kubectl -n confighub wait --for=create deployment/my-kubescape-worker --timeout=120s
    cub worker install --space $SPACE \
      --export-secret-only \
      -n confighub \
      my-kubescape-worker 2>/dev/null | kubectl apply -f -

    # Wait for the worker to be ready
    kubectl -n confighub rollout status deployment/my-kubescape-worker --timeout=120s

The worker connects to ConfigHub and registers the `vet-kubescape` function alongside the standard built-in functions.

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
