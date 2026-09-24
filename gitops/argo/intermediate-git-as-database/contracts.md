# Contracts

Stable command outputs for `gitops/argo/intermediate-git-as-database`.

### `./setup.sh --explain`

- mutates: no
- output shape: plain text
- stable text anchors: `This is a read-only plan`, `Nothing will be mutated.`
- proves: the plan before anything runs

### `./setup.sh --explain-json`

- mutates: no
- output shape: JSON object
- stable fields: `example_name`, `mutates`, `mutates_confighub`, `mutates_live_infra`, `questions`, `evaluation_modes`
- proves: the example is read-only end to end

### `./setup.sh`

- mutates: no
- output shape: plain text report from `prompts/scout/scout.py`
- stable text anchors: `SEVEN QUESTIONS TO ASK IF YOU ARE USING GIT AS A DATABASE`, `Q1` to `Q7`, `Q7b`, `Q7c`
- proves: each planted problem in `repo/` is visible from the files alone

### `./verify.sh`

- mutates: no
- output shape: plain text, one `ok` or `FAIL` line per check
- stable success text: `All checks passed.`
- proves: scout's output matches every answer in `EXPECTED.md`

### `./cleanup.sh`

- mutates: no
- stable text: `Nothing to clean up.`
- proves: nothing was created
