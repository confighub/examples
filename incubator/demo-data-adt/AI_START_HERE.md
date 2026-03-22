# AI Start Here

Use this page when you want to drive `demo-data-adt` safely with an AI assistant.

## What This Example Is For

This is a scan-first App-Deployment-Target example.

It is useful when the human wants immediate risk findings on labeled workload fixtures without needing a cluster.

## Read-Only First

Start here:

```bash
cd incubator/demo-data-adt
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
- `./setup.sh` writes local output only
- `./verify.sh` is read-only with respect to ConfigHub and live infrastructure
- `./cleanup.sh` removes local sample output only

This example does not require a live cluster.
It does require `cub-scout`.

## What To Verify

```bash
jq '.static.findings' sample-output/dev-eshop.scan.json
jq '.static.findings' sample-output/prod-eshop.scan.json
jq '.static.findings' sample-output/prod-website.scan.json
```

Use the evidence like this:

- `dev-eshop` proves immediate warnings exist
- `prod-eshop` proves the clean case
- `prod-website` proves the same issue can show up in a different app and owner path

## Related Files

- [README.md](./README.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)
