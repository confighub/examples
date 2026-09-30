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
- stable text anchors: `SEVEN QUESTIONS: IS YOUR CONFIG REPO DOING A DATABASE'S JOB?`, section headings `1  Am I using` to `7  Am I using`, `Also:`
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
