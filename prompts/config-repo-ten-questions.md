# Ten questions to ask an AI about your config repository

Paste the prompt below into Claude Code, Cursor, Codex or any agentic tool with
shell access to a clone of your configuration repository. It works on Helm values,
Argo CD ApplicationSets, Flux Kustomizations, Kustomize overlays, or anything
shaped like them.

It takes a few minutes and it reads only. Nothing is written, nothing is sent
anywhere, and no account is needed.

The questions come in four groups, because four things tend to go wrong with a
configuration repository once it is large enough to matter.

- **Drift** — a fix nobody wrote back
- **Sprawl** — variety nobody declared
- **Blast** — reach nobody could see
- **Limbo** — a wave nobody finished

---

## The prompt

You have shell access to a clone of my configuration repository. Answer these ten
questions.

Rules. Measure — write and run scripts, and never state a number you did not
compute. If something cannot be measured from the repository alone, say NOT
OBSERVED and say what you would need. Report findings as questions for me, not as
defects: some of this is deliberate. State your scope, meaning which directories
you covered and which definitions you could not resolve.

**Drift**

1. For each ApplicationSet, HelmRelease or Kustomization: is self-heal on, is prune
   on, and is any tolerance for drift declared explicitly or just left at the
   default? Give me the counts.
2. Is there anywhere in this repo where observed data lands — facts about the
   running system rather than intent? What stops someone editing it by hand?

**Sprawl**

3. Find directories where filenames encode a key, like `cluster-env-region`. Count
   how many files would have to change to rename one entity.
4. For labels like environment, region, org or team: how many separate places
   declare the valid values, do they agree with each other, and do they agree with
   the values actually in use? And are missing values files silently ignored — if
   so, how many layer lookups across the repo currently resolve to nothing?

**Blast**

5. How many templated sources are there — charts, bases — and how many fully
   rendered manifests are stored anywhere in this repo?
6. For each shared values path, starting with the most widely shared: if I change
   one line in it, how many Applications or targets re-render? Resolve selectors
   against the target inventory if there is one. If you cannot resolve a selector,
   say so — do not assume it matches everything. Then do the same for one cluster's
   own values file and show me the numbers side by side.
7. Do the charts pin their dependencies? Check for `Chart.lock` and for version
   ranges. If a dependency is a range and the lock file is not committed, tell me
   what happens to a rollout when a new version is published halfway through.
8. List every CI check, bot rate limit and freeze window. For each: does it limit
   what a single change can do, or only how often changes can happen?

**Limbo**

9. For each definition, count how many times the same version string is repeated.
   Show me the worst one.
10. Which definitions hold two or more different versions right now? For each, is
    there anything in the repo that says whether that is a rollout in progress or
    one that stopped?

---

## What to do with the answers

Read question 8 first. Most repositories turn out to be governed by quotas — a cap
on files per pull request, a rate limit, a freeze window — rather than by anything
that knows what a change reaches. A quota limits how often you change things. It
does not limit what one change does. If question 6 returns a large number and
question 8 returns only quotas, those two answers belong together.

Then read question 4 against question 3. Declared variety and encoded variety are
the same information stored two ways, and the second kind grows when the first kind
has stopped being able to take it.

The answers that matter most are usually the ones nobody chose: a default nobody
set, a values file that has never resolved for anything, a version string repeated
forty times because promotion is find-and-replace. None of those was a decision.

## A note on what this cannot see

Questions 3 to 10 can be answered from a clone. Question 1 tells you what you have
allowed to differ, not what has actually differed, and question 2 tells you whether
there is anywhere for observed facts to live. Real drift measurement needs access
to the running systems as well.

Question 7 is here because unpinned dependencies are worth knowing about, and not
because any particular tool fixes them.
