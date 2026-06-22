// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package cli wires cub-rbac's cobra commands. It works identically whether run
// as `cub-rbac ...` standalone or as `cub rbac ...` via the cub plugin protocol;
// the only difference is cosmetic — cub sets CUB_PLUGIN=1 in the environment,
// which we surface in user-facing follow-up suggestions via InvocationName.
package cli

import (
	"os"

	"github.com/spf13/cobra"
)

// invokedAsPlugin reports whether the binary was launched by `cub rbac`.
func invokedAsPlugin() bool {
	return os.Getenv("CUB_PLUGIN") == "1"
}

// InvocationName is the command name to print in user-facing follow-up
// instructions. It returns "cub rbac" when launched via the cub plugin protocol
// so the operator can copy-paste the suggestion back into their shell; otherwise
// it returns os.Args[0] verbatim (e.g. "bin/cub-rbac", "cub-rbac"). Falls back
// to "cub-rbac" if os.Args is empty.
func InvocationName() string {
	if invokedAsPlugin() {
		return "cub rbac"
	}
	if len(os.Args) > 0 && os.Args[0] != "" {
		return os.Args[0]
	}
	return "cub-rbac"
}

// NewRoot builds the root cobra command with all subcommands attached.
func NewRoot() *cobra.Command {
	root := &cobra.Command{
		Use:   "cub-rbac",
		Short: "Manage Kubernetes RBAC config-as-data across a ConfigHub fleet",
		Long: `cub-rbac manages Kubernetes RBAC config (Role, ClusterRole, RoleBinding,
ClusterRoleBinding, ServiceAccount) stored as data in ConfigHub Units across a
fleet of cluster-Spaces. It is designed for use by an AI agent in a terminal:
fleet inventory, effective-access ("who can") queries, hygiene findings, and
guardrailed edits.

All ConfigHub I/O is performed by shelling out to the cub CLI, so cub-rbac uses
your existing cub session — run 'cub auth login' first if you are not signed in.`,
		SilenceUsage: true,
	}
	root.AddCommand(
		// Read / query
		newSnapshotCmd(),
		newListCmd(),
		newWhoCanCmd(),
		newAccessCmd(),
		newFindingsCmd(),
		// Write
		newEditCmd(),
		newFleetEditCmd(),
		newPromoteCmd(),
		newGuardrailsCmd(),
		// Diagnostics
		newPreflightCmd(),
		newVersionCmd(),
	)
	return root
}
