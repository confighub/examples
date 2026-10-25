# Kyverno Policy Validation Example

This example demonstrates a custom ConfigHub function that validates Kubernetes resources against [Kyverno](https://kyverno.io/) policies and Kubernetes [ValidatingAdmissionPolicy](https://kubernetes.io/docs/reference/access-authn-authz/validating-admission-policy/) resources. It uses the `kyverno` CLI for offline validation, avoiding heavy build dependencies.

## Prerequisites

- The `kyverno` CLI must be installed and available in `PATH`. See [Kyverno CLI installation](https://kyverno.io/docs/installation/#install-kyverno-cli).
- A ConfigHub organization you can log in to with `cub auth login`.

## Quick Start

Build the kyverno CLI (if not already installed):

    # From a kyverno source checkout:
    go build -o /usr/local/bin/kyverno ./cmd/cli/kubectl-kyverno/

    # Or install from release:
    # See https://kyverno.io/docs/installation/#install-kyverno-cli

Build the example function bot:

    go build

### Running locally

Create a Channel and start the bot on it:

    cub channel create --space $SPACE kyverno
    ./kyverno $SPACE/kyverno

The bot authenticates as you, through your active cub context, and declares the `vet-kyverno` function on the Channel when it starts. The `kyverno` CLI must be in PATH.

### Running in a Kubernetes cluster

Build and push a container image:

    docker build -f Dockerfile -t my-registry/kyverno-bot:latest .
    docker push my-registry/kyverno-bot:latest

In a cluster the bot runs as a service account. [`deploy-bot.sh`](../deploy-bot.sh) creates the service account, gives it Claim on a Channel, declares the bot's functions on the Channel from `--print-commands`, puts a new key for the service account in a Secret, and runs the image with a Deployment that reads it. It waits until the bot holds the Channel:

    IMAGE_PULL_POLICY=IfNotPresent ../deploy-bot.sh $SPACE kyverno-bot confighub my-registry/kyverno-bot:latest

The bot serves the Channel `$SPACE/kyverno-bot`.

## Usage

The `vet-kyverno` function takes a single parameter: a YAML document containing one or more Kyverno policies (ValidatingPolicy, ClusterPolicy, or Policy resources) or Kubernetes [ValidatingAdmissionPolicy](https://kubernetes.io/docs/reference/access-authn-authz/validating-admission-policy/) resources.

    cub function do vet-kyverno '<policy-yaml>' --where "Slug='my-unit'" --channel "my-space/kyverno"

Policies from https://kyverno.io/policies/ can be used directly. Kubernetes native ValidatingAdmissionPolicy resources are also supported, allowing pre-deployment validation with the same CEL-based policies used for admission control.

## How It Works

1. The function writes the policy YAML and resource YAML to temporary files.
2. It executes `kyverno apply <policy> --resource <resources> --policy-report --output-format=json`.
3. It parses the JSON policy report to extract policy/rule failures and field paths (converted from JSON Pointer to dot notation).
4. It returns a `ValidationResult` with details, failed attributes, and paths where available.

## End-to-End Demo

For a complete end-to-end demo using Kind, see [demo.sh](demo.sh). Run it from this directory:

    bash demo.sh

The demo creates a Kind cluster, deploys the kyverno CLI function bot, and validates test resources against both Kyverno ValidatingPolicy and Kubernetes ValidatingAdmissionPolicy.

## Running Tests

Unit tests (require `kyverno` CLI in PATH):

    go test -v ./...

Tests will skip automatically if the kyverno CLI is not found.
