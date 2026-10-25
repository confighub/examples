# kubescape Validation Example

This example demonstrates a custom ConfigHub function that validates Kubernetes resources against security controls using [kubescape](https://kubescape.io/). It uses the `kubescape` CLI for scanning, avoiding heavy build dependencies.

## Prerequisites

- The `kubescape` CLI must be installed and available in `PATH`. See [kubescape installation](https://github.com/kubescape/kubescape/blob/master/docs/getting-started.md).
- A ConfigHub organization you can log in to with `cub auth login`.

## Quick Start

Build the kubescape CLI (if not already installed):

    # From a kubescape source checkout:
    go build -o /usr/local/bin/kubescape .

    # Or install from release:
    # See https://github.com/kubescape/kubescape#install

Build the example function bot:

    go build

### Running locally

Create a Channel and start the bot on it:

    cub channel create --space $SPACE kubescape
    ./kubescape $SPACE/kubescape

The bot authenticates as you, through your active cub context, and declares the `vet-kubescape` function on the Channel when it starts. The `kubescape` CLI must be in PATH.

### Running in a Kubernetes cluster

Build and push a container image:

    docker build -f Dockerfile -t my-registry/kubescape-bot:latest .
    docker push my-registry/kubescape-bot:latest

In a cluster the bot runs as a service account. [`deploy-bot.sh`](../deploy-bot.sh) creates the service account, gives it Claim on a Channel, declares the bot's functions on the Channel from `--print-commands`, puts a new key for the service account in a Secret, and runs the image with a Deployment that reads it. It waits until the bot holds the Channel:

    IMAGE_PULL_POLICY=IfNotPresent ../deploy-bot.sh $SPACE kubescape-bot confighub my-registry/kubescape-bot:latest

The bot serves the Channel `$SPACE/kubescape-bot`.

## Usage

The `vet-kubescape` function takes no parameters and runs all default kubescape security controls.

    cub function do vet-kubescape --where "Slug='my-unit'" --channel "my-space/kubescape"

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
