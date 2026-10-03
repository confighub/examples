# Hello World Custom Function

This example registers a custom ConfigHub function named `hello-world`.
When you invoke it, the function adds the annotation
`confighub-example/hello-world-greeting` to every Kubernetes resource in the
selected unit.

## What This Example Proves

- how to register a custom function in a worker
- how to accept one required string parameter
- how to mutate `Kubernetes/YAML` unit data
- how to run the worker locally, connected to ConfigHub as a Worker

This example mutates ConfigHub unit data. It does not apply anything to a
cluster unless you later choose to apply the unit through some other workflow.

## Fastest Path

Prerequisites:

- `cub auth login`
- `go`
- access to a live ConfigHub org

From this directory:

```bash
SPACE="hello-world-function-demo-$(( RANDOM % 9000 + 1000 ))"
WORKER="hello-world-worker"
UNIT="hello-world-demo"

cub space create "$SPACE"
go build -o ./hello-world-function .
```

Start the worker in one terminal:

```bash
cub worker create --space "$SPACE" "$WORKER"
eval "$(cub worker get-envs --space "$SPACE" "$WORKER")"
export CONFIGHUB_URL=https://hub.confighub.com
./hello-world-function
```

In a second terminal, create a test unit and invoke the function:

```bash
cub unit create --space "$SPACE" "$UNIT" test_input.yaml

cub function do \
  --space "$SPACE" \
  --worker "$WORKER" \
  --where "Slug = '$UNIT'" \
  --output-only \
  hello-world "Hello from ConfigHub!"
```

Success looks like returned YAML containing:

```yaml
metadata:
  annotations:
    confighub-example/hello-world-greeting: Hello from ConfigHub!
```

If you want a preview without saving the mutation back to ConfigHub, add
`--dry-run` to `cub function do`.

## One-Command Demo

Run:

```bash
bash demo.sh
```

The demo script:

- builds the worker binary
- creates a temporary space
- runs the worker locally
- creates a sample unit from `test_input.yaml`
- invokes `hello-world`
- verifies that the greeting annotation was added
- cleans up the temporary space

Set `NOCLEANUP=1` if you want to keep the demo space for inspection.

## File Guide

- [main.go](./main.go) registers the function with a worker connector
- [hello_world_function.go](./hello_world_function.go) defines the function
  signature and mutation logic
- [test_input.yaml](./test_input.yaml) is the sample unit used in the quick
  path and demo
- [Dockerfile](./Dockerfile) packages the worker for container-based runs

## How The Function Works

The worker starts in [main.go](./main.go). It registers one function against the
`Kubernetes/YAML` toolchain and then opens a worker connector using:

- `CONFIGHUB_URL`
- `CONFIGHUB_WORKER_ID`
- `CONFIGHUB_WORKER_SECRET`

`cub worker get-envs` prints the Worker's ID and secret; `CONFIGHUB_URL` is the server you are logged in to.

The actual mutation lives in [hello_world_function.go](./hello_world_function.go):

- the function signature is named `hello-world`
- it requires one parameter, `greeting`
- it walks each YAML document in the unit
- it sets `metadata.annotations.confighub-example/hello-world-greeting`

## Adapting This Example

If you want to turn this into your own function, the smallest set of changes is:

1. Rename the function in `GetHelloWorldFunctionSignature()`.
2. Replace the mutation in `HelloWorldFunction()`.
3. Update the example invocation in this README and `demo.sh`.

## Cleanup

When you are done:

```bash
cub space delete "$SPACE" --force
```
