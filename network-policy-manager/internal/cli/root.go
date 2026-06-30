// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package cli wires cub-netpol's cobra commands. It works identically whether
// run as `cub-netpol ...` standalone or as `cub netpol ...` via the cub plugin
// protocol; the only difference is cosmetic — cub sets CUB_PLUGIN=1 in the
// environment, which we surface in user-facing follow-up suggestions via
// InvocationName.
package cli

import (
	"os"

	"github.com/spf13/cobra"
)

// invokedAsPlugin reports whether the binary was launched by `cub netpol`.
func invokedAsPlugin() bool {
	return os.Getenv("CUB_PLUGIN") == "1"
}

// InvocationName is the command name to print in user-facing follow-up
// instructions. It returns "cub netpol" when launched via the cub plugin
// protocol so the operator can copy-paste the suggestion back into their shell;
// otherwise it returns os.Args[0] verbatim (e.g. "bin/cub-netpol",
// "cub-netpol"). Falls back to "cub-netpol" if os.Args is empty.
func InvocationName() string {
	if invokedAsPlugin() {
		return "cub netpol"
	}
	if len(os.Args) > 0 && os.Args[0] != "" {
		return os.Args[0]
	}
	return "cub-netpol"
}

// NewRoot builds the root cobra command with all subcommands attached.
func NewRoot() *cobra.Command {
	root := &cobra.Command{
		Use:   "cub-netpol",
		Short: "Manage Kubernetes NetworkPolicy config-as-data across a ConfigHub fleet",
		Long: `cub-netpol manages Kubernetes NetworkPolicy config stored as data in ConfigHub
Units across a fleet of cluster-Spaces, reasoning about it together with the
Namespaces, workloads, and Services it must cover. It is designed for use by an
AI agent in a terminal.

This build ships read-only analysis — fleet inventory ('snapshot', 'list'),
coverage gaps ('coverage'), effective connectivity ('who-can-reach',
'reachable-from'), hygiene findings ('findings') — plus config-as-data fixes
that author NetworkPolicies as Units ('default-deny', 'allow'). Writes are
dry-run unless you pass --commit --change-desc, and create Units without applying
them to a cluster.

ConfigHub I/O uses your existing cub session — run 'cub auth login' first if you
are not signed in.`,
		SilenceUsage: true,
	}
	root.AddCommand(
		// Read / inventory
		newSnapshotCmd(),
		newListCmd(),
		// Analysis
		newCoverageCmd(),
		newWhoCanReachCmd(),
		newReachableFromCmd(),
		newFindingsCmd(),
		// Write (config-as-data fixes; dry-run unless --commit)
		newDefaultDenyCmd(),
		newAllowCmd(),
		newAllowFromLinksCmd(),
		newFixCmd(),
		newFleetCmd(),
		newPromoteCmd(),
		newGuardrailsCmd(),
		// Diagnostics
		newPreflightCmd(),
		newVersionCmd(),
	)
	return root
}
