// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package cli wires cub-namespace's cobra commands. It works identically whether
// run as `cub-namespace ...` standalone or as `cub namespace ...` via the cub
// plugin protocol; the only difference is cosmetic — cub sets CUB_PLUGIN=1 in
// the environment, which we surface in user-facing follow-up suggestions via
// InvocationName.
package cli

import (
	"os"

	"github.com/spf13/cobra"
)

// invokedAsPlugin reports whether the binary was launched by `cub namespace`.
func invokedAsPlugin() bool {
	return os.Getenv("CUB_PLUGIN") == "1"
}

// InvocationName is the command name to print in user-facing follow-up
// instructions. It returns "cub namespace" when launched via the cub plugin
// protocol so the operator can copy-paste the suggestion back into their shell;
// otherwise it returns os.Args[0] verbatim (e.g. "bin/cub-namespace",
// "cub-namespace"). Falls back to "cub-namespace" if os.Args is empty.
func InvocationName() string {
	if invokedAsPlugin() {
		return "cub namespace"
	}
	if len(os.Args) > 0 && os.Args[0] != "" {
		return os.Args[0]
	}
	return "cub-namespace"
}

// NewRoot builds the root cobra command with all subcommands attached.
func NewRoot() *cobra.Command {
	root := &cobra.Command{
		Use:   "cub-namespace",
		Short: "Manage Kubernetes namespaces and their policy envelope across a ConfigHub fleet",
		Long: `cub-namespace manages Kubernetes namespaces and their policy envelope —
pod-security labels, a default-deny NetworkPolicy, and baseline RBAC — stored as
data in ConfigHub Units across a fleet of cluster-Spaces. It is designed for use
by an AI agent in a terminal, and is a sibling of rbac-manager and
network-policy-manager.

This build ships read-only analysis — fleet inventory ('snapshot', 'list'),
per-namespace envelope completeness ('envelope'), cross-variant consistency
('consistency'), and ranked governance findings ('findings') — the fleet-wide
reads that per-resource validators and runtime tenancy controllers (Capsule,
HNC) cannot do. Later milestones add guardrailed scaffold / backfill edits that
author the envelope as Units without applying them to a cluster.

ConfigHub I/O uses your existing cub session — run 'cub auth login' first if you
are not signed in.`,
		SilenceUsage: true,
	}
	root.AddCommand(
		// Read / inventory
		newSnapshotCmd(),
		newListCmd(),
		// Analysis
		newEnvelopeCmd(),
		newConsistencyCmd(),
		newFindingsCmd(),
		// Write (config-as-data fixes; dry-run unless --commit)
		newApplyEnvelopeCmd(),
		newBackfillCmd(),
		newPromoteCmd(),
		newGuardrailsCmd(),
		// Diagnostics
		newPreflightCmd(),
		newVersionCmd(),
	)
	return root
}
