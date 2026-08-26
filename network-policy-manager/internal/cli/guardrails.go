// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

// The NetworkPolicy guardrail pack. The mechanism -- policy Space, Triggers,
// shared Filter, wiring each in-scope Space, and reporting the Units a Trigger
// marked -- is managerkit/guardrails'. What is netpol's is the rules, and the
// annotate command, which turns a coverage finding no single resource can
// express into an annotation the pack's own rule then warns on.

import (
	"context"
	"strings"

	"github.com/spf13/cobra"

	"github.com/confighub/sdk/core/cubapi"

	"github.com/confighub/examples/managerkit/guardrails"
	"github.com/confighub/examples/network-policy-manager/internal/cub"
	"github.com/confighub/examples/network-policy-manager/internal/netpol"
	"github.com/confighub/examples/network-policy-manager/internal/snapshot"
)

// findingAnnotation is written by `guardrails annotate` onto Units the analyzers
// flag, and read by the coverage rule (annotate-then-validate).
const findingAnnotation = "netpol.confighub.com/finding"

// Guardrail CEL expressions, evaluated per resource (r aliases the resource).
const (
	// Every ingress rule must name its sources — an empty `from` admits all.
	celNoAllowAllIngress = "r.kind != 'NetworkPolicy' || !has(r.spec.ingress) || r.spec.ingress.all(rule, has(rule.from) && size(rule.from) > 0)"
	// No egress rule may permit the whole internet via 0.0.0.0/0.
	celNoWideCidrEgress = "r.kind != 'NetworkPolicy' || !has(r.spec.egress) || r.spec.egress.all(rule, !has(rule.to) || rule.to.all(peer, !has(peer.ipBlock) || peer.ipBlock.cidr != '0.0.0.0/0'))"
	// Annotate-then-validate: warn while a finding annotation is present. The
	// null check is required: a resource written with a bare `annotations:` key
	// has the key present with a null value, so has() is true but `in` against
	// null raises "no such overload" and the rule reports a spurious failure.
	celNoCoverageFinding = "!has(r.metadata.annotations) || r.metadata.annotations == null || !('" + findingAnnotation + "' in r.metadata.annotations)"
)

var pack = guardrails.Pack{
	Label: "netpol-guardrails",
	Rules: []guardrails.Rule{
		{Slug: "netpol-no-allow-all-ingress", Description: "Warns on a NetworkPolicy ingress rule with an empty `from` (admits all sources). Fix: enumerate the allowed sources.", Expression: celNoAllowAllIngress},
		{Slug: "netpol-no-wide-cidr-egress", Description: "Warns on an egress rule permitting 0.0.0.0/0. Fix: restrict the CIDR and exclude the cloud-metadata IP.", Expression: celNoWideCidrEgress},
		{Slug: "netpol-coverage-finding", Description: "Warns while a netpol.confighub.com/finding annotation is present (set by `cub-netpol guardrails annotate`). Fix: close the coverage gap, then re-run annotate.", Expression: celNoCoverageFinding},
	},
}

func preflight(ctx context.Context) (*cubapi.Client, error) { return cub.Preflight(ctx) }

func newGuardrailsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "guardrails",
		Short: "Install, inspect, and feed the NetworkPolicy guardrail policy pack",
		Long: `guardrails manages a pack of NetworkPolicy validation policies, defined once in a
policy Space and enforced fleet-wide via a shared Trigger Filter:

  netpol-no-allow-all-ingress  ingress rules must name their sources
  netpol-no-wide-cidr-egress   no egress to 0.0.0.0/0
  netpol-coverage-finding      warns while a coverage finding is annotated (§ annotate)

Triggers are created with Warn=true (advisory ApplyWarnings, never blocking).
Promote one to blocking later with:
  cub trigger update <slug> --space <policy-space> --unwarn

The coverage rule realizes the annotate-then-validate model: the manager can't
set ApplyWarnings directly (only a failed Trigger can), so 'guardrails annotate'
writes a finding annotation onto flagged Units and this Trigger turns it into a
warning.`,
	}
	cmd.AddCommand(pack.InstallCmd(preflight), pack.StatusCmd(preflight), pack.AnnotateCmd(preflight, annotateSpec))
	return cmd
}

// annotateSpec is the netpol half of `guardrails annotate`: which Units an
// analyzer flagged, and what to write on them. The rest of the command is
// managerkit/guardrails'.
//
// Several analyzers can flag one Unit, so the findings are grouped: one
// annotation per Unit naming every analyzer that objected, rather than each
// overwriting the last.
var annotateSpec = guardrails.AnnotateSpec{
	Key:   findingAnnotation,
	Short: "Write a finding annotation onto each Unit the analyzers flag (dry-run unless --commit)",
	Long: `annotate runs the findings analyzers and writes a netpol.confighub.com/finding
annotation onto each flagged Unit (those with a resource-level finding, e.g.
uncovered-ingress or allow-all). Paired with the netpol-coverage-finding rule
from 'guardrails install', this turns a finding into an advisory ApplyWarning —
the manager cannot set warnings directly, only a failed rule can.

Re-run after fixing config to clear stale annotations (a fixed Unit produces no
finding, so its annotation is removed). Dry run unless --commit --change-desc.`,
	Targets: func(ctx context.Context, c *cubapi.Client, where, cluster string) ([]guardrails.Target, error) {
		snap, err := snapshot.Load(ctx, c, where)
		if err != nil {
			return nil, err
		}
		byUnit := map[string]*guardrails.Target{}
		var order []string
		for clusterName, cl := range snap.Clusters {
			if cluster != "" && clusterName != cluster {
				continue
			}
			for _, f := range netpol.AnalyzeFindings(cl) {
				if f.Origin.UnitID == "" || f.Origin.SpaceID == "" {
					continue // namespace-level finding: no single Unit to annotate
				}
				key := f.Origin.SpaceID + "/" + f.Origin.UnitSlug
				t, ok := byUnit[key]
				if !ok {
					t = &guardrails.Target{
						SpaceID: f.Origin.SpaceID, UnitSlug: f.Origin.UnitSlug,
						Space: f.Origin.Space, Cluster: f.Origin.Cluster,
					}
					byUnit[key] = t
					order = append(order, key)
				}
				if !strings.Contains(t.Value, f.Analyzer) {
					if t.Value != "" {
						t.Value += ","
					}
					t.Value += f.Analyzer
				}
			}
		}
		out := make([]guardrails.Target, 0, len(order))
		for _, key := range order {
			out = append(out, *byUnit[key])
		}
		return out, nil
	},
}
