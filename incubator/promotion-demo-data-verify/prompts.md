## Orient Me First

Read this verification wrapper, do not run anything yet, and explain:

- what stable example it depends on
- what it checks
- what it reads
- what it does not verify
- what the JSON output is for

Start with:

- `./verify.sh --explain`
- `./verify.sh --explain-json | jq`

## Run The Stable Setup And Verify It

Guide me through the full sequence:

1. create the stable `promotion-demo-data` dataset
2. run this wrapper
3. show the important checks
4. clean up afterwards

Before each command, say whether it mutates anything and what success looks like.

## Show Me The JSON Contract

Run `./verify.sh --json | jq` and explain:

- which checks are equality vs minimum-count checks
- which checks are the most important for AI/CI gating
- how to interpret a failed result quickly

## Compare CLI Verification To The GUI

After running the checks, show me:

- which spaces or units I should inspect in the ConfigHub UI
- what the CLI proved already
- what the UI still requires me to inspect manually
