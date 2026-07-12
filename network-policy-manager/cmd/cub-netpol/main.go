// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Command cub-netpol manages Kubernetes NetworkPolicy config as data across a
// ConfigHub fleet. It can be run standalone (cub-netpol ...) or as a cub plugin
// (cub netpol ...).
package main

import (
	"fmt"
	"os"

	"github.com/confighub/sdk/core/plugin"

	"github.com/confighub/examples/network-policy-manager/internal/cli"
	"github.com/confighub/examples/network-policy-manager/internal/version"
)

func main() {
	manifest := plugin.Manifest{
		Name:    "cub-netpol",
		Version: version.Version,
		Commands: []plugin.Command{{
			Name:    "netpol",
			Summary: "Manage Kubernetes NetworkPolicy config-as-data across a ConfigHub fleet",
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
