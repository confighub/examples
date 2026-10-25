# kube-score Validation Example

This example demonstrates a custom ConfigHub function that validates Kubernetes resources using [kube-score](https://kube-score.com/) best-practice checks. It uses the `kube-score` CLI for scoring, avoiding heavy build dependencies.

## Prerequisites

- The `kube-score` CLI must be installed and available in `PATH`. See [kube-score installation](https://github.com/zegl/kube-score#installation).
- A ConfigHub organization you can log in to with `cub auth login`.

## Quick Start

Build the kube-score CLI (if not already installed):

    # From a kube-score source checkout:
    go build -o /usr/local/bin/kube-score ./cmd/kube-score/

    # Or install from release:
    # See https://github.com/zegl/kube-score#installation

Build the example function bot:

    go build

### Running locally

Create a Channel and start the bot on it:

    cub channel create --space $SPACE kube-score
    ./kube-score $SPACE/kube-score

The bot authenticates as you, through your active cub context, and declares the `vet-kube-score` function on the Channel when it starts. The `kube-score` CLI must be in PATH.

### Running in a Kubernetes cluster

Build and push a container image:

    docker build -f Dockerfile -t my-registry/kube-score-bot:latest .
    docker push my-registry/kube-score-bot:latest

In a cluster the bot runs as a service account. [`deploy-bot.sh`](../deploy-bot.sh) creates the service account, gives it Claim on a Channel, declares the bot's functions on the Channel from `--print-commands`, puts a new key for the service account in a Secret, and runs the image with a Deployment that reads it. It waits until the bot holds the Channel:

    IMAGE_PULL_POLICY=IfNotPresent ../deploy-bot.sh $SPACE kube-score-bot confighub my-registry/kube-score-bot:latest

The bot serves the Channel `$SPACE/kube-score-bot`.

## Usage

The `vet-kube-score` function takes a single parameter: a score threshold that determines when validation fails.

    cub function do vet-kube-score 'Critical' --where "Slug='my-unit'" --channel "my-space/kube-score"

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
