# Copyable Prompts

## Orient Me First

Read `gpu-eks-h100-training` and do not mutate anything yet.

Explain:
- what stack it is for
- what it reads
- what it writes
- what is structural proof vs real NVIDIA deployment proof
- what success should look like

Then run only the read-only preview commands.

## Safe Walkthrough

Guide me through `gpu-eks-h100-training` step by step.

Before each command:
- explain what it does
- say whether it mutates ConfigHub or live infrastructure
- say what success looks like
- say what to inspect next

## Verify Everything

After running `gpu-eks-h100-training`, verify:
- created spaces
- created units
- layered variant ancestry
- recipe manifest
- target binding if used
- live apply state if used
