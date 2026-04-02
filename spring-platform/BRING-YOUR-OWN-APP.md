# Bring Your Own Spring Boot App

This is the page for the first practical question a team will ask:

> "Nice demo. Can I deploy my own Spring Boot app with this?"

The short answer is:

- yes, as an adaptation of the example
- no, not yet as a turnkey import workflow

Today, `spring-platform` is best used as:

- a reference implementation
- a proof path
- a shared vocabulary for app-owned versus platform-owned config

## What You Can Reuse As-Is

From [`springboot-platform-app`](./springboot-platform-app/):

- the Spring Boot shape: app config, profiles, Docker image, health checks
- the ConfigHub setup path: `./confighub-setup.sh`, `./confighub-verify.sh`, `./confighub-cleanup.sh`
- the real Kubernetes proof path: `./bin/create-cluster`, `./bin/build-image`, `./bin/install-worker`, `./confighub-setup.sh --with-targets`
- the mutation story: apply-here, lift-upstream, block/escalate
- the visibility tools: `./generator/render.sh --trace`, `./generator/render.sh --explain-field`, `./confighub-compare.sh`, `./confighub-refresh-preview.sh`

## What You Must Replace

The sample app is `inventory-api`. If you want to use your own app, you need to replace or adapt:

- `springboot-platform-app/upstream/app/`
- `springboot-platform-app/confighub/inventory-api-*.yaml`
- `springboot-platform-app/operational/*.yaml`
- `springboot-platform-app/operational/field-routes.yaml`
- `springboot-platform-app/bin/build-image`
- any scripts that assume:
  - app name = `inventory-api`
  - namespace = `inventory-api`
  - image = `inventory-api:local`

You will also need to revisit:

- ports
- actuator/health paths
- env vars
- Spring profiles
- image name and registry

## What "Deploy Your Own App" Means Today

Today, this is the supported interpretation:

1. Put your app where the sample app lives.
2. Update the rendered manifests and ConfigHub unit YAMLs for your app.
3. Update field ownership rules.
4. Run the same proof path.

That proof path is:

```bash
cd springboot-platform-app
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

## Recommended Team Path

If you are evaluating this with a real team, do it in this order:

1. Run the sample `inventory-api` unchanged.
2. Confirm the team understands the three mutation routes.
3. Replace the app with one internal service.
4. Keep the platform policy small and explicit.
5. Prove the service on noop targets first.
6. Only then run the Kind path.

This keeps the evaluation focused on the model, not on porting five things at once.

## What Team-Ready Would Mean

If you want this to feel turnkey for internal teams, the next layer of work is:

- a documented "replace the sample app" path
- fewer hard-coded `inventory-api` assumptions
- a generator or import path that updates manifests and ConfigHub YAMLs for a new app
- a clear ownership contract for field routes

That is different from the current state, which is:

- a strong flagship example
- a real proof path
- a good starting point for one team to adapt

## Which Example Should A Team Start With?

- start with [`springboot-platform-app`](./springboot-platform-app/) if the question is "how does config get generated?"
- use [`springboot-platform-app-centric`](./springboot-platform-app-centric/) if the question is "how do we operate one app across environments?"
- use [`springboot-platform-platform-centric`](./springboot-platform-platform-centric/) if the question is "how does a platform team govern many apps?"

If the goal is "deploy one of our own apps", start with the core example first.
