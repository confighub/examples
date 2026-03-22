# AI Start Here

Use this page when you want to drive `connect-and-compare` safely with an AI assistant.

## What This Example Is For

This is a fixture-first evidence example.

It is useful when the human wants to show:

- immediate standalone signal
- compare output
- history output
- no live cluster requirement

## Read-Only First

Start here:

```bash
cd incubator/connect-and-compare
./setup.sh --explain
./setup.sh --explain-json | jq
```

These commands do not mutate ConfigHub and do not mutate live infrastructure.

## Recommended Path

```bash
./setup.sh
./verify.sh
```

## Important Boundaries

- `./setup.sh --explain` is read-only
- `./setup.sh` writes local sample output only
- `./verify.sh` writes to a temporary directory only
- `./cleanup.sh` removes local sample output only

This example does not require a live cluster.
It does require `cub-scout`.

## What To Verify

```bash
jq '.alignment' sample-output/03-compare.json
jq '.alignment[] | select(.status != "aligned")' sample-output/03-compare.json
cat sample-output/01-doctor.txt
cat sample-output/04-history.txt
```

Use the evidence like this:

- doctor output proves immediate standalone signal
- compare output proves intent versus observed alignment facts
- history output proves the review trail story

## Related Files

- [README.md](./README.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)
