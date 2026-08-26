// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

// This tool's guardrail pack. The mechanism -- policy Space, Triggers, shared
// Filter, wiring each in-scope Space, and reporting the Units a Trigger marked
// -- is managerkit/guardrails'. What is this tool's is the rules, and which
// Units the annotate command has something to say about.

import (
	"context"
	"strings"

	"github.com/spf13/cobra"

	"github.com/confighub/sdk/core/cubapi"

	"github.com/confighub/examples/managerkit/guardrails"
	"github.com/confighub/examples/namespace-manager/internal/cub"
	"github.com/confighub/examples/namespace-manager/internal/nsmanager"
	"github.com/confighub/examples/namespace-manager/internal/snapshot"
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
	cmd.AddCommand(pack.InstallCmd(preflight), pack.StatusCmd(preflight), pack.AnnotateCmd(preflight, annotateSpec))
	return cmd
}

// annotateSpec is the namespace half of `guardrails annotate`: which namespaces
// have an incomplete envelope, and what to write on their Namespace Unit. The
// rest of the command is managerkit/guardrails'.
var annotateSpec = guardrails.AnnotateSpec{
	Key:   findingAnnotation,
	Noun:  "Namespace Unit",
	Short: "Write a finding annotation onto the Namespace Unit of each incomplete namespace (dry-run unless --commit)",
	Long: `annotate runs the envelope analysis and writes a namespace.confighub.com/finding
annotation (value = the missing members) onto the v1/Namespace Unit of each
incomplete namespace. Paired with the namespace-envelope-finding rule from
'guardrails install', this turns a set-aware finding into an advisory
ApplyWarning — the manager cannot set warnings directly, only a failed rule can.

Only namespaces that have a Namespace Unit are annotated (there is nowhere else
to put the annotation). Re-run after fixing config. Dry run unless --commit
--change-desc.`,
	Targets: func(ctx context.Context, c *cubapi.Client, where, cluster string) ([]guardrails.Target, error) {
		snap, err := snapshot.Load(ctx, c, where)
		if err != nil {
			return nil, err
		}
		var out []guardrails.Target
		for _, cl := range snap.Clusters {
			if cluster != "" && cl.Cluster != cluster {
				continue
			}
			for _, e := range nsmanager.AnalyzeCluster(cl) {
				if e.Complete || e.UnitSlug == "" || e.SpaceID == "" {
					continue // complete, or no Namespace Unit to annotate
				}
				out = append(out, guardrails.Target{
					SpaceID: e.SpaceID, UnitSlug: e.UnitSlug,
					Value:   strings.Join(e.Missing, ","),
					Cluster: e.Cluster, Namespace: e.Namespace,
				})
			}
		}
		return out, nil
	},
}
