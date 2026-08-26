// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

// This tool's guardrail pack. The mechanism -- policy Space, Triggers, shared
// Filter, wiring each in-scope Space, and reporting the Units a Trigger marked
// -- is managerkit/guardrails'. What is this tool's is the rules, and which
// Units the annotate command has something to say about.

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/confighub/sdk/core/cubapi"

	"github.com/confighub/examples/managerkit/guardrails"
	"github.com/confighub/examples/observability-manager/internal/cub"
	"github.com/confighub/examples/observability-manager/internal/observability"
	"github.com/confighub/examples/observability-manager/internal/snapshot"
)

// coverageAnnotation is written by `guardrails annotate` onto each uncovered
// metrics-Service Unit, and read by the coverage rule (annotate-then-validate --
// coverage is a cross-Unit property a per-Unit CEL check cannot compute).
const coverageAnnotation = "observability.confighub.com/coverage"

// The null check is required: a resource written with a bare `annotations:` key has
// the key present with a null value, so has() is true but `in` against null raises
// "no such overload" and the rule reports a spurious failure.
const celNoCoverageFinding = "!has(r.metadata.annotations) || r.metadata.annotations == null || !('" + coverageAnnotation + "' in r.metadata.annotations)"

var pack = guardrails.Pack{
	Label: "observability-guardrails",
	Rules: []guardrails.Rule{
		{Slug: "servicemonitor-coverage",
			Description: "Warns while an observability.confighub.com/coverage annotation is present (set by `cub-observability guardrails annotate`) — a metrics Service with no ServiceMonitor. Fix: `cub-observability ensure-servicemonitor`, then re-run annotate.",
			Expression:  celNoCoverageFinding},
	},
}

func preflight(ctx context.Context) (*cubapi.Client, error) { return cub.Preflight(ctx) }

func newGuardrailsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "guardrails",
		Short: "Install, inspect, and feed the observability guardrail policy pack",
		Long: `guardrails manages the observability guardrail pack, defined once in a policy
Space and enforced fleet-wide via a shared Trigger Filter:

  servicemonitor-coverage   warns while a Service is annotated as having no
                            ServiceMonitor (a metrics Service that isn't scraped)

ServiceMonitor coverage is a cross-Unit property (the ServiceMonitor and the
Service live in separate Units), so a per-Unit vet-cel can't compute it directly.
This realizes annotate-then-validate: 'guardrails annotate' writes the coverage
finding onto each uncovered metrics-Service Unit, and this Trigger turns it into
an advisory ApplyWarning. Triggers are Warn=true; promote to blocking with
  cub trigger update servicemonitor-coverage --space observability-policy --unwarn`,
	}
	cmd.AddCommand(pack.InstallCmd(preflight), pack.StatusCmd(preflight), pack.AnnotateCmd(preflight, annotateSpec))
	return cmd
}

// annotateSpec is the observability half of `guardrails annotate`: which Service
// Units nothing scrapes, and what to write on them. The rest of the command is
// managerkit/guardrails'.
var annotateSpec = guardrails.AnnotateSpec{
	Key:   coverageAnnotation,
	Noun:  "Service Unit",
	Short: "Annotate each uncovered metrics-Service Unit with a coverage finding (dry-run unless --commit)",
	Long: `annotate runs the coverage analysis and writes an observability.confighub.com/
coverage annotation onto each metrics-exposing Service Unit that no ServiceMonitor
selects. Paired with the servicemonitor-coverage rule from 'guardrails install',
this turns the cross-Unit coverage finding into an advisory ApplyWarning.

Re-run after adding ServiceMonitors. Dry run unless --commit --change-desc.`,
	Targets: func(ctx context.Context, c *cubapi.Client, where, cluster string) ([]guardrails.Target, error) {
		snap, err := snapshot.Load(ctx, c, where)
		if err != nil {
			return nil, err
		}
		var out []guardrails.Target
		for _, r := range observability.AnalyzeCoverage(snap.Clusters) {
			if r.Covered || r.SpaceID == "" || r.UnitSlug == "" {
				continue
			}
			if cluster != "" && r.Cluster != cluster {
				continue
			}
			out = append(out, guardrails.Target{
				SpaceID: r.SpaceID, UnitSlug: r.UnitSlug, Value: "uncovered",
				Cluster: r.Cluster, Namespace: r.Namespace,
				Detail: "service: " + r.Service,
			})
		}
		return out, nil
	},
}
