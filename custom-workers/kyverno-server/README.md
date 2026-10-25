# Kyverno Server Validation Example

This example demonstrates a custom ConfigHub function that validates Kubernetes resources against [Kyverno](https://kyverno.io/) policies by sending [AdmissionReview](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/) requests to a running Kyverno server. Unlike the [CLI-based kyverno example](../kyverno/), this approach calls the Kyverno webhook directly, avoiding the overhead of spawning a CLI process for each invocation.

## How It Works

1. For each Kubernetes resource in the configuration data, the function converts it to JSON and wraps it in a Kubernetes `AdmissionReview` request.
2. The request is POSTed to the Kyverno kyverno-resource-validating-webhook-cfg webhook's endpoint.
3. The Kyverno server evaluates the resource against all deployed policies and returns an `AdmissionResponse`.
4. The function aggregates results across all resources into a `ValidationResult` with details and failed attributes.

Policies are not passed as parameters — they must be deployed in the Kyverno cluster. This means the same policies used for admission control are also used for pre-deployment validation via ConfigHub.

## Prerequisites

- Kyverno deployed in a Kubernetes cluster with policies configured. See [Kyverno installation](https://kyverno.io/docs/installation/).
- Network access from the bot to the Kyverno webhook service.
- A ConfigHub organization you can log in to with `cub auth login`.

## Configuration

The function uses environment variables to connect to the Kyverno server:

| Variable                  | Required | Description                                                                         |
| ------------------------- | -------- | ----------------------------------------------------------------------------------- |
| `KYVERNO_URL`             | Yes      | Base URL of the Kyverno webhook (e.g., `https://kyverno-svc.kyverno.svc:443`)       |
| `KYVERNO_CA_CERT_PATH`    | No       | Path to a CA certificate file for TLS verification (for Kyverno's self-signed cert) |
| `KYVERNO_SKIP_TLS_VERIFY` | No       | Set to `true` to skip TLS certificate verification (development only)               |

## Quick Start

### Installing in a Kubernetes cluster

The kyverno-server function expects to run in a Kubernetes cluster so that it can call Kyverno's admission webhook.

To deploy the bot in a cluster, first build and push a container image:

    docker build -f Dockerfile -t my-registry/kyverno-server-bot:latest .
    docker push my-registry/kyverno-server-bot:latest

Let the bot list ValidatingWebhookConfigurations, which is how it finds the webhooks to call:

    kubectl create clusterrole webhook-reader \
      --verb=list,watch --resource=validatingwebhookconfigurations.admissionregistration.k8s.io
    kubectl create clusterrolebinding kyverno-bot-webhook-reader \
      --clusterrole=webhook-reader \
      --group="system:serviceaccounts:kyverno-bot"

In a cluster the bot runs as a service account. [`deploy-bot.sh`](../deploy-bot.sh) creates the service account, gives it Claim on a Channel, declares the bot's functions on the Channel from `--print-commands`, puts a new key for the service account in a Secret, and runs the image with a Deployment that reads it. It waits until the bot holds the Channel:

    IMAGE_PULL_POLICY=IfNotPresent ../deploy-bot.sh $SPACE kyverno-bot kyverno-bot my-registry/kyverno-server-bot:latest \
      KYVERNO_URL=https://kyverno-svc.kyverno.svc:443 \
      KYVERNO_SKIP_TLS_VERIFY=true

For a complete end-to-end demo using Kind, see [demo.sh](demo.sh).

The bot serves `vet-kyverno-server` on the Channel `$SPACE/kyverno-bot`.

### Running out-of-cluster

You can also run the bot outside the cluster using `kubectl port-forward` to access the Kyverno webhook:

    # Port-forward the Kyverno webhook service
    kubectl -n kyverno port-forward svc/kyverno-svc 8443:443 &

    # Create a Channel for the bot
    cub channel create --space $SPACE kyverno-server

    # Set Kyverno environment
    export KYVERNO_URL=https://localhost:8443
    export KYVERNO_SKIP_TLS_VERIFY=true

    # Run the bot, as you, through your active cub context
    ./kyverno-server $SPACE/kyverno-server
