# kubepug Validation Example

This example demonstrates a custom ConfigHub function that checks Kubernetes resources for deprecated and deleted APIs using [kubepug](https://github.com/kubepug/kubepug). It uses the `kubepug` CLI for scanning, avoiding heavy build dependencies.

## Prerequisites

- The `kubepug` CLI must be installed and available in `PATH`. See [kubepug installation](https://github.com/kubepug/kubepug#installation).
- A ConfigHub organization you can log in to with `cub auth login`.

## Quick Start

Build the kubepug CLI (if not already installed):

    # From a kubepug source checkout:
    go build -o /usr/local/bin/kubepug .

    # Or install from release:
    # See https://github.com/kubepug/kubepug#installation

Build the example function bot:

    go build

### Running locally

Create a Channel and start the bot on it:

    cub channel create --space $SPACE kubepug
    ./kubepug $SPACE/kubepug

The bot authenticates as you, through your active cub context, and declares the `vet-kubepug` function on the Channel when it starts. The `kubepug` CLI must be in PATH.

### Running in a Kubernetes cluster

Build and push a container image:

    docker build -f Dockerfile -t my-registry/kubepug-bot:latest .
    docker push my-registry/kubepug-bot:latest

In a cluster the bot runs as a service account. [`deploy-bot.sh`](../deploy-bot.sh) creates the service account, gives it Claim on a Channel, declares the bot's functions on the Channel from `--print-commands`, puts a new key for the service account in a Secret, and runs the image with a Deployment that reads it. It waits until the bot holds the Channel:

    IMAGE_PULL_POLICY=IfNotPresent ../deploy-bot.sh $SPACE kubepug-bot confighub my-registry/kubepug-bot:latest

The bot serves the Channel `$SPACE/kubepug-bot`.

## Usage

The `vet-kubepug` function takes a single parameter: the target Kubernetes version to check against.

    cub function do vet-kubepug 'v1.25' --where "Slug='my-unit'" --channel "my-space/kubepug"

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
