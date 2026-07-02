// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package cli wires cub-autoscale's cobra commands. It works identically whether
// run as `cub-autoscale ...` standalone or as `cub autoscale ...` via the cub
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
		return "cub autoscale"
	}
	if len(os.Args) > 0 && os.Args[0] != "" {
		return os.Args[0]
	}
	return "cub-autoscale"
}

// NewRoot builds the root cobra command with all subcommands attached.
func NewRoot() *cobra.Command {
	root := &cobra.Command{
		Use:   "cub-autoscale",
		Short: "Manage HorizontalPodAutoscalers and KEDA ScaledObjects across a ConfigHub fleet",
		Long: `cub-autoscale manages Kubernetes autoscaling stored as data in ConfigHub across
a fleet: HorizontalPodAutoscalers and their conversion to KEDA ScaledObjects. It
is designed for use by an AI agent in a terminal, and is a sibling of
rbac-manager, network-policy-manager, namespace-manager, workload-manager,
scheduling-manager, and observability-manager.

The HPA→KEDA conversion (convert-keda) runs the convert-hpa-to-keda ConfigHub
function in an embedded executor in-process: it fetches the Unit's data, runs the
function locally, and writes the result back — no server-side function or worker
required.

Read-only analysis ('snapshot', 'list', 'findings') surveys autoscaling coverage
across the fleet, including the cross-resource check for a PodDisruptionBudget
that would block scale-down at an autoscaler's minReplicas.

Writes are config-as-data and dry-run unless --commit: 'set-hpa' edits an HPA's
bounds/targets, 'convert-keda' rewrites an HPA as a KEDA ScaledObject, 'profile'
applies reusable parameterized edits, 'fleet-edit' applies one across a selector,
'promote' upgrades downstream Units from upstream, and 'guardrails' installs the
vet-cel policy pack (an autoscaler must not be pinned).

ConfigHub I/O uses your existing cub session — run 'cub auth login' first if you
are not signed in.`,
		SilenceUsage: true,
	}
	root.AddCommand(
		// Read (fleet analysis)
		newSnapshotCmd(),
		newListCmd(),
		newFindingsCmd(),
		// Write (config-as-data fixes; dry-run unless --commit)
		newSetHPACmd(),
		newConvertCmd(),
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
