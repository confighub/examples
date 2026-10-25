# kube-linter Validation Example

This example demonstrates a custom ConfigHub function that validates Kubernetes resources using [kube-linter](https://github.com/stackrox/kube-linter) best-practice checks. It uses the `kube-linter` CLI for linting, avoiding heavy build dependencies.

## Prerequisites

- The `kube-linter` CLI must be installed and available in `PATH`. See [kube-linter installation](https://github.com/stackrox/kube-linter#installing-kubelinter).
- A ConfigHub organization you can log in to with `cub auth login`.

## Quick Start

Build the kube-linter CLI (if not already installed):

    # From a kube-linter source checkout:
    go build -o /usr/local/bin/kube-linter ./cmd/kube-linter/

    # Or install from release:
    # See https://github.com/stackrox/kube-linter#installing-kubelinter

Build the example function bot:

    go build

### Running locally

Create a Channel and start the bot on it:

    cub channel create --space $SPACE kube-linter
    ./kube-linter $SPACE/kube-linter

The bot authenticates as you, through your active cub context, and declares the `vet-kube-linter` function on the Channel when it starts. The `kube-linter` CLI must be in PATH.

### Running in a Kubernetes cluster

Build and push a container image:

    docker build -f Dockerfile -t my-registry/kube-linter-bot:latest .
    docker push my-registry/kube-linter-bot:latest

In a cluster the bot runs as a service account. [`deploy-bot.sh`](../deploy-bot.sh) creates the service account, gives it Claim on a Channel, declares the bot's functions on the Channel from `--print-commands`, puts a new key for the service account in a Secret, and runs the image with a Deployment that reads it. It waits until the bot holds the Channel:

    IMAGE_PULL_POLICY=IfNotPresent ../deploy-bot.sh $SPACE kube-linter-bot confighub my-registry/kube-linter-bot:latest

The bot serves the Channel `$SPACE/kube-linter-bot`.

## Usage

The `vet-kube-linter` function takes no parameters and runs all default kube-linter checks.

    cub function do vet-kube-linter --where "Slug='my-unit'" --channel "my-space/kube-linter"

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
