## Orient Me First

Read this example, do not mutate anything yet, and explain:

- what app it represents
- how deployments map to ConfigHub spaces
- what target modes exist
- what the three mutation outcomes are
- what the wrapper delegates to in `../springboot-platform-app/`

Start with:

- `cat deployment-map.json | jq`
- `./setup.sh --explain`
- `./setup.sh --explain-json | jq`

## Safe Walkthrough

Guide me through `springboot-platform-app-centric` stage by stage.

Before each command:

- explain what it does
- say whether it mutates ConfigHub or live infrastructure
- tell me what success looks like
- tell me what the GUI shows today and what the current GUI gap is
- pause until I say continue

## Compare Target Modes

Explain the difference between:

- `./setup.sh`
- `./setup.sh --confighub-only`
- `./setup.sh --with-targets`

Tell me what each mode reads, what it writes, and what kind of proof it gives me.

## Show The Three Mutation Outcomes

Walk me through `./demo.sh` and then point me to the right flow file for each outcome:

- `flows/apply-here.md`
- `flows/lift-upstream.md`
- `flows/block-escalate.md`

Keep the explanation app-centric, not implementation-centric, unless I ask for the lower-level details.
