// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Command cub-workload manages the cross-cutting security and reliability posture
// of Kubernetes workloads as data in ConfigHub, in a form usable by an AI agent
// in a terminal: fleet inventory, a per-workload production-readiness scorecard
// (security context, resources, probes, availability), and (in later milestones)
// guardrailed config-as-data fixes applied through reusable profiles.
//
// It can be run standalone (cub-workload ...) or as a cub plugin
// (cub workload ...). All ConfigHub I/O goes through the ConfigHub API directly
// via the github.com/confighub/sdk/core/cubapi package, using the ambient cub
// session.
package main

import (
	"fmt"
	"os"

	"github.com/confighub/sdk/core/plugin"

	"github.com/confighub/examples/workload-manager/internal/cli"
	"github.com/confighub/examples/workload-manager/internal/version"
)

func main() {
	// When cub installs or upgrades this plugin it invokes the binary as a
	// hook; HandleHook writes cub-plugin.yaml into the plugin directory and we
	// exit without running the normal command tree. The command token is
	// "workload" (so the plugin is invoked as `cub workload ...`), while the
	// binary basename is cub-workload (used as the entrypoint by default).
	manifest := plugin.Manifest{
		Name:    "cub-workload",
		Version: version.Version,
		Commands: []plugin.Command{{
			Name:    "workload",
			Summary: "Manage the security and reliability posture of Kubernetes workloads across a ConfigHub fleet",
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
