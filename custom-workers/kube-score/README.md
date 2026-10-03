# kube-score Validation Example

This example demonstrates a custom ConfigHub function that validates Kubernetes resources using [kube-score](https://kube-score.com/) best-practice checks. It uses the `kube-score` CLI for scoring, avoiding heavy build dependencies.

## Prerequisites

- The `kube-score` CLI must be installed and available in `PATH`. See [kube-score installation](https://github.com/zegl/kube-score#installation).
- A running ConfigHub server (for the worker mode).

## Quick Start

Build the kube-score CLI (if not already installed):

    # From a kube-score source checkout:
    go build -o /usr/local/bin/kube-score ./cmd/kube-score/

    # Or install from release:
    # See https://github.com/zegl/kube-score#installation

Build the example function executor:

    go build

### Running locally

Create a Worker, put its credentials in your shell, name the server, and start the function executor:

    cub worker create --space $SPACE my-kube-score-worker
    eval "$(cub worker get-envs --space $SPACE my-kube-score-worker)"
    export CONFIGHUB_URL=https://hub.confighub.com
    ./kube-score

The function executor reads `CONFIGHUB_WORKER_ID`, `CONFIGHUB_WORKER_SECRET` and `CONFIGHUB_URL` from its environment and connects to ConfigHub as that Worker. The `kube-score` CLI must be in PATH.

### Running in a Kubernetes cluster

Build and push a container image:

    docker build -f Dockerfile -t my-registry/kube-score-worker:latest .
    docker push my-registry/kube-score-worker:latest

Then give the cluster the Worker's credentials as a Secret and run the image with a Deployment that reads it. [`deploy-worker.sh`](../deploy-worker.sh) does both, and waits for the rollout:

    IMAGE_PULL_POLICY=IfNotPresent ../deploy-worker.sh $SPACE my-kube-score-worker confighub my-registry/kube-score-worker:latest

[External Functions](https://docs.confighub.com/guide/external-functions/#in-kubernetes) in the ConfigHub docs shows the Secret and the Deployment it applies, if you would rather write them yourself or keep the Deployment in a Unit.

The function executor connects to ConfigHub and registers the `vet-kube-score` function with the Worker.

## Usage

The `vet-kube-score` function takes a single parameter: a score threshold that determines when validation fails.

    cub function do vet-kube-score 'Critical' --where "Slug='my-unit'" --worker "my-space/my-worker"

Possible threshold values: `Critical`, `High`, `Medium`, `Low`. If any finding has a score at or above the threshold, validation fails.

## Score Mapping

kube-score grades are mapped to ConfigHub scores:

| kube-score Grade | ConfigHub Score |
|------------------|-----------------|
| CRITICAL (1)     | Critical        |
| WARNING (5)      | Medium          |
| AlmostOK (7)     | Low             |
| AllOK (10)       | None (ignored)  |

## How It Works

1. The function writes the resource YAML to a temporary file.
2. It executes `kube-score score --output-format json <resources>`.
3. It parses the JSON output, maps grades to scores, and resolves container paths to gaby dot notation.
4. It returns a `ValidationResult` with `MaxScore`, failed attributes (each with a score), and details.
5. If `MaxScore >= threshold`, `Passed` is false; otherwise true.

## Running Tests

Unit tests (require `kube-score` CLI in PATH):

    go test -v ./...

Tests will skip automatically if the kube-score CLI is not found. Several unit tests for helper functions run without the binary.
