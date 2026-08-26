// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

// This tool's guardrail pack. The mechanism -- policy Space, Triggers, shared
// Filter, wiring each in-scope Space, and reporting the Units a Trigger marked
// -- is managerkit/guardrails'. What is this tool's is the rules, and the annotate command that feeds one of them.

import (
	"context"
	"sort"

	"fmt"
	"github.com/confighub/examples/observability-manager/internal/observability"
	"github.com/confighub/examples/observability-manager/internal/snapshot"
	"github.com/confighub/sdk/cliutil"

	"github.com/spf13/cobra"

	"github.com/confighub/sdk/core/cubapi"

	"github.com/confighub/examples/managerkit/guardrails"
	"github.com/confighub/examples/observability-manager/internal/cub"
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
	cmd.AddCommand(pack.InstallCmd(preflight), pack.StatusCmd(preflight), newGuardrailsAnnotateCmd())
	return cmd
}

type annotateResult struct {
	Cluster   string `json:"cluster"`
	Namespace string `json:"namespace"`
	Unit      string `json:"unit"`
	Service   string `json:"service"`
	OK        bool   `json:"ok"`
	Error     string `json:"error,omitempty"`
}

func newGuardrailsAnnotateCmd() *cobra.Command {
	var output, clusterFilter string
	var filter filterFlags
	var commit cliutil.CommitFlags
	cmd := &cobra.Command{
		Use:   "annotate",
		Short: "Annotate each uncovered metrics-Service Unit with a coverage finding (dry-run unless --commit)",
		Long: `annotate runs the coverage analysis and writes an observability.confighub.com/
coverage annotation onto each metrics-exposing Service Unit that no ServiceMonitor
selects. Paired with the servicemonitor-coverage Trigger from 'guardrails
install', this turns the cross-Unit coverage finding into an advisory ApplyWarning.

Re-run after adding ServiceMonitors. Dry run unless --commit --change-desc.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			changeDesc, dryRun, err := commit.Validate("annotate Service Units with ServiceMonitor-coverage findings")
			if err != nil {
				return err
			}
			client, err := cub.Preflight(cmd.Context())
			if err != nil {
				return err
			}
			snap, err := snapshot.Load(cmd.Context(), client, filter.Predicate())
			if err != nil {
				return err
			}
			var results []annotateResult
			for _, r := range observability.AnalyzeCoverage(snap.Clusters) {
				if r.Covered || r.SpaceID == "" || r.UnitSlug == "" {
					continue
				}
				if clusterFilter != "" && r.Cluster != clusterFilter {
					continue
				}
				ar := annotateResult{Cluster: r.Cluster, Namespace: r.Namespace, Unit: r.UnitSlug, Service: r.Service}
				if !dryRun {
					err := guardrails.Annotate(cmd.Context(), client, r.SpaceID, r.UnitSlug, coverageAnnotation, "uncovered", changeDesc)
					ar.OK = err == nil
					if err != nil {
						ar.Error = err.Error()
					}
				}
				results = append(results, ar)
			}
			sort.Slice(results, func(i, j int) bool {
				if results[i].Cluster != results[j].Cluster {
					return results[i].Cluster < results[j].Cluster
				}
				return results[i].Unit < results[j].Unit
			})
			if output == outputJSON {
				return printJSON(cmd.OutOrStdout(), results)
			}
			out := cmd.OutOrStdout()
			for _, r := range results {
				fprintln(out, fmt.Sprintf("%s/%s  %s  (service: %s)", r.Cluster, dash(r.Namespace), r.Unit, r.Service))
			}
			if dryRun {
				fprintln(out, fmt.Sprintf("\nDry run — %d Service Unit(s) would be annotated. Re-run with --commit --change-desc \"…\".", len(results)))
			} else {
				fprintln(out, fmt.Sprintf("\nAnnotated %d Service Unit(s).", len(results)))
			}
			return nil
		},
	}
	addOutputFlag(cmd, &output)
	addFilterFlags(cmd, &filter)
	cmd.Flags().StringVar(&clusterFilter, "cluster", "", "only annotate Units in this cluster")
	commit.Bind(cmd)
	return cmd
}
