# Managing Helm-sourced platform components

When running applications on Kubernetes, it is common to install a set of _platform_ components that provide supporting services. Helm is one of the most popular tools for doing this.

This example demonstrates how you can install and maintain such platform components across many environments using ConfigHub.

You will find instructions for this example in [ConfigHub Docs](https://docs.confighub.com/get-started/examples/helm-platform-components/).

If you want the maintained repo-side Helm provenance and governed change path,
use [`cub-gen/examples/helm-paas`](https://github.com/confighub/cub-gen/tree/main/examples/helm-paas). That example focuses on chart/value ownership, overlay
selection, governed ALLOW/BLOCK proof, and connected/live Helm runtime
evidence.
