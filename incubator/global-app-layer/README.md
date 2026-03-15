# `global-app-layer`

This incubator package is the current home for ConfigHub recipe and layer experiments, starting with `global-app` and extending into domain-shaped examples.

It brings three things together in one place:

- analysis of why the NVIDIA AICR style matters for ConfigHub
- a working recipes-and-layers spec for ConfigHub
- four runnable worked examples that teach the model in stages

## Start Here

Read in this order:

1. [01-nvidia-aicr-fit.md](./01-nvidia-aicr-fit.md)
2. [02-recipes-and-layers-spec.md](./02-recipes-and-layers-spec.md)
3. [04-review-and-next-steps.md](./04-review-and-next-steps.md)

Then try the examples in this order:

1. [single-component](./single-component/README.md)
2. [frontend-postgres](./frontend-postgres/README.md)
3. [realistic-app](./realistic-app/README.md)
4. [gpu-eks-h100-training](./gpu-eks-h100-training/README.md)

## What This Package Proves

- A recipe can be modeled as an ordered clone chain.
- The bundle is a deployment artifact, not the source of truth for provenance.
- An explicit recipe manifest is useful even when execution stays implicit in clones and links.
- The same layer names can keep a shared meaning across multiple components while the mutations remain component-specific.

## Testing

Repo-wide verification:

```bash
cd <examples-repo-root>
./scripts/verify.sh
```

Example-specific verification:

```bash
cd incubator/global-app-layer/single-component
./setup.sh
./verify.sh
./cleanup.sh

cd ../frontend-postgres
./setup.sh
./verify.sh
./cleanup.sh

cd ../realistic-app
./setup.sh
./verify.sh
./cleanup.sh

cd ../gpu-eks-h100-training
./setup.sh
./verify.sh
./cleanup.sh
```

These example scripts require a working `cub` CLI, `jq`, and an authenticated ConfigHub context.
