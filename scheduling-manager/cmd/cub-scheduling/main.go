// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Command cub-scheduling manages the placement of Kubernetes workloads as data in
// ConfigHub — where a pod is allowed to land: nodeSelector, tolerations, and node
// affinity — across a fleet, in a form usable by an AI agent in a terminal.
// (Spreading a workload's own replicas — pod anti-affinity / topology spread — is
// an availability concern owned by cub-workload, not placement.)
//
// It can be run standalone (cub-scheduling ...) or as a cub plugin
// (cub scheduling ...). All ConfigHub I/O goes through the ConfigHub API directly
// via the github.com/confighub/sdk/core/cubapi package, using the ambient cub
// session.
package main

import (
	"fmt"
	"os"

	"github.com/confighub/sdk/core/plugin"

	"github.com/confighub/examples/scheduling-manager/internal/cli"
	"github.com/confighub/examples/scheduling-manager/internal/version"
)

func main() {
	manifest := plugin.Manifest{
		Name:    "cub-scheduling",
		Version: version.Version,
		Commands: []plugin.Command{{
			Name:    "scheduling",
			Summary: "Manage Kubernetes workload placement (nodeSelector, tolerations, node affinity) across a ConfigHub fleet",
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
