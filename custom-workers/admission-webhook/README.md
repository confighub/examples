# Admission Webhook Validation Example

This example demonstrates custom ConfigHub functions that validate Kubernetes resources against the policies of an admission controller running in a cluster, by sending [AdmissionReview](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/) requests to its webhook. One bot serves two functions:

| Function             | Admission controller                                              | Policies                                     |
| -------------------- | ----------------------------------------------------------------- | -------------------------------------------- |
| `vet-kyverno-server` | [Kyverno](https://kyverno.io/)                                    | Kyverno policies                             |
| `vet-opa-gatekeeper` | [OPA Gatekeeper](https://open-policy-agent.github.io/gatekeeper/) | Gatekeeper constraint templates and constraints |

Unlike the [CLI-based kyverno example](../kyverno/), this approach calls the Kyverno webhook directly, avoiding the overhead of spawning a CLI process for each invocation.

## How It Works

1. For each Kubernetes resource in the configuration data, the function converts it to JSON and wraps it in a Kubernetes `AdmissionReview` request.
2. The request is POSTed to the admission controller's webhook endpoints, which the bot finds by watching the cluster's ValidatingWebhookConfigurations: `kyverno-resource-validating-webhook-cfg` for Kyverno, `gatekeeper-validating-webhook-configuration` for Gatekeeper.
3. The admission controller evaluates the resource against everything deployed in it and returns an `AdmissionResponse`.
4. The function aggregates results across all resources into a `ValidationResult` with details and failed attributes.

Policies are not passed as parameters — they must be deployed in the cluster. This means the same policies used for admission control are also used for pre-deployment validation via ConfigHub.

The code is in three packages: [`webhook`](webhook/) sends AdmissionReviews and discovers webhook endpoints, and [`kyverno`](kyverno/) and [`gatekeeper`](gatekeeper/) turn each controller's responses into validation results. To support another admission controller, add a package like those two and register its function in [`main.go`](main.go).

## Prerequisites

- Kyverno, OPA Gatekeeper or both deployed in a Kubernetes cluster with policies configured. See [Kyverno installation](https://kyverno.io/docs/installation/) and [Gatekeeper installation](https://open-policy-agent.github.io/gatekeeper/website/docs/install/).
- Network access from the bot to the webhook services.
- A ConfigHub organization you can log in to with `cub auth login`.

## Configuration

The bot declares both functions on its Channel, and serves each whose webhook URL it is given. Set one URL or both:

| Variable                     | Description                                                                                                          |
| ---------------------------- | -------------------------------------------------------------------------------------------------------------------- |
| `KYVERNO_URL`                | Base URL of the Kyverno webhook (e.g., `https://kyverno-svc.kyverno.svc:443`); serves `vet-kyverno-server`           |
| `KYVERNO_CA_CERT_PATH`       | Path to a CA certificate file for TLS verification (for Kyverno's self-signed cert)                                  |
| `KYVERNO_SKIP_TLS_VERIFY`    | Set to `true` to skip TLS certificate verification (development only)                                                |
| `GATEKEEPER_URL`             | Base URL of the Gatekeeper webhook (e.g., `https://gatekeeper-webhook-service.gatekeeper-system.svc:443`); serves `vet-opa-gatekeeper` |
| `GATEKEEPER_CA_CERT_PATH`    | Path to a CA certificate file for TLS verification (for Gatekeeper's self-signed cert)                               |
| `GATEKEEPER_SKIP_TLS_VERIFY` | Set to `true` to skip TLS certificate verification (development only)                                                |

Invoking a function whose URL is not set fails with a message naming the missing variable.

## Quick Start

### Installing in a Kubernetes cluster

The bot expects to run in a Kubernetes cluster so that it can call the admission webhooks.

To deploy the bot in a cluster, first build and push a container image:

    docker build -f Dockerfile -t my-registry/admission-webhook-bot:latest .
    docker push my-registry/admission-webhook-bot:latest

Let the bot list ValidatingWebhookConfigurations, which is how it finds the webhooks to call:

    kubectl create clusterrole webhook-reader \
      --verb=list,watch --resource=validatingwebhookconfigurations.admissionregistration.k8s.io
    kubectl create clusterrolebinding admission-bot-webhook-reader \
      --clusterrole=webhook-reader \
      --group="system:serviceaccounts:admission-bot"

In a cluster the bot runs as a service account. [`deploy-bot.sh`](../deploy-bot.sh) creates the service account, gives it Claim on a Channel, declares the bot's functions on the Channel from `--print-commands`, puts a new key for the service account in a Secret, and runs the image with a Deployment that reads it. It waits until the bot holds the Channel. For a cluster running both admission controllers:

    IMAGE_PULL_POLICY=IfNotPresent ../deploy-bot.sh $SPACE admission-bot admission-bot my-registry/admission-webhook-bot:latest \
      KYVERNO_URL=https://kyverno-svc.kyverno.svc:443 \
      KYVERNO_SKIP_TLS_VERIFY=true \
      GATEKEEPER_URL=https://gatekeeper-webhook-service.gatekeeper-system.svc:443 \
      GATEKEEPER_SKIP_TLS_VERIFY=true

The bot serves `vet-kyverno-server` and `vet-opa-gatekeeper` on the Channel `$SPACE/admission-bot`.

For complete end-to-end demos using Kind, see [demo-kyverno.sh](demo-kyverno.sh) and [demo-gatekeeper.sh](demo-gatekeeper.sh).

### Running out-of-cluster

You can also run the bot outside the cluster using `kubectl port-forward` to access a webhook, here Kyverno's:

    # Port-forward the Kyverno webhook service
    kubectl -n kyverno port-forward svc/kyverno-svc 8443:443 &

    # Create a Channel for the bot
    cub channel create --space $SPACE admission-webhook

    # Set Kyverno environment
    export KYVERNO_URL=https://localhost:8443
    export KYVERNO_SKIP_TLS_VERIFY=true

    # Build the bot and run it, as you, through your active cub context
    go build .
    ./admission-webhook $SPACE/admission-webhook

For Gatekeeper, port-forward `svc/gatekeeper-webhook-service` in `gatekeeper-system` and set `GATEKEEPER_URL` and `GATEKEEPER_SKIP_TLS_VERIFY` instead.
