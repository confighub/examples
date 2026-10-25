# Prompts

Copyable prompts for AI-assisted exploration.

## Orient Me

```text
Read this example, do not mutate anything yet, and explain:
- what app it represents
- how deployments map to ConfigHub spaces
- why it creates no targets
- what the three mutation outcomes are
```

## Safe Walkthrough

```text
Guide me through this example stage by stage.
Before each command:
- explain what it does
- say whether it mutates ConfigHub or live infrastructure
- tell me what success looks like
- pause until I say continue
```

## Show The Three Mutation Outcomes

```text
Walk me through ./demo.sh and then point me to the right flow file for each outcome:
- flows/apply-here.md
- flows/lift-upstream.md
- flows/block-escalate.md

Keep the explanation app-centric.
```

## Field Ownership

```text
Explain why some fields are mutable and some are blocked.
Show me how to use the generator's explain-field command.
```
