// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

// This tool's guardrail pack. The mechanism -- policy Space, Triggers, shared
// Filter, wiring each in-scope Space, and reporting the Units a Trigger marked
// -- is managerkit/guardrails'. What is this tool's is the rules.

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/confighub/sdk/core/cubapi"

	"github.com/confighub/examples/autoscale-manager/internal/cub"
	"github.com/confighub/examples/managerkit/guardrails"
)

// celAutoscalerNotPinned passes unless an autoscaler is pinned (min == max, so
// it can't actually scale). An HPA's maxReplicas is required and minReplicas
// defaults to 1; a ScaledObject's bounds are both optional, so an absent bound
// is treated as unpinned.
const celAutoscalerNotPinned = "!(r.kind in ['HorizontalPodAutoscaler', 'ScaledObject'])" +
	" || (r.kind == 'HorizontalPodAutoscaler' && (!has(r.spec.minReplicas) || r.spec.minReplicas < r.spec.maxReplicas))" +
	" || (r.kind == 'ScaledObject' && (!has(r.spec.minReplicaCount) || !has(r.spec.maxReplicaCount) || r.spec.minReplicaCount < r.spec.maxReplicaCount))"

var pack = guardrails.Pack{
	App:          "autoscale-manager",
	DefaultSpace: "autoscale-policy",
	FilterSlug:   "autoscale-guardrails",
	Label:        "autoscale-guardrails",
	Rules: []guardrails.Rule{
		{Slug: "autoscaler-not-pinned",
			Description: "Warns on an HPA/ScaledObject with min == max (it can't actually scale). Fix: `cub-autoscale set-hpa --min <lower> --max <higher>` or a profile.",
			Expression:  celAutoscalerNotPinned},
		// Not a CEL rule: schema validation covers the KEDA ScaledObjects
		// `cub-autoscale convert-keda` produces, since keda.sh is in the catalog.
		{Slug: "schema-valid",
			Description: "Warns when a resource fails Kubernetes/CRD schema validation — covers KEDA ScaledObjects produced by `cub-autoscale convert-keda` (keda.sh is in the schema catalog).",
			Function:    "vet-schemas"},
	},
}

func preflight(ctx context.Context) (*cubapi.Client, error) { return cub.Preflight(ctx) }

func newGuardrailsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "guardrails",
		Short: "Install and inspect the autoscaling guardrail policy pack",
		Long: `guardrails manages a pack of autoscaling validation policies, defined once in a
policy Space and enforced fleet-wide via a shared Trigger Filter:

  autoscaler-not-pinned   an HPA/ScaledObject must not have min == max (it would
                          be unable to scale) — a per-resource vet-cel check
  schema-valid            a resource must pass Kubernetes/CRD schema validation
                          (vet-schemas) — this is the post-convert check for
                          convert-keda output: a committed ScaledObject fires the
                          Mutation trigger and is validated against keda.sh's schema

Both rules run per-resource (a single Unit answers each), created with Warn=true
(advisory ApplyWarnings, never blocking). Promote one to blocking with:
cub trigger update <slug> --space autoscale-policy --unwarn`,
	}
	cmd.AddCommand(pack.InstallCmd(preflight), pack.StatusCmd(preflight))
	return cmd
}
