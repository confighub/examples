# Contracts

## Read-only preview contracts

### `./verify.sh --explain`

- mutates: no
- output: plain text summary
- stable text anchors:
  - `promotion-demo-data-verify`
  - `What it checks:`
  - `Safe next steps:`
- proves:
  - what the wrapper does
  - what it reads
  - which stable example it depends on

### `./verify.sh --explain-json`

- mutates: no
- output: JSON object
- stable fields:
  - `example_name`
  - `proof_type`
  - `mutates_confighub`
  - `mutates_live_infra`
  - `depends_on_example`
  - `setup_command`
  - `cleanup_command`
  - `checks[]`
- proves:
  - the exact verification plan the wrapper will run
  - the names and thresholds of the high-signal checks

## Verification contracts

### `./verify.sh`

- mutates: no
- output: plain text pass/fail report
- stable summary anchors:
  - `=== ConfigHub Demo Verification ===`
  - `=== Verification Complete ===`
  - `Passed:`
  - `Failed:`
- proves:
  - the stable demo data still has the expected shape in ConfigHub

### `./verify.sh --json`

- mutates: no
- output: JSON object
- stable fields:
  - `example_name`
  - `status`
  - `passed`
  - `failed`
  - `checks[]`
- stable fields per check:
  - `name`
  - `comparison`
  - `expected`
  - `actual`
  - `status`
- proves:
  - which verification assertions passed or failed
  - which threshold or equality test each assertion used
  - whether the wrapper considers the dataset healthy overall

## Upstream dependency contract

### `../../promotion-demo-data/setup.sh`

- mutates: yes, ConfigHub only
- provides:
  - the stable demo dataset this wrapper verifies
- note:
  - this wrapper does not replace stable setup or cleanup; it only verifies the result
