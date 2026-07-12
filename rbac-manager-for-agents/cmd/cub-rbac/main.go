// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Command cub-rbac manages Kubernetes RBAC config as data across a ConfigHub
// fleet. It can be run standalone (cub-rbac ...) or as a cub plugin
// (cub rbac ...).
package main

import (
	"fmt"
	"os"

	"github.com/confighub/sdk/core/plugin"

	"github.com/confighub/examples/rbac-manager-for-agents/internal/cli"
	"github.com/confighub/examples/rbac-manager-for-agents/internal/version"
)

func main() {
	manifest := plugin.Manifest{
		Name:    "cub-rbac",
		Version: version.Version,
		Commands: []plugin.Command{{
			Name:    "rbac",
			Summary: "Manage Kubernetes RBAC config-as-data across a ConfigHub fleet",
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
