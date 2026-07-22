// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package cli wires cub-eks's cobra commands. It works identically whether run
// as `cub-eks ...` standalone or as `cub eks ...` via the cub plugin protocol;
// the only difference is cosmetic — cub sets CUB_PLUGIN=1 in the environment,
// which we surface in user-facing follow-up suggestions via InvocationName.
package cli

import (
	"os"

	"github.com/spf13/cobra"
)

// invokedAsPlugin reports whether the binary was launched by `cub eks`.
func invokedAsPlugin() bool {
	return os.Getenv("CUB_PLUGIN") == "1"
}

// InvocationName is the command name to print in user-facing follow-up
// instructions. It returns "cub eks" when launched via the cub plugin protocol
// so the operator can copy-paste the suggestion back into their shell;
// otherwise it returns os.Args[0] verbatim (e.g. "bin/cub-eks", "cub-eks").
// Falls back to "cub-eks" if os.Args is empty.
func InvocationName() string {
	if invokedAsPlugin() {
		return "cub eks"
	}
	if len(os.Args) > 0 && os.Args[0] != "" {
		return os.Args[0]
	}
	return "cub-eks"
}

// NewRoot builds the root cobra command with all subcommands attached.
func NewRoot() *cobra.Command {
	root := &cobra.Command{
		Use:   "cub-eks",
		Short: "Manage AWS EKS clusters as Crossplane managed resources across a ConfigHub fleet",
		Long: `cub-eks manages AWS EKS clusters as data: the control plane, node capacity,
addons, access entries, and the VPC and IAM resources a cluster needs — stored
as ConfigHub Units containing Crossplane managed resources, across a fleet of
cluster-Spaces. It is designed for use by an AI agent in a terminal, and is a
sibling of workload-manager, namespace-manager, rbac-manager, and
network-policy-manager.

The command shape is eksctl's; the substrate is not. eksctl's config file is an
input to a one-shot run and the real state lives in CloudFormation. A Crossplane
managed resource IS the desired state, continuously reconciled — and ConfigHub
makes that state a versioned, fleet-queryable, promotable, gated source of
record. So a control-plane upgrade becomes a promotion, and a nodegroup scale
becomes a Unit mutation with a diff and a revision.

The differentiated capability is 'plan'. Crossplane does not replace a resource
when an immutable field changes — it refuses the update and retries forever,
leaving the Unit committed, applied, and permanently inert. cub-eks grades every
pending change as in-place, rolling, or a replacement, so that divergence is
caught at the source of record instead of in a controller log.

This build ships the M0 skeleton (auth preflight and version); later milestones
add the read commands ('snapshot', 'list', 'get', 'versions', 'status', 'plan',
'consistency', 'findings') and guardrailed writes ('create', 'upgrade', 'scale',
'replace-nodegroup', 'fleet-edit', 'promote', 'guardrails').

ConfigHub I/O uses your existing cub session — run 'cub auth login' first if you
are not signed in.`,
		SilenceUsage: true,
	}
	root.AddCommand(
		// Read / inventory
		newSnapshotCmd(),
		newListCmd(),
		newGetCmd(),
		newVersionsCmd(),
		newPlanCmd(),
		newFindingsCmd(),
		// Write (config-as-data generation; dry-run unless --commit)
		newCreateCmd(),
		newScaleCmd(),
		newUpgradeCmd(),
		newReplaceNodeGroupCmd(),
		// Fleet ops + enforcement
		newFleetEditCmd(),
		newPromoteCmd(),
		newGuardrailsCmd(),
		newAttributesCmd(),
		// Diagnostics
		newPreflightCmd(),
		newVersionCmd(),
	)
	return root
}
