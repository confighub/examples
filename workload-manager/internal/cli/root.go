// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package cli wires cub-workload's cobra commands. It works identically whether
// run as `cub-workload ...` standalone or as `cub workload ...` via the cub
// plugin protocol; the only difference is cosmetic — cub sets CUB_PLUGIN=1 in
// the environment, which we surface in user-facing follow-up suggestions via
// InvocationName.
package cli

import (
	"os"

	"github.com/spf13/cobra"
)

// invokedAsPlugin reports whether the binary was launched by `cub workload`.
func invokedAsPlugin() bool {
	return os.Getenv("CUB_PLUGIN") == "1"
}

// InvocationName is the command name to print in user-facing follow-up
// instructions. It returns "cub workload" when launched via the cub plugin
// protocol so the operator can copy-paste the suggestion back into their shell;
// otherwise it returns os.Args[0] verbatim (e.g. "bin/cub-workload",
// "cub-workload"). Falls back to "cub-workload" if os.Args is empty.
func InvocationName() string {
	if invokedAsPlugin() {
		return "cub workload"
	}
	if len(os.Args) > 0 && os.Args[0] != "" {
		return os.Args[0]
	}
	return "cub-workload"
}

// NewRoot builds the root cobra command with all subcommands attached.
func NewRoot() *cobra.Command {
	root := &cobra.Command{
		Use:   "cub-workload",
		Short: "Manage the security and reliability posture of Kubernetes workloads across a ConfigHub fleet",
		Long: `cub-workload manages the cross-cutting security and reliability posture of
Kubernetes workloads — container/pod security context, resource requests and
limits, probes, and availability (PodDisruptionBudget coverage plus pod
anti-affinity / topology spread) — stored as data in ConfigHub Units across a
fleet of cluster-Spaces. It is designed for use by an AI agent in a terminal,
and is a sibling of rbac-manager, network-policy-manager, and namespace-manager.

It reasons over the whole managed set (a fleet-wide production-readiness
scorecard) and fixes gaps as data through reusable, parameterized profiles —
committing new Unit revisions rather than editing live clusters. Its value over
per-object validators (kube-score, kube-linter) is the fix-as-data and the
fleet: bulk remediation across a selector and override-preserving promotion.

This build ships the M0 skeleton (auth preflight and version); later milestones
add the read-only scorecard ('snapshot', 'list', 'readiness', 'availability',
'findings') and guardrailed writes ('harden', 'set-resources', 'set-probes',
'ensure-pdb', 'ensure-spread', 'profile', 'fleet-edit', 'promote',
'guardrails').

ConfigHub I/O uses your existing cub session — run 'cub auth login' first if you
are not signed in.`,
		SilenceUsage: true,
	}
	root.AddCommand(
		// Read / inventory
		newSnapshotCmd(),
		newListCmd(),
		// Analysis
		newReadinessCmd(),
		newAvailabilityCmd(),
		newFindingsCmd(),
		// Write (config-as-data fixes; dry-run unless --commit)
		newHardenCmd(),
		newSetResourcesCmd(),
		newSetProbesCmd(),
		newEnsurePDBCmd(),
		newEnsureSpreadCmd(),
		newProfileCmd(),
		// Fleet ops + enforcement
		newFleetEditCmd(),
		newPromoteCmd(),
		newGuardrailsCmd(),
		// Diagnostics
		newPreflightCmd(),
		newVersionCmd(),
	)
	return root
}
