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

	"github.com/confighub/examples/managerkit/guardrails"
	"github.com/confighub/examples/scheduling-manager/internal/cub"
)

const controllerKinds = "['Deployment', 'StatefulSet', 'DaemonSet', 'ReplicaSet']"

// celTolerationNeedsPlacement passes unless a controller has tolerations but
// neither a non-empty nodeSelector nor a required node affinity (i.e. it tolerates
// a taint but doesn't actually pin where it lands).
const celTolerationNeedsPlacement = "!(r.kind in " + controllerKinds + ")" +
	" || !has(r.spec.template.spec.tolerations)" +
	" || (has(r.spec.template.spec.nodeSelector) && size(r.spec.template.spec.nodeSelector) > 0)" +
	" || (has(r.spec.template.spec.affinity) && has(r.spec.template.spec.affinity.nodeAffinity) && has(r.spec.template.spec.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution))"

var pack = guardrails.Pack{
	Label: "scheduling-guardrails",
	Rules: []guardrails.Rule{
		{Slug: "workload-toleration-needs-placement",
			Description: "Warns on a controller that tolerates a taint but has no nodeSelector or required node affinity (may schedule onto general nodes). Fix: `cub-scheduling set-node-selector` / `set-node-affinity`, or a placement profile.",
			Expression:  celTolerationNeedsPlacement},
	},
}

func preflight(ctx context.Context) (*cubapi.Client, error) { return cub.Preflight(ctx) }

func newGuardrailsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "guardrails",
		Short: "Install and inspect the placement guardrail policy pack",
		Long: `guardrails manages a pack of placement validation policies, defined once in a
policy Space and enforced fleet-wide via a shared Trigger Filter:

  workload-toleration-needs-placement   a controller that tolerates a taint must
                                        pin where it lands (nodeSelector or
                                        required node affinity)

The rule is a plain per-resource vet-cel check (a single Unit answers it), created
with Warn=true (advisory ApplyWarnings, never blocking). Promote it to blocking
with: cub trigger update <slug> --space scheduling-policy --unwarn`,
	}
	cmd.AddCommand(pack.InstallCmd(preflight), pack.StatusCmd(preflight))
	return cmd
}
