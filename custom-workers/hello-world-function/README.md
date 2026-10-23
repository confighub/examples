# Hello World Custom Function

This example registers a custom ConfigHub function named `hello-world`.
When you invoke it, the function adds the annotation
`confighub-example/hello-world-greeting` to every Kubernetes resource in the
selected unit.

The function runs in a function bot: a process that claims a ConfigHub
Channel and runs the functions invoked through it. ConfigHub never connects to
the bot. The bot polls its Channel for work, so it can run anywhere that can
reach ConfigHub.

## What This Example Proves

- how to register a custom function with a function bot
- how to accept one required string parameter
- how to mutate `Kubernetes/YAML` unit data
- how to serve the function from a Channel, first as yourself and then as a
  service account

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
CHANNEL="functions"
UNIT="hello-world-demo"

cub space create "$SPACE"
cub channel create --space "$SPACE" "$CHANNEL"
go build -o ./hello-world-function .
```

Start the function bot in one terminal:

```bash
./hello-world-function "$SPACE/$CHANNEL"
```

The bot authenticates as you, through your active cub context. When it
starts, it claims the Channel and declares the functions it serves on it.

In a second terminal, create a test unit and invoke the function:

```bash
cub unit create --space "$SPACE" "$UNIT" test_input.yaml

cub function do \
  --space "$SPACE" \
  --channel "$SPACE/$CHANNEL" \
  --where "Slug = '$UNIT'" \
  --show data \
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

To see what the bot declared, run:

```bash
cub command list --space "$SPACE"
cub channel get "$SPACE/$CHANNEL"
```

## Running It As A Service Account

A deployed bot authenticates as a service account. The service account needs
only Claim on its Channel, and whoever sets up the Channel declares the
bot's functions on it:

```bash
SA="hello-world-bot"

cub serviceaccount create "$SA"
cub serviceaccount key add "$SA" --generate ./hello-world-bot.jwk

./hello-world-function --print-commands |
  cub channel create --space "$SPACE" "$CHANNEL" \
    --permission "Claim:serviceaccount:$SA" \
    --commands -
```

If the Channel already exists from the fastest path, use
`cub channel update --patch "$SPACE/$CHANNEL" --permission "Claim:serviceaccount:$SA" --commands -`
instead of `cub channel create`.

Then run the bot with the service account's private key:

```bash
export CONFIGHUB_URL=https://hub.confighub.com
export CONFIGHUB_AUTH_PRIVATE_KEY="$(cat ./hello-world-bot.jwk)"
./hello-world-function "$SPACE/$CHANNEL"
```

The bot finds its functions already declared and writes nothing to the
Channel. When you change the functions the bot serves, run the
`--print-commands` pipeline again with `cub channel update --patch` to update
the declaration.

A bot may also declare its functions itself, as it does in the fastest path.
That needs Edit on the Channel as well as Claim, given with a second
`--permission "Edit:serviceaccount:$SA"`.

The private key is a credential. Keep it out of source control and give it to
a deployed bot as a secret.

## One-Command Demo

Run:

```bash
bash demo.sh
```

The demo script:

- builds the function bot
- creates a temporary space and a Channel in it
- runs the bot locally, as you
- creates a sample unit from `test_input.yaml`
- invokes `hello-world` through the Channel
- verifies that the greeting annotation was added
- cleans up the temporary space

Set `NOCLEANUP=1` if you want to keep the demo space for inspection.

## File Guide

- [main.go](./main.go) registers the function and serves it on a Channel
- [hello_world_function.go](./hello_world_function.go) defines the function
  signature and mutation logic
- [test_input.yaml](./test_input.yaml) is the sample unit used in the quick
  path and demo
- [Dockerfile](./Dockerfile) packages the bot for container-based runs

## How The Function Works

The bot starts in [main.go](./main.go). It registers one function against the
`Kubernetes/YAML` toolchain and then starts a `channel.Connector` using:

- the Channel, as the first argument or `CONFIGHUB_CHANNEL`
- `CONFIGHUB_URL` and `CONFIGHUB_AUTH_PRIVATE_KEY`, when it runs as a
  service account

Without `CONFIGHUB_AUTH_PRIVATE_KEY`, the bot uses your active cub context.

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
cub space delete "$SPACE" --recursive
cub serviceaccount delete hello-world-bot
rm -f ./hello-world-bot.jwk
```
