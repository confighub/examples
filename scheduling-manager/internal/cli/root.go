// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package cli wires cub-scheduling's cobra commands. It works identically whether
// run as `cub-scheduling ...` standalone or as `cub scheduling ...` via the cub
// plugin protocol.
package cli

import (
	"os"

	"github.com/spf13/cobra"
)

func invokedAsPlugin() bool { return os.Getenv("CUB_PLUGIN") == "1" }

// InvocationName is the command name to print in user-facing follow-ups.
func InvocationName() string {
	if invokedAsPlugin() {
		return "cub scheduling"
	}
	if len(os.Args) > 0 && os.Args[0] != "" {
		return os.Args[0]
	}
	return "cub-scheduling"
}

// NewRoot builds the root cobra command with all subcommands attached.
func NewRoot() *cobra.Command {
	root := &cobra.Command{
		Use:   "cub-scheduling",
		Short: "Manage Kubernetes workload placement (nodeSelector, tolerations, node affinity) across a ConfigHub fleet",
		Long: `cub-scheduling manages where Kubernetes workloads are allowed to land —
nodeSelector, tolerations, and node affinity — stored as data in ConfigHub Units
across a fleet of cluster-Spaces. It is designed for use by an AI agent in a
terminal, and is a sibling of rbac-manager, network-policy-manager,
namespace-manager, and workload-manager.

Placement is "which node a pod lands on". Spreading a workload's own replicas
(pod anti-affinity, topology spread) is an availability concern owned by
workload-manager, not this tool.

This build ships read-only analysis — fleet inventory ('snapshot', 'list'),
per-workload placement ('placement'), and ranked findings ('findings'). Later
milestones add guardrailed placement edits (nodeSelector / tolerations / node
affinity) via reusable profiles.

ConfigHub I/O uses your existing cub session — run 'cub auth login' first if you
are not signed in.`,
		SilenceUsage: true,
	}
	root.AddCommand(
		// Read / inventory
		newSnapshotCmd(),
		newListCmd(),
		// Analysis
		newPlacementCmd(),
		newFindingsCmd(),
		// Write (config-as-data fixes; dry-run unless --commit)
		newSetNodeSelectorCmd(),
		newSetTolerationsCmd(),
		newSetNodeAffinityCmd(),
		newProfileCmd(),
		newFleetEditCmd(),
		newPromoteCmd(),
		newGuardrailsCmd(),
		// Diagnostics
		newPreflightCmd(),
		newVersionCmd(),
	)
	return root
}
