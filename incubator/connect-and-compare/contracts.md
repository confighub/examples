# Contracts

## Read-Only Contracts

### `./setup.sh --explain-json`

- mutates: no
- proves:
  - the example is fixture-first
  - it does not mutate ConfigHub
  - it does not mutate live infrastructure
  - it writes local output only
- expected anchors:
  - `.example == "connect-and-compare"`
  - `.mutatesConfighub == false`
  - `.mutatesLiveInfrastructure == false`
  - `.writesLocalFilesOnly == true`

## Local Evidence Contracts

### `jq '.alignment' sample-output/03-compare.json`

- mutates: no
- proves:
  - compare output was generated
  - aligned and non-aligned states are inspectable

### `jq '.alignment[] | select(.status != "aligned")' sample-output/03-compare.json`

- mutates: no
- proves:
  - the example contains both `git-only` and `cluster-only` evidence

### `cat sample-output/01-doctor.txt`

- mutates: no
- proves:
  - standalone signal was generated from the doctor fixture

### `cat sample-output/04-history.txt`

- mutates: no
- proves:
  - history output was generated from the synthetic ChangeSet fixture
