# Promotion record for apptique prod

This file is the receipt a promotion pull request writes alongside its one
line change to `kustomization.yaml`. It is not read by Argo CD, and Kustomize
ignores it. `../../../verify.sh` reads it, because nothing else in this repo
records what was actually promoted.

- promoted_tag: v1.3.0
- promoted_from: dev
- source_pr: intermediate-ci-to-gitops#41 (fictional, this example is never run)
- note: dev is already on v1.4.0. Production is one promotion behind, the
  way a real fleet usually is mid-rollout, not because anything is broken.

## Why this file exists

A pull request diff on `kustomization.yaml` shows one line: the tag going
from one string to another. It does not show whether that string is the tag
someone actually meant to promote. This file is the plain-language claim the
PR was supposed to make true. When `newTag` above stops matching
`promoted_tag` here, either this file is stale, or the wrong tag was merged.
See "Break it on purpose" in `../../../README.md`.
