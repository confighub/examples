// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package cli wires cub-observability's cobra commands. It works identically
// whether run as `cub-observability ...` standalone or as `cub observability ...`
// via the cub plugin protocol.
package cli

import (
	"os"

	"github.com/spf13/cobra"
)

func invokedAsPlugin() bool { return os.Getenv("CUB_PLUGIN") == "1" }

// InvocationName is the command name to print in user-facing follow-ups.
func InvocationName() string {
	if invokedAsPlugin() {
		return "cub observability"
	}
	if len(os.Args) > 0 && os.Args[0] != "" {
		return os.Args[0]
	}
	return "cub-observability"
}

// NewRoot builds the root cobra command with all subcommands attached.
func NewRoot() *cobra.Command {
	root := &cobra.Command{
		Use:   "cub-observability",
		Short: "Manage Prometheus ServiceMonitor coverage and telemetry sidecars across a ConfigHub fleet",
		Long: `cub-observability manages the observability posture of Kubernetes workloads
stored as data in ConfigHub across a fleet: Prometheus ServiceMonitor coverage of
metrics-exposing Services, and OpenTelemetry / telemetry sidecar injection. It is
designed for use by an AI agent in a terminal, and is a sibling of rbac-manager,
network-policy-manager, namespace-manager, workload-manager, and
scheduling-manager.

ServiceMonitor coverage is a cross-Unit property: the ServiceMonitor and the
Service it selects live in separate Units, so a per-Unit validator can't tell
whether a metrics Service is actually scraped. This build ships read-only
analysis — 'snapshot', 'list', 'coverage', 'sidecars', 'findings' — and later
milestones add guardrailed writes (author a ServiceMonitor, inject an otel
sidecar via set-path) through reusable profiles.

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
		newSidecarsCmd(),
		newFindingsCmd(),
		// Write (config-as-data fixes; dry-run unless --commit)
		newEnsureServiceMonitorCmd(),
		newInjectSidecarCmd(),
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
