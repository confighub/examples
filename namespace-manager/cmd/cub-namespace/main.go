// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Command cub-namespace manages Kubernetes namespaces and their policy
// envelope across a ConfigHub fleet. It can be run standalone
// (cub-namespace ...) or as a cub plugin (cub namespace ...).
package main

import (
	"fmt"
	"os"

	"github.com/confighub/sdk/core/plugin"

	"github.com/confighub/examples/namespace-manager/internal/cli"
	"github.com/confighub/examples/namespace-manager/internal/version"
)

func main() {
	manifest := plugin.Manifest{
		Name:    "cub-namespace",
		Version: version.Version,
		Commands: []plugin.Command{{
			Name:    "namespace",
			Summary: "Manage Kubernetes namespaces and their policy envelope across a ConfigHub fleet",
		}},
	}
	if handled, err := plugin.HandleHook(manifest); handled {
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	if err := cli.NewRoot().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
