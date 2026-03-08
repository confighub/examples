# Persona Quickstart: Jesper, Ilya, Claude

Use this when you want the shortest path to the right demo behavior.

## Shared Prerequisites

```bash
cub auth login
./scripts/verify.sh
```

Use a space with at least one ready target+worker, or pass `--target`.

## Existing user (iterating)

Goal: iterate on same app while deciding reuse vs fresh interactively.

```bash
CUB_UP_ON_EXISTS=reuse CUB_UP_STALE_ACTION=prompt \
./scripts/cub-up-human-flow.sh \
  app \
  ./incubator/cub-up/global-app \
  dev \
  <existing-target>
```

## First time user (shareable demo)

Goal: each run feels fresh and easy to share.

```bash
CUB_UP_ON_EXISTS=fresh CUB_UP_STALE_ACTION=fresh \
./scripts/cub-up-human-flow.sh \
  platform \
  ./incubator/cub-up/argocd-guestbook \
  dev \
  <existing-target>
```

## Claude (AI-led flow)

Goal: AI runs one command and narrates assertion transitions.

```bash
CUB_UP_ON_EXISTS=fresh CUB_UP_STALE_ACTION=fresh \
./scripts/cub-up-ai-flow.sh \
  app \
  ./incubator/cub-up/global-app \
  dev \
  <existing-target>
```

## Human + AI Pair

Goal: human follows GUI checkpoints while AI narrates and asserts.

```bash
./scripts/cub-up-pair-flow.sh \
  app \
  ./incubator/cub-up/global-app \
  dev \
  <existing-target>
```
