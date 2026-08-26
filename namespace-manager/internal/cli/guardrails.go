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
	"github.com/confighub/examples/namespace-manager/internal/nsmanager"
	"github.com/confighub/examples/namespace-manager/internal/snapshot"
	"github.com/confighub/sdk/cliutil"
	"strings"

	"github.com/spf13/cobra"

	"github.com/confighub/sdk/core/cubapi"

	"github.com/confighub/examples/managerkit/guardrails"
	"github.com/confighub/examples/namespace-manager/internal/cub"
)

// findingAnnotation is written by `guardrails annotate` onto the Namespace Unit
// of each incomplete namespace, and read by the envelope-finding rule
// (annotate-then-validate).
const findingAnnotation = "namespace.confighub.com/finding"

// Guardrail CEL expressions, evaluated per resource (r aliases the resource).
const (
	// A Namespace must carry the pod-security enforce label.
	celHasPodSecurity = "r.kind != 'Namespace' || (has(r.metadata.labels) && 'pod-security.kubernetes.io/enforce' in r.metadata.labels)"
	// Annotate-then-validate: warn while an envelope finding annotation is present.
	// The null check is required: a resource written with a bare `annotations:` key
	// has the key present with a null value, so has() is true but `in` against null
	// raises "no such overload" and the rule reports a spurious failure.
	celNoEnvelopeFinding = "!has(r.metadata.annotations) || r.metadata.annotations == null || !('" + findingAnnotation + "' in r.metadata.annotations)"
)

var pack = guardrails.Pack{
	Label: "namespace-guardrails",
	Rules: []guardrails.Rule{
		{Slug: "namespace-has-pod-security", Description: "Warns on a Namespace with no pod-security.kubernetes.io/enforce label. Fix: `cub-namespace apply-envelope --space <s>`.", Expression: celHasPodSecurity},
		{Slug: "namespace-envelope-finding", Description: "Warns while a namespace.confighub.com/finding annotation is present (set by `cub-namespace guardrails annotate`). Fix: close the envelope gap, then re-run annotate.", Expression: celNoEnvelopeFinding},
	},
}

func preflight(ctx context.Context) (*cubapi.Client, error) { return cub.Preflight(ctx) }

func newGuardrailsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "guardrails",
		Short: "Install, inspect, and feed the namespace-envelope guardrail policy pack",
		Long: `guardrails manages a pack of namespace-governance validation policies, defined
once in a policy Space and enforced fleet-wide via a shared Trigger Filter:

  namespace-has-pod-security   a Namespace must carry a pod-security enforce label
  namespace-envelope-finding   warns while an envelope finding is annotated (§ annotate)

Triggers are created with Warn=true (advisory ApplyWarnings, never blocking).
Promote one to blocking later with:
  cub trigger update <slug> --space <policy-space> --unwarn

The envelope-finding rule realizes the annotate-then-validate model: the manager
can't set ApplyWarnings directly (only a failed Trigger can), so 'guardrails
annotate' writes a finding annotation onto each incomplete namespace's Namespace
Unit and this Trigger turns it into a warning.

The namespace-name invariant (metadata.namespace == normalizeName(Component)) is
enforced separately, by a cluster-selected mutating set-namespace Trigger — the
promotable active-correction option, wired outside this advisory pack.`,
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
		Short: "Write a finding annotation onto the Namespace Unit of each incomplete namespace (dry-run unless --commit)",
		Long: `annotate runs the envelope analysis and writes a namespace.confighub.com/finding
annotation (value = the missing members) onto the v1/Namespace Unit of each
incomplete namespace. Paired with the namespace-envelope-finding Trigger from
'guardrails install', this turns a set-aware finding into an advisory
ApplyWarning — the manager cannot set warnings directly, only a failed Trigger
can.

Only namespaces that have a Namespace Unit are annotated (there is nowhere else
to put the annotation). Re-run after fixing config. Dry run unless
--commit --change-desc.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			changeDesc, dryRun, err := commit.Validate("annotate Namespace Units with envelope findings")
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
			for _, c := range snap.Clusters {
				if clusterFilter != "" && c.Cluster != clusterFilter {
					continue
				}
				for _, e := range nsmanager.AnalyzeCluster(c) {
					if e.Complete || e.UnitSlug == "" || e.SpaceID == "" {
						continue // complete, or no Namespace Unit to annotate
					}
					r := annotateResult{Cluster: e.Cluster, Namespace: e.Namespace, Unit: e.UnitSlug, Finding: strings.Join(e.Missing, ",")}
					if !dryRun {
						err := guardrails.Annotate(cmd.Context(), client, e.SpaceID, e.UnitSlug, findingAnnotation, r.Finding, changeDesc)
						r.OK = err == nil
						if err != nil {
							r.Error = err.Error()
						}
					}
					results = append(results, r)
				}
			}
			sort.Slice(results, func(i, j int) bool {
				if results[i].Cluster != results[j].Cluster {
					return results[i].Cluster < results[j].Cluster
				}
				return results[i].Namespace < results[j].Namespace
			})

			if output == outputJSON {
				return printJSON(cmd.OutOrStdout(), results)
			}
			out := cmd.OutOrStdout()
			for _, r := range results {
				fprintln(out, fmt.Sprintf("%s/%s  %s  (finding: %s)", r.Cluster, r.Namespace, r.Unit, r.Finding))
			}
			if dryRun {
				fprintln(out, fmt.Sprintf("\nDry run — %d Namespace Unit(s) would be annotated. Re-run with --commit --change-desc \"…\".", len(results)))
			} else {
				fprintln(out, fmt.Sprintf("\nAnnotated %d Namespace Unit(s).", len(results)))
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

// annotateUnit sets the finding annotation on one Unit via the set-annotation
// function (a committed Unit-data mutation).
