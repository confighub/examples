# Bring Your Own Spring Boot App

This is the page for the first practical question a team will ask:

> "Nice demo. Can I deploy my own Spring Boot app with this?"

The short answer is:

- yes, using the scaffold command
- the scaffold handles mechanical renaming; you still need to replace the actual app code and review field ownership

## Quick Start: Scaffold Your App

```bash
cd spring-platform/springboot-platform-app

# See what would be created
./bin/scaffold-app my-service --dry-run

# Generate a scaffolded copy
./bin/scaffold-app my-service --output ../my-service

# With custom Java package
./bin/scaffold-app my-service --package com.mycompany.myservice --output ../my-service
```

The scaffold command:

- Copies the example directory
- Renames `inventory-api` → `my-service` across all files
- Updates Java package from `com.example.inventory` → your package
- Renames Java classes: `Inventory*` → `MyService*`
- Updates ConfigHub YAML slugs, labels, and filenames
- Updates field-routes with your app prefix
- Creates `ADAPTATION-CHECKLIST.md` listing what you still need to review

The scaffold is regression-checked: `./bin/verify-scaffold` proves the rename works for a second app shape.

## What the Scaffold Does NOT Do

The scaffold handles mechanical renaming. You still need to:

- **Replace the sample app code** in `upstream/app/` with your actual service
- **Review ports and health paths** if yours differ from the default
- **Review field ownership** in `operational/field-routes.yaml`
- **Update environment variables** for your service's needs
- **Configure image registry** if not using local Kind loading

The generated `ADAPTATION-CHECKLIST.md` lists these items with checkboxes.

## What You Can Reuse As-Is

From [`springboot-platform-app`](./springboot-platform-app/):

- the Spring Boot shape: app config, profiles, Docker image, health checks
- the ConfigHub setup path: `./confighub-setup.sh`, `./confighub-verify.sh`, `./confighub-cleanup.sh`
- the real Kubernetes proof path: `./bin/create-cluster`, `./bin/build-image`, `./bin/install-worker`, `./confighub-setup.sh --with-targets`
- the mutation story: apply-here, lift-upstream, block/escalate
- the visibility tools: `./generator/render.sh --trace`, `./generator/render.sh --explain-field`, `./confighub-compare.sh`, `./confighub-refresh-preview.sh`

## Verifying Your Scaffolded App

After scaffolding and adapting:

```bash
cd my-service  # your scaffolded directory
./setup.sh --explain
./verify.sh
./confighub-setup.sh --with-noop-targets
./confighub-verify.sh
```

If you want real Kubernetes delivery:

```bash
./bin/create-cluster
./bin/build-image
CUB_SPACE=springboot-infra ./bin/install-worker
export KUBECONFIG=var/springboot-platform.kubeconfig
export WORKER_SPACE=springboot-infra
./confighub-setup.sh --with-targets
./verify-e2e.sh
```

## From This Example to the Real Tool

If you want to evaluate the real generator path instead of only adapting the fixed example, go to [`cub-gen/examples/springboot-paas`](https://github.com/confighub/cub-gen/tree/main/examples/springboot-paas).

```bash
cd /Users/alexis/Public/github-repos/cub-gen
go build -o ./cub-gen ./cmd/cub-gen
./examples/springboot-paas/demo-local.sh

# connected path
cub auth login
./examples/springboot-paas/demo-connected.sh
```

Use `spring-platform` to learn the model and adapt the example. Use `springboot-paas` to see the real generator run in the product repo.

## Recommended Team Path

If you are evaluating this with a real team, do it in this order:

1. Run the sample `inventory-api` unchanged.
2. Confirm the team understands the three mutation routes.
3. Replace the app with one internal service.
4. Keep the platform policy small and explicit.
5. Prove the service on noop targets first.
6. Only then run the Kind path.

This keeps the evaluation focused on the model, not on porting five things at once.

## What Is Now Supported vs. Still Manual

**Supported by the scaffold:**

- ✓ Renaming `inventory-api` to your app name
- ✓ Updating Java package and class names
- ✓ Renaming ConfigHub YAML files and updating slugs/labels
- ✓ Updating field-routes with your app prefix
- ✓ Generating a checklist of remaining manual steps

**Still requires your judgment:**

- Replacing the stub app code with your actual service
- Deciding which fields are app-owned vs platform-owned
- Configuring ports, health paths, and environment variables
- Setting up image registry and deployment targets

## Which Example Should A Team Start With?

- start with [`springboot-platform-app`](./springboot-platform-app/) if the question is "how does config get generated?"
- use [`springboot-platform-app-centric`](./springboot-platform-app-centric/) if the question is "how do we operate one app across environments?"
- use [`springboot-platform-platform-centric`](./springboot-platform-platform-centric/) if the question is "how does a platform team govern many apps?"

If the goal is "deploy one of our own apps", start with the core example first.
