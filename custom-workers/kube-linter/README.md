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

Build the example worker:

    go build

### Running locally with `cub worker run`

The simplest way to run the example is with `cub worker run`, which automatically creates the worker and sets up the environment:

    cub worker run --space $SPACE --executable ./kube-linter my-kube-linter-worker

This will create the worker if it doesn't exist, set the required environment variables (`CONFIGHUB_WORKER_ID`, `CONFIGHUB_WORKER_SECRET`, `CONFIGHUB_URL`), and start the executable. The `kube-linter` CLI must be in PATH.

### Running directly with environment variables

Alternatively, you can set up the environment manually:

    eval "$(cub worker get-envs --space $SPACE my-kube-linter-worker)"
    ./kube-linter

### Installing in a Kubernetes cluster

To deploy the worker in a Kubernetes cluster, first build and push a container image:

    docker build -f Dockerfile -t my-registry/kube-linter-worker:latest .
    docker push my-registry/kube-linter-worker:latest

Then install using `cub worker install`:

    # Create the worker unit in ConfigHub
    cub worker install --space $SPACE \
      --unit kube-linter-worker-unit \
      --target $TARGET \
      --image my-registry/kube-linter-worker:latest \
      my-kube-linter-worker

    # Apply the worker unit to the cluster
    cub unit apply --space $SPACE kube-linter-worker-unit

    # Wait for the namespace and deployment, then install the secret
    kubectl -n confighub wait --for=create deployment/my-kube-linter-worker --timeout=120s
    cub worker install --space $SPACE \
      --export-secret-only \
      -n confighub \
      my-kube-linter-worker 2>/dev/null | kubectl apply -f -

    # Wait for the worker to be ready
    kubectl -n confighub rollout status deployment/my-kube-linter-worker --timeout=120s

The worker connects to ConfigHub and registers the `vet-kube-linter` function alongside the standard built-in functions.

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
