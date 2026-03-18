# ConfigHub Hello World for This Package

This is the smallest useful starting point before the layered recipe examples.

It does **not** build a layered recipe.
It just shows the core ConfigHub move:

1. create a space
2. create one unit from one YAML file
3. inspect it
4. optionally bind a target and apply it

Once that feels clear, the `global-app-layer` examples simply repeat the same pattern across more than one specialization stage.

## What You Will Learn

- what a `space` is
- what a `unit` is
- what it means to store Kubernetes YAML in ConfigHub
- how a deployment target fits in

## Prerequisites

- `cub` installed
- `cub auth login`
- `jq`
- optional: a live target if you want to apply to a cluster

## One Unit, One Space

From the repo root:

```bash
cd <your-examples-checkout>

PREFIX="$(cub space new-prefix)"
SPACE="${PREFIX}-hello-world"

# Create one space
cub space create "${SPACE}" --label ExampleName=global-app-layer-hello

# Create one unit from one manifest
cub unit create \
  --space "${SPACE}" \
  hello-backend \
  global-app/baseconfig/backend.yaml \
  --label ExampleName=global-app-layer-hello

# Inspect the stored config
cub unit get --space "${SPACE}" --data-only hello-backend
```

What happened:

- the YAML file became a versioned ConfigHub unit
- the unit now lives in one ConfigHub space
- nothing layered happened yet

## Optional: Apply It to a Real Target

If you already have a worker and target:

```bash
cub unit set-target --space "${SPACE}" hello-backend <space/target>
cub unit approve --space "${SPACE}" hello-backend
cub unit apply --space "${SPACE}" hello-backend
```

What the placeholder means:

- `<space/target>` means the full target reference, using the target's space and slug
- you only need this for optional live delivery
- the earlier steps already proved that the manifest can be loaded and inspected in the ConfigHub database without any target

To find a real target reference:

```bash
cub target list --space "*" --json
```

That is the core delivery loop:

- store config in ConfigHub
- bind it to a target
- apply it through a worker

## GUI Check

In the ConfigHub GUI:

1. open the new `${SPACE}` space
2. open `hello-backend`
3. inspect the YAML data
4. if you set a target, inspect the target binding too

## Cleanup

```bash
cub space delete "${SPACE}"
```

## How This Connects to the Layered Examples

The layered examples in this package use the same core idea, but add one new rule:

- instead of one unit in one space, each specialization stage gets its own variant unit in its own space

So:

- this hello world shows the base object model
- `single-component` shows the first layered variant chain
- the later examples show larger shared recipes and deployment variants
