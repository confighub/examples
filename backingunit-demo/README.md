# A change workflow and its backing Unit, as a live demo

A change workflow says how a change is promoted: the stages it moves through, and the gates each
stage requires. Created with `--with-backing-units`, the workflow also gets a backing Unit — an
ordinary Unit whose data is the workflow as configuration. Editing the workflow edits that Unit, so
every change to the workflow is a Revision, with who made it and when.

```
./changeworkflow-demo.sh            the demo
./changeworkflow-demo.sh cleanup    delete the space it made
```

Each command is typed out on screen and then waits. Press **space** to run it; `s` skips it, `f`
stops the typing effect, `!` opens a subshell, and `q` quits. The runner is
[`scripts/demo-lib.sh`](../scripts/demo-lib.sh), described in the
[tutorial demo's README](../tutorial/README.md#using-the-runner-for-your-own-demo).

## Before you start

- `cub`, authenticated (`cub auth login`)

No cluster and no worker: everything happens in one Space, `workflow-demo` unless `DEMO_SPACE` says
otherwise.

## What it shows

1. Create a three-stage workflow, dev → staging → prod, each stage gated on the one before it having
   released the change and reporting healthy.
2. The workflow, and its backing Unit's data, Unit metadata and Revisions.
3. `cub changeworkflow edit` opens `$EDITOR` (`vi` if unset). Add `Validated` to the prod stage's
   `Prerequisites`, save, and the workflow has the new gate — and the backing Unit has a new
   Revision.

## Rehearsing

```
DEMO_DRYRUN=1 DEMO_AUTO=1 ./changeworkflow-demo.sh    # read the whole thing back, run nothing
DEMO_AUTO=1 ./changeworkflow-demo.sh                  # run end to end, nobody at the keyboard
```

With `DEMO_AUTO=1`, or with no terminal on standard input, a script stands in for the editor and
makes the same change you would make by hand.

| Variable | |
| --- | --- |
| `DEMO_SPACE` | the Space to work in (default `workflow-demo`) |
| `DEMO_SPEED` | characters per second while typing (default 45; `0` is instant) |
| `DEMO_AUTO` | never wait for a key, and edit the workflow without an editor |
| `DEMO_DRYRUN` | show every command, run none of them |
| `NO_COLOR` | plain text |
