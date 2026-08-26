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
	"github.com/confighub/examples/workload-manager/internal/snapshot"
	"github.com/confighub/examples/workload-manager/internal/workload"
	"github.com/confighub/sdk/cliutil"

	"github.com/spf13/cobra"

	"github.com/confighub/sdk/core/cubapi"

	"github.com/confighub/examples/managerkit/guardrails"
	"github.com/confighub/examples/workload-manager/internal/cub"
)

// pdbCoverageAnnotation is written by `guardrails annotate` onto each uncovered
// multi-replica workload Unit, and read by the pdb-coverage rule
// (annotate-then-validate -- the one finding a per-Unit CEL check cannot see).
const pdbCoverageAnnotation = "workload.confighub.com/pdb-coverage"

const controllerKinds = "['Deployment', 'StatefulSet', 'DaemonSet', 'ReplicaSet']"

const (
	celHasLimits = "!(r.kind in " + controllerKinds + ") || (has(r.spec.template.spec.containers) && r.spec.template.spec.containers.all(c, has(c.resources) && has(c.resources.limits) && 'memory' in c.resources.limits))"

	celRunsNonRoot = "!(r.kind in " + controllerKinds + ") || (has(r.spec.template.spec.securityContext) && has(r.spec.template.spec.securityContext.runAsNonRoot) && r.spec.template.spec.securityContext.runAsNonRoot) || (has(r.spec.template.spec.containers) && r.spec.template.spec.containers.all(c, has(c.securityContext) && has(c.securityContext.runAsNonRoot) && c.securityContext.runAsNonRoot))"

	celTerminationMsg = "!(r.kind in " + controllerKinds + ") || (has(r.spec.template.spec.containers) && r.spec.template.spec.containers.all(c, has(c.terminationMessagePolicy) && c.terminationMessagePolicy == 'FallbackToLogsOnError'))"

	// Annotate-then-validate: warn while a pdb-coverage finding annotation is
	// present. The null check is required: a resource written with a bare
	// `annotations:` key has the key present with a null value, so has() is true
	// but `in` against null raises "no such overload" and the rule reports a
	// spurious failure.
	celNoPDBFinding = "!has(r.metadata.annotations) || r.metadata.annotations == null || !('" + pdbCoverageAnnotation + "' in r.metadata.annotations)"
)

var pack = guardrails.Pack{
	Label: "workload-guardrails",
	Rules: []guardrails.Rule{
		{Slug: "workload-has-limits", Description: "Warns on a controller whose containers don't all set resources.limits.memory. Fix: `cub-workload set-resources <space>/<unit>`.", Expression: celHasLimits},
		{Slug: "workload-runs-nonroot", Description: "Warns on a controller not running as non-root (pod or all containers). Fix: `cub-workload harden <space>/<unit>`.", Expression: celRunsNonRoot},
		{Slug: "workload-termination-message-policy", Description: "Warns on a controller whose containers don't all set terminationMessagePolicy: FallbackToLogsOnError. Fix: the termination-message-policy profile.", Expression: celTerminationMsg},
		{Slug: "workload-pdb-coverage", Description: "Warns while a workload.confighub.com/pdb-coverage annotation is present (set by `cub-workload guardrails annotate`). Fix: `cub-workload ensure-pdb`, then re-run annotate.", Expression: celNoPDBFinding},
	},
}

func preflight(ctx context.Context) (*cubapi.Client, error) { return cub.Preflight(ctx) }

func newGuardrailsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "guardrails",
		Short: "Install, inspect, and feed the workload-readiness guardrail policy pack",
		Long: `guardrails manages a pack of workload-readiness validation policies, defined once
in a policy Space and enforced fleet-wide via a shared Trigger Filter:

  workload-has-limits                controller containers set a memory limit
  workload-runs-nonroot              controller runs as non-root
  workload-termination-message-policy  containers set FallbackToLogsOnError
  workload-pdb-coverage              warns while a PDB-coverage finding is annotated

The first three are plain per-resource vet-cel checks — a single Unit answers
them, no annotation needed. The last realizes annotate-then-validate for the one
property vet-cel can't see under one-resource-per-Unit: whether a *matching* PDB
exists in some other Unit. 'guardrails annotate' writes that finding onto each
uncovered workload Unit and this Trigger turns it into a warning.

Triggers are created with Warn=true (advisory ApplyWarnings, never blocking).
Promote one to blocking later with:
  cub trigger update <slug> --space <policy-space> --unwarn`,
	}
	cmd.AddCommand(pack.InstallCmd(preflight), pack.StatusCmd(preflight), newGuardrailsAnnotateCmd())
	return cmd
}

type annotateResult struct {
	Cluster   string `json:"cluster"`
	Namespace string `json:"namespace"`
	Unit      string `json:"unit"`
	Finding   string `json:"finding"`
	OK        bool   `json:"ok"`
	Error     string `json:"error,omitempty"`
}

func newGuardrailsAnnotateCmd() *cobra.Command {
	var output, clusterFilter string
	var filter filterFlags
	var commit cliutil.CommitFlags
	cmd := &cobra.Command{
		Use:   "annotate",
		Short: "Annotate each uncovered multi-replica workload Unit with a PDB-coverage finding (dry-run unless --commit)",
		Long: `annotate runs the availability analysis and writes a workload.confighub.com/
pdb-coverage annotation onto each multi-replica workload Unit that has no matching
PodDisruptionBudget. Paired with the workload-pdb-coverage Trigger from
'guardrails install', this turns the cross-Unit coverage finding — the one a
per-Unit Trigger can't compute — into an advisory ApplyWarning.

Re-run after adding PDBs. Dry run unless --commit --change-desc.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			changeDesc, dryRun, err := commit.Validate("annotate workload Units with PDB-coverage findings")
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
			for _, r := range workload.AnalyzeAvailability(snap.Clusters) {
				if r.HasPDB || r.SpaceID == "" || r.UnitSlug == "" {
					continue // covered, or no Unit to annotate
				}
				if clusterFilter != "" && r.Cluster != clusterFilter {
					continue
				}
				ar := annotateResult{Cluster: r.Cluster, Namespace: r.Namespace, Unit: r.UnitSlug, Finding: "uncovered"}
				if !dryRun {
					err := guardrails.Annotate(cmd.Context(), client, r.SpaceID, r.UnitSlug, pdbCoverageAnnotation, ar.Finding, changeDesc)
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
				fprintln(out, fmt.Sprintf("%s/%s  %s  (finding: %s)", r.Cluster, r.Namespace, r.Unit, r.Finding))
			}
			if dryRun {
				fprintln(out, fmt.Sprintf("\nDry run — %d workload Unit(s) would be annotated. Re-run with --commit --change-desc \"…\".", len(results)))
			} else {
				fprintln(out, fmt.Sprintf("\nAnnotated %d workload Unit(s).", len(results)))
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

// annotateUnit sets the PDB-coverage finding annotation on one Unit via the
// set-annotation function (a committed Unit-data mutation).
