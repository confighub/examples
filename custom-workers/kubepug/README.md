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

Build the example worker:

    go build

### Running locally with `cub worker run`

The simplest way to run the example is with `cub worker run`, which automatically creates the worker and sets up the environment:

    cub worker run --space $SPACE --executable ./kubepug my-kubepug-worker

This will create the worker if it doesn't exist, set the required environment variables (`CONFIGHUB_WORKER_ID`, `CONFIGHUB_WORKER_SECRET`, `CONFIGHUB_URL`), and start the executable. The `kubepug` CLI must be in PATH.

### Running directly with environment variables

Alternatively, you can set up the environment manually:

    eval "$(cub worker get-envs --space $SPACE my-kubepug-worker)"
    ./kubepug

### Installing in a Kubernetes cluster

To deploy the worker in a Kubernetes cluster, first build and push a container image:

    docker build -f Dockerfile -t my-registry/kubepug-worker:latest .
    docker push my-registry/kubepug-worker:latest

Then install using `cub worker install`:

    # Create the worker unit in ConfigHub
    cub worker install --space $SPACE \
      --unit kubepug-worker-unit \
      --target $TARGET \
      --image my-registry/kubepug-worker:latest \
      my-kubepug-worker

    # Apply the worker unit to the cluster
    cub unit apply --space $SPACE kubepug-worker-unit

    # Wait for the namespace and deployment, then install the secret
    kubectl -n confighub wait --for=create deployment/my-kubepug-worker --timeout=120s
    cub worker install --space $SPACE \
      --export-secret-only \
      -n confighub \
      my-kubepug-worker 2>/dev/null | kubectl apply -f -

    # Wait for the worker to be ready
    kubectl -n confighub rollout status deployment/my-kubepug-worker --timeout=120s

The worker connects to ConfigHub and registers the `vet-kubepug` function alongside the standard built-in functions.

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
