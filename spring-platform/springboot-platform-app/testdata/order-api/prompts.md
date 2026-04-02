# Prompts

Copyable prompts for AI-assisted exploration.

## Orient Me

```text
Read this example, do not mutate anything yet, and explain:
- what stack it is for
- what it reads and writes
- which files are app-owned vs platform-owned
- the three mutation routes
```

## Safe Walkthrough

```text
Guide me through this example step by step.
Before each command:
- explain what it does
- say whether it mutates ConfigHub or live infrastructure
- tell me what success looks like
```

## Generator Deep Dive

```text
Explain the generator transformation:
- what inputs it reads
- what outputs it produces
- how field lineage determines mutation routes
- why some fields are blocked
```

## Verify The Model

```text
After reviewing this example, verify:
- the machine-readable contract contains all three behaviors
- the route rules align with the narrative
- the example remains read-only in preview mode
```

## Extend It

```text
Propose how this example could become a live ConfigHub example next.
Keep the answer grounded in existing files and say:
- what should stay structural
- what new scripts would be needed
- what live proof would still be missing
```
