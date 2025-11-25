# ConfigHub Examples

This repo contains examples that demonstrate how [ConfigHub](https://confighub.com) works in various scenarios. You'll need access to ConfigHub to go through the examples. ConfigHub is currently in gated preview. Please [sign up](https://auth.confighub.com/sign-up) and wait to be activated.

## How to use this repo

Clone this repo to your local machine and make sure you have `cub` installed and that you are logged in. Each example is contained in its own directory. To try out an example, `cd` into the directory and follow the instructions in the README.

Some examples will include running a local Kind Kubernetes cluster on your machine so it is a good idea to have Kind installed as well.

## General structure and script behavior

Scripts and commands in this repo are designed to not interfere with other resources already in your organization. **BUT run them at your own risk**. You should check the contents of scripts before executing them. This will help you understand how the CLI commands work and can also prevent any unforseen accidents.

Scripts are usually located in a `bin` directory in the example dir and are executed from the example root, e.g. `bin/cleanup`. There are usually at least an `install` script and a `cleanup` script.

## Examples

* [Global App](global-app/README.md) - demonstrates how to use ConfigHub to manage a typical micro-service application deployed in different variants for testing, staging, and production across multiple regions
* [Helm Platform Components](helm-platform-components/README.md) - demonstrates how to render Helm charts into ConfigHub and customize the config inside ConfigHub
