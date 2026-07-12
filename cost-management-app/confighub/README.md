# ConfigHub Self-Management Pack

This directory defines how the generated operational app manages itself through
ConfigHub.

The app has two governed planes:

| Plane | Meaning |
|---|---|
| The app's job | The operational workflow described in `data/operational-workflow.json`. |
| The app itself | This app's config, Variants, OAuth client, deployment, promotion, lifecycle, security, fleet placement, and proof. |

## Delivery Contract

- Component: `cost-management-app`
- Status: `WATCH`
- Target provider: `OCI`
- Controller: `Argo`
- Publication: `cub release publish`
- Route: `ConfigHub app config -> OCI artifact -> Argo -> Kubernetes`
- Kubernetes namespace: `cost-management-app`

## Variant Config

The `variants/` files are starter ConfigHub app config Units. A live install
should create real Variant spaces, upload or author these Units in ConfigHub,
promote between Variants, and record proof.

## Kubernetes Delivery

The `k8s/` files are starter workload manifests for ConfigHub OCI GitOps. In a
live install, govern these manifests in ConfigHub, bind them to `OCI`,
review the exact Unit-revision manifest, publish it through `cub release publish`,
then prove the controller and runtime state. Publication must remain blocked if
the platform cannot atomically bind the expected manifest.

## Stop Rule

Repo-local app code is not live proof. Keep this app in WATCH until ConfigHub, controller, runtime, URL, and receipt evidence exist for the app itself.
