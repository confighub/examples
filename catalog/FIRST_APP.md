# What an app looks like here

Start with the [realistic app](../global-app-layer/realistic-app/README.md).
It models a small chat application with three coordinated components: a
[frontend Deployment](../global-app-layer/baseconfig/frontend.yaml), a
[backend Deployment](../global-app-layer/baseconfig/backend.yaml), and a
[Postgres StatefulSet](../global-app-layer/baseconfig/postgres.yaml). The
frontend and backend refer to published Cubbychat image names; the example
uses a sample database password and disables its optional Ollama feature.
This is an application-shaped configuration example, not a claim that this
checkout has a healthy running chat service.

The application code stays in [Cubbychat](https://github.com/confighub/cubbychat).
Here, a Unit holds a component's configuration, and a Space groups related
configuration. ConfigHub lets you version, compare and govern those concrete
objects; it does not replace the application code or the delivery controller.

The path through the example is:

```text
source YAML for frontend + backend + Postgres
  -> specialized region, role, recipe and deployment manifests
  -> ConfigHub Spaces, Units and clone links (after setup)
  -> bound target and applied deployment (only in the separate live path)
```

The base files are already Kubernetes YAML. “Flattened” means concrete
Kubernetes objects after templates, overlays and layer choices have been
resolved. These base inputs still use a `confighubplaceholder` namespace and
development values, so they need deployment-specific resolution before apply.
The recipe describes how the three components become deployment variants together. In the
preview below, a planned `<generated-prefix>-deploy-cluster-a` Space has
`backend-cluster-a`, `frontend-cluster-a` and `postgres-cluster-a` Units.
The plan names what setup would create; it does not create those Units.

From this repository root, run the same first step whether you are a person
or an AI assistant. The plan needs Bash and jq:

```bash
cd global-app-layer/realistic-app
./setup.sh --explain-json | jq '{example, mode, mutates, spaces, deployment_units: [.units[] | select(.stage == "deployment") | .unit]}'
```

This is read-only. Expect `mutates: false`, five planned Spaces and the three
deployment Units. The [AI guide](../global-app-layer/realistic-app/AI_START_HERE.md)
and [contract](../global-app-layer/realistic-app/contracts.md) describe the
same app in more detail. The [Workshop model guide](https://confighub.github.io/helm-expt/site/d/docs/user/model-and-vocabulary.html)
explains how catalog, variant, stack and fleet terms relate to this shape.

Now make one visible change to **this app's frontend** without an account or
cluster. Install [Workshop's `cub config` plugin](https://github.com/confighub/cub-workshop#readme)
first; `cub` alone does not supply this command. The helper uses Bash, ripgrep,
sed and shasum. Return to this repository root and run:

```bash
bash catalog/first-app-local-change.sh
```

The script copies `global-app-layer/baseconfig/frontend.yaml` into a fresh
local directory, changes its one `replicas: 1` line to `replicas: 2` in the
copy, and runs `cub config diff before.yaml after.yaml --json --out result.json`.
It never overwrites the source file. The checked `result.json` reports three
objects, one changed Deployment and one replaced field, `/spec/replicas`, from
1 to 2. It also records before/after SHA-256 values. The exact sanitized
result is in the [qualification receipt](./qualification.json). This local
comparison does not validate Kubernetes admission, perform a layer merge,
write ConfigHub data or deploy anything.

To see how templated source can become flattened manifests and how a platform
can allow or refuse an edit, continue with the separate
[payments-api Helm example](https://github.com/confighub/cub-gen/tree/fa70ffad379765d8052f6873ccdb51b8edf4b3e7/examples/helm-paas).
It is a small chart used to teach field ownership, not the Cubbychat app.
Its `values.yaml` sets `image.tag: v1.0.0`; the Deployment template uses that
value as a container image. From a checkout of `confighub/cub-gen` at the
[reviewed revision](https://github.com/confighub/cub-gen/tree/fa70ffad379765d8052f6873ccdb51b8edf4b3e7),
the local source-to-manifest step is:

```bash
helm template payments-api examples/helm-paas | rg 'kind: Deployment|replicas:|image: ghcr.io/example/payments-api'
```

The reviewed chart rendered a Deployment with two replicas and image
`ghcr.io/example/payments-api:v1.0.0`. Its `ghcr.io/example` reference is a
teaching placeholder, not a verified runnable payments service. The next
step makes one allowed source edit and one refused template edit in a
temporary clone, retaining JSON reports:

```bash
./examples/helm-paas/demo-governed-change.sh
```

The checked local result was `allow-report.json` with `status: pass` for
`values.yaml`, and `block-report.json` with `status: fail` for a direct edit to
`templates/deployment.yaml`. The helper builds `cub-gen` and writes only to
its local output directory. See the [pinned AI guide](https://github.com/confighub/cub-gen/blob/fa70ffad379765d8052f6873ccdb51b8edf4b3e7/examples/helm-paas/AI_START_HERE.md)
for the owner boundary. The qualification receipt also records this command
and sanitized result. An allowed local edit is a useful
review artifact; it is not a connected approval or delivery receipt.

If you later have a compatible authenticated ConfigHub context, the realistic
app's [README](../global-app-layer/realistic-app/README.md) documents
`./setup.sh` followed by `./verify.sh --json` for the ConfigHub-only path.
That setup creates five Spaces, layered Units and clone links. Binding a target
and running `./apply-live.sh` is a further step with cluster effects, and the
README explains its target preflight and verification. Neither connected nor
live step was run for this catalog. The [Workshop GitOps adopter guide](https://confighub.github.io/helm-expt/site/d/docs/user/gitops-adopter-guide.html)
continues the source-to-controller discussion for Argo CD and Flux; a
ConfigHub Unit or OCI artifact alone does not show what a controller applied.

For an AI assistant, the user-facing route is the same: show the source files,
run the read-only plan, explain the resulting planned Units, and show the
frontend replica diff. The Helm ownership lesson is optional. Stop at a missing
context, target, or authority. Do not infer a deployed workload from the
plan, a simulated decision, or the example image name.
