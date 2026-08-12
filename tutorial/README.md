# The get-started tutorial, as a live demo

[The ConfigHub tutorial](https://docs.confighub.com/get-started/tutorial/) read aloud: the same
commands, in the same order, driven one keypress at a time so you can talk between them.

```
./tutorial.sh                 every section, in order
./tutorial.sh change flow     just those sections
./tutorial.sh --list          what the sections are
```

Each command is typed out on screen and then waits. Press **space** to run it. The output is the
real output — this runs against your ConfigHub organization and your machine, and it is the
tutorial, not a recording of it.

| Key | |
| --- | --- |
| space, enter | run the command on screen |
| `s` | skip it, leave it unrun |
| `f` | stop typing character by character for the rest of the demo |
| `!` | drop into a subshell; exit that shell to come back |
| `q` | quit |

## Before you start

- `cub`, authenticated (`cub auth login`)
- Docker running, plus `kind` and `kubectl`

The `cluster` and `prod` sections each build a real kind cluster with Argo CD in it, so they take a
few minutes. The rest are quick.

## The sections

| Section | What it shows |
| --- | --- |
| `cluster` | `cub cluster up`: a local cluster wired to ConfigHub, with a target addressing it |
| `install` | a component's base, seeded from a config bundle with `cub variant upload` |
| `release` | the dev deployment, and a release the cluster pulls |
| `change` | change the base, promote, release — the daily loop |
| `prod` | a second cluster, and a production deployment stamped from the same base |
| `flow` | a change flowing base → dev → prod, with a protected value and the conflict it produces |
| `cleanup` | tear both clusters down and delete the spaces |

`flow` is the interesting five minutes if the clusters are already up: it is where protection,
merge conflicts, and `--dry-run` all show up in one story.

## Rehearsing

```
DEMO_DRYRUN=1 DEMO_AUTO=1 ./tutorial.sh     # read the whole thing back, run nothing
DEMO_SPEED=0 ./tutorial.sh flow             # no typing effect; still one keypress per command
```

| Variable | |
| --- | --- |
| `DEMO_SPEED` | characters per second while typing (default 45; `0` is instant) |
| `DEMO_AUTO` | never wait for a key — for rehearsing end to end |
| `DEMO_DRYRUN` | show every command, run none of them |
| `NO_COLOR` | plain text |

## Using the runner for your own demo

`demo-lib.sh` is the whole mechanism and has no dependencies beyond bash — no `pv`, no `tmux`, and
nothing newer than the bash that ships with macOS.

```bash
#!/usr/bin/env bash
source "$(dirname "${BASH_SOURCE[0]}")/demo-lib.sh"

heading "Where the config lives"
desc "Everything starts from the base."
run  "cub unit list --space cubbychat-base"
```

`desc` narrates, `run` types and executes, `heading` marks a section, `pause` waits for a beat, and
`silent` runs setup nobody needs to watch. Commands run in the calling shell, not a subshell, so
`source cluster.env` and `cd` affect the commands after them.
