# Seven questions: is your config repo doing a database's job?

Paste the prompt below into Claude Code, Cursor, Codex or any agentic tool with
shell access to a clone of your configuration repository. It works on Argo CD
ApplicationSets, Flux Kustomizations and HelmReleases, Helm values and Kustomize
overlays, or anything shaped like them.

It reads only. Nothing is written, nothing is sent anywhere, and no account is
needed.

Each question names a habit most config repos have, and the database job that
habit is quietly standing in for. None of them is a defect until the owner says
so. Several answers will turn out to be deliberate.

To see what good answers look like before you point it at your own repo, run it
first on the [example fleet](../gitops/argo/intermediate-git-as-database/README.md),
which has one planted instance of each question and an answer sheet.

---

## The prompt

You have shell access to a clone of my configuration repository. Answer these seven
questions, in order.

Rules. Measure: write and run scripts, and never state a number you did not
compute. If something cannot be measured from the repository alone, say NOT
OBSERVED and say what you would need. Report findings as questions for me, not as
defects: some of this is deliberate. State your scope: which directories you
covered, which definitions you could not resolve, and any assumption you made
about where targets and their labels come from.

**1. Am I using filenames as primary keys?**
Find directories whose filenames encode a key, like `org-env-region`. Pick one
entity and count every file whose path or content refers to it: that is the cost
of renaming it. For labels such as environment, region, org, team or provider,
compare the values in use with the values that have their own values file. Are
missing values files silently ignored? If so, fill in each target's labels and
count how many value-file lookups currently resolve to nothing.

**2. Am I approving the query, but deploying the result?**
Count the templated sources (charts, bases) and the fully rendered manifests stored
anywhere in the repo. Is there anything that shows a reviewer the rendered output
of a change before it merges, such as hydration, or a CI job that renders and
comments?

**3. Am I using find-and-replace as my transaction?**
For each definition, count how many times the same version string is repeated.
Show me the worst one. Is there a script or tool whose job is to edit version
strings across files? Is there anywhere a promotion is recorded as one event, with
what, where, when and who?

**4. Am I using someone's memory as my status column?**
Which definitions hold two or more different versions right now? For each, is
there anything in the repo that records a start, a target, a completion condition
or an owner, so that a rollout in progress can be told apart from one that stopped?

**5. Am I using grep as my query engine?**
For each shared values path, starting with the most widely shared: if I change one
line, how many targets re-render? Resolve selectors against the target inventory if
there is one. If you cannot resolve a selector, say so; never assume it matches
everything. Count the values in that file and multiply: targets reached times
values is the blast radius if the whole file changes. Do the same for one target's
own values file and show both side by side. Is reach shown anywhere at review time?

**6. Am I using rate limits as my constraints?**
List every CI check, pull request size limit, bot rate limit and freeze window.
For each: does it limit how often changes happen, or what a single change can do?
Which files does each one actually cover, and does it cover the file with the
largest reach from question 5?

**7. Am I using defaults as my drift policy?**
For each definition: is self-heal on, is prune on, and is any tolerance for drift
declared (ignore rules, sync options, comments) or left at the default? Give me the
counts. Is there anywhere in the repo where observed facts about the running system
are written back, and what stops someone editing them by hand?

Output one short section per question: the numbers, the script that produced them,
and what you could not determine.

---

## Reading the answers

Read question 6 next to question 5. If the widest-reaching file is covered by none
of your controls, your safety rules are guarding the wrong files.

Read question 1 as two findings. The rename count is what each reorganisation
costs. The lookups that resolve to nothing are config that is not doing what
someone thinks it does, and nothing will ever report it.

Questions 3 and 4 usually point at the same definitions. A version repeated once
per wave is what makes it possible for a rollout to stop halfway without anyone
noticing.

## What this cannot see

Question 7 tells you what you have allowed to differ, not what has differed.
Measuring actual drift needs the running systems. Question 4 can show you that
nothing records rollout state; which of your rollouts are really stuck needs
history and live status.

## A script version

[scout](./scout/README.md) runs the same seven questions as fixed checks in one
read-only Python script, so the same repo gives the same answer every run. It
covers Argo CD ApplicationSets with Helm best, and says NOT OBSERVED where it
cannot resolve something. The prompt adapts to any repo shape and explains itself
in your terms. Running both is a useful cross-check.
