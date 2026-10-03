# Kyverno Policy Validation Example

This example demonstrates a custom ConfigHub function that validates Kubernetes resources against [Kyverno](https://kyverno.io/) policies and Kubernetes [ValidatingAdmissionPolicy](https://kubernetes.io/docs/reference/access-authn-authz/validating-admission-policy/) resources. It uses the `kyverno` CLI for offline validation, avoiding heavy build dependencies.

## Prerequisites

- The `kyverno` CLI must be installed and available in `PATH`. See [Kyverno CLI installation](https://kyverno.io/docs/installation/#install-kyverno-cli).
- A running ConfigHub server (for the worker mode).

## Quick Start

Build the kyverno CLI (if not already installed):

    # From a kyverno source checkout:
    go build -o /usr/local/bin/kyverno ./cmd/cli/kubectl-kyverno/

    # Or install from release:
    # See https://kyverno.io/docs/installation/#install-kyverno-cli

Build the example worker:

    go build

### Running locally

Create the Worker, put its credentials in your shell, name the server, and start the executable:

    cub worker create --space $SPACE my-kyverno-worker
    eval "$(cub worker get-envs --space $SPACE my-kyverno-worker)"
    export CONFIGHUB_URL=https://hub.confighub.com
    ./kyverno

The executable reads `CONFIGHUB_WORKER_ID`, `CONFIGHUB_WORKER_SECRET` and `CONFIGHUB_URL` from its environment and connects to ConfigHub as that Worker. The `kyverno` CLI must be in PATH.

### Running in a Kubernetes cluster

Build and push a container image:

    docker build -f Dockerfile -t my-registry/kyverno-worker:latest .
    docker push my-registry/kyverno-worker:latest

Then give the cluster the Worker's credentials as a Secret and run the image with a Deployment that reads it. [`deploy-worker.sh`](../deploy-worker.sh) does both, and waits for the rollout:

    IMAGE_PULL_POLICY=IfNotPresent ../deploy-worker.sh $SPACE my-kyverno-worker confighub my-registry/kyverno-worker:latest

[External Functions](https://docs.confighub.com/guide/external-functions/#in-kubernetes) in the ConfigHub docs shows the Secret and the Deployment it applies, if you would rather write them yourself or keep the Deployment in a Unit.

The worker connects to ConfigHub and registers the `vet-kyverno` function.

## Usage

The `vet-kyverno` function takes a single parameter: a YAML document containing one or more Kyverno policies (ValidatingPolicy, ClusterPolicy, or Policy resources) or Kubernetes [ValidatingAdmissionPolicy](https://kubernetes.io/docs/reference/access-authn-authz/validating-admission-policy/) resources.

    cub function do vet-kyverno '<policy-yaml>' --where "Slug='my-unit'" --worker "my-space/my-worker"

Policies from https://kyverno.io/policies/ can be used directly. Kubernetes native ValidatingAdmissionPolicy resources are also supported, allowing pre-deployment validation with the same CEL-based policies used for admission control.

## How It Works

1. The function writes the policy YAML and resource YAML to temporary files.
2. It executes `kyverno apply <policy> --resource <resources> --policy-report --output-format=json`.
3. It parses the JSON policy report to extract policy/rule failures and field paths (converted from JSON Pointer to dot notation).
4. It returns a `ValidationResult` with details, failed attributes, and paths where available.

## End-to-End Demo

For a complete end-to-end demo using Kind, see [demo.sh](demo.sh). Run from the `public/` directory:

    bash examples/kyverno/demo.sh

The demo creates a Kind cluster, deploys a kyverno CLI worker, and validates test resources against both Kyverno ValidatingPolicy and Kubernetes ValidatingAdmissionPolicy.

## Running Tests

Unit tests (require `kyverno` CLI in PATH):

    go test -v ./...

Tests will skip automatically if the kyverno CLI is not found.
