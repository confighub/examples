# OPA Gatekeeper Validation Example

This example demonstrates a custom ConfigHub function that validates Kubernetes resources against [OPA Gatekeeper](https://open-policy-agent.github.io/gatekeeper/) constraints by sending [AdmissionReview](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/) requests to a running Gatekeeper server.

## How It Works

1. For each Kubernetes resource in the configuration data, the function converts it to JSON and wraps it in a Kubernetes `AdmissionReview` request.
2. The request is POSTed to the Gatekeeper webhook endpoint.
3. Gatekeeper evaluates the resource against all deployed constraint templates and constraints, and returns an `AdmissionResponse`.
4. The function aggregates results across all resources into a `ValidationResult` with details and failed attributes.

Constraints are not passed as parameters — they must be deployed in the Gatekeeper cluster. This means the same constraints used for admission control are also used for pre-deployment validation via ConfigHub.

## Prerequisites

- OPA Gatekeeper deployed in a Kubernetes cluster with constraint templates and constraints configured. See [Gatekeeper installation](https://open-policy-agent.github.io/gatekeeper/website/docs/install/).
- Network access from the bot to the Gatekeeper webhook service.
- A ConfigHub organization you can log in to with `cub auth login`.

## Configuration

The function uses environment variables to connect to the Gatekeeper server:

| Variable                     | Required | Description                                                                         |
| ---------------------------- | -------- | ----------------------------------------------------------------------------------- |
| `GATEKEEPER_URL`             | Yes      | Base URL of the Gatekeeper webhook (e.g., `https://gatekeeper-webhook-service.gatekeeper-system.svc:443`) |
| `GATEKEEPER_CA_CERT_PATH`    | No       | Path to a CA certificate file for TLS verification (for Gatekeeper's self-signed cert) |
| `GATEKEEPER_SKIP_TLS_VERIFY` | No       | Set to `true` to skip TLS certificate verification (development only)               |

## Quick Start

### Installing in a Kubernetes cluster

The opa-gatekeeper function expects to run in a Kubernetes cluster so that it can call Gatekeeper's admission webhook.

To deploy the bot in a cluster, first build and push a container image:

    docker build -f Dockerfile -t my-registry/opa-gatekeeper-bot:latest .
    docker push my-registry/opa-gatekeeper-bot:latest

Let the bot list ValidatingWebhookConfigurations, which is how it finds the webhooks to call:

    kubectl create clusterrole webhook-reader \
      --verb=list,watch --resource=validatingwebhookconfigurations.admissionregistration.k8s.io
    kubectl create clusterrolebinding gatekeeper-bot-webhook-reader \
      --clusterrole=webhook-reader \
      --group="system:serviceaccounts:gatekeeper-bot"

In a cluster the bot runs as a service account. [`deploy-bot.sh`](../deploy-bot.sh) creates the service account, gives it Claim on a Channel, declares the bot's functions on the Channel from `--print-commands`, puts a new key for the service account in a Secret, and runs the image with a Deployment that reads it. It waits until the bot holds the Channel:

    IMAGE_PULL_POLICY=IfNotPresent ../deploy-bot.sh $SPACE gatekeeper-bot gatekeeper-bot my-registry/opa-gatekeeper-bot:latest \
      GATEKEEPER_URL=https://gatekeeper-webhook-service.gatekeeper-system.svc:443 \
      GATEKEEPER_SKIP_TLS_VERIFY=true

For a complete end-to-end demo using Kind, see [demo.sh](demo.sh).

The bot serves `vet-opa-gatekeeper` on the Channel `$SPACE/gatekeeper-bot`.

### Running out-of-cluster

You can also run the bot outside the cluster using `kubectl port-forward` to access the Gatekeeper webhook:

    # Port-forward the Gatekeeper webhook service
    kubectl -n gatekeeper-system port-forward svc/gatekeeper-webhook-service 8443:443 &

    # Create a Channel for the bot
    cub channel create --space $SPACE opa-gatekeeper

    # Set Gatekeeper environment
    export GATEKEEPER_URL=https://localhost:8443
    export GATEKEEPER_SKIP_TLS_VERIFY=true

    # Run the bot, as you, through your active cub context
    ./opa-gatekeeper $SPACE/opa-gatekeeper
