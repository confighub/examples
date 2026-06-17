# Contracts: ctrlplane-on-confighub

Stable command outputs automation and AI assistants can rely on. The default
path is read-only; the only mutating path is documented and gated.

## `./setup.sh --explain`

- mutates: no
- output shape: plain text
- stable text anchors: `Ctrlplane -> ConfigHub mapping plan`, `Spaces to create`,
  `Units to create`, `Targets to bind`, `Gates (from Policies)`, `Mapping seams`
- proves: a Ctrlplane System bundle can be rendered as a ConfigHub governed-app
  plan without touching ConfigHub or any cluster

## `./setup.sh --explain-json`

- mutates: no
- output shape: JSON object
- stable fields: `example_name`, `mutates`, `app`, `delivery_strategy`,
  `spaces`, `units`, `targets`, `plan.release_targets_preview`, `mapping_notes`,
  `warnings`
- proves: the mapping is machine-readable and the `mutates` flag is `false`

## `./setup.sh --cub-commands`

- mutates: no (prints only; nothing is executed)
- output shape: a shell script body (valid `bash -n`)
- stable text anchors: `cub space create`, `cub unit create --space`
- proves: the plan lowers to a concrete, reviewable `cub` command set, including
  the explicit caveat that each Unit needs a supplied manifest

## `./verify.sh`

- mutates: no
- output shape: plain text
- stable success text: `All checks passed.`
- proves: the mapper runs, emits valid JSON, and the stable fields hold

## `./setup.sh --apply`

- mutates: ConfigHub (intended) — but this POC refuses to auto-create and instead
  prints the plan, checks auth, and tells you to review `--cub-commands` and run
  them by hand
- proves: nothing live yet — the live create/apply path is the next proof step
  for this example and is **not yet verified end-to-end** against a ConfigHub
  space (see README "Status")
