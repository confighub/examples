// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/confighub/sdk/cliutil"
	"github.com/confighub/sdk/core/cubapi"
	api "github.com/confighub/sdk/core/function/api"
	"github.com/confighub/sdk/managerkit"
	"github.com/confighub/sdk/managerkit/write"

	"github.com/confighub/examples/namespace-manager/internal/cub"
	"github.com/confighub/examples/namespace-manager/internal/nsmanager"
)

type applyEnvelopeReport struct {
	Level      string                       `json:"level"`
	Version    string                       `json:"version"`
	Namespaces []nsmanager.PodSecurityApply `json:"namespaces"`
	// Edits are the labels each Unit gets.
	Edits  []nsmanager.UnitLabelEdits `json:"edits"`
	Result write.Report               `json:"result"`
}

func newApplyEnvelopeCmd() *cobra.Command {
	var output, clusterFilter, namespaceFilter, level, version string
	var spaces []string
	var bases bool
	var filter filterFlags
	var commit cliutil.CommitFlags
	cmd := &cobra.Command{
		Use:   "apply-envelope (--space <space> | --cluster <cluster> | --namespace <ns>)",
		Short: "Set each namespace's Pod Security Standard to the most restrictive level its workloads meet",
		Long: `apply-envelope labels each selected namespace with the most restrictive Pod
Security Standard level all of its workloads meet -- the ACHIEVABLE column of
'cub-namespace pod-security' -- or with --level, a level you choose:

  pod-security.kubernetes.io/enforce          the level
  pod-security.kubernetes.io/enforce-version  --version (default latest)
  pod-security.kubernetes.io/warn             restricted, below restricted, unless a warn level is set

The Namespace label is what admission enforces. The enforce labels also go on
each of the namespace's workloads, wherever its Unit is, so that the
namespace-pod-security-standard guardrail ('cub-namespace guardrails install')
can hold each workload to its namespace's level in ConfigHub, before it reaches
a cluster. Labeling a workload does not touch its pod template, so nothing
restarts.

A typical loop: 'pod-security' to see each namespace's achievable level and
what blocks the next one; 'cub-workload harden' (or other fixes) on the
blockers; 'pod-security' again to confirm; then apply-envelope.

apply-envelope never lowers a namespace's level. A Namespace that already
enforces the level, or a stricter one, keeps its labels, version included
("keep"), and its workloads get copies -- so where a stricter label is already
in place, the guardrail flags the workloads it rejects. A namespace is left
alone, with the reason, when the cluster owns it (kube-system, default, ...),
when some of its workloads could not be checked, when it has no workloads in
ConfigHub and no --level was given, or when its workloads do not meet --level.

Select namespaces by the Space holding their Namespace Unit (--space), --cluster
or --namespace. With --bases it labels base Spaces instead, each judged by its
own workloads (and listed under its slug as the cluster); the variants made from
a base pick the labels up when they are upgraded from it.

The server-side filters scope which Units are read; narrowing them can hide
workloads that run in a namespace from other Spaces.
The Units are edited, not published — rolling them out is a separate
'cub release publish'. Dry-run unless --commit --change-desc; the dry run shows
each Unit's diff.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(spaces) == 0 && clusterFilter == "" && namespaceFilter == "" {
				return fmt.Errorf("select the namespaces with --space, --cluster or --namespace")
			}
			switch level {
			case "", nsmanager.LevelPrivileged, nsmanager.LevelBaseline, nsmanager.LevelRestricted:
			default:
				return fmt.Errorf("--level must be privileged, baseline or restricted")
			}
			if version == "" {
				version = "latest"
			}
			changeDesc, dryRun, err := commit.Validate("apply the Pod Security Standard level their workloads meet to " + describeSelection(spaces, clusterFilter, namespaceFilter))
			if err != nil {
				return err
			}
			client, err := cub.Preflight(cmd.Context())
			if err != nil {
				return err
			}
			where, err := filter.Predicate()
			if err != nil {
				return err
			}
			fleet, err := loadPodSecurity(cmd.Context(), client, where, version, bases)
			if err != nil {
				return err
			}
			selected := func(n *nsmanager.NamespaceEntity) bool {
				return (len(spaces) == 0 || slices.Contains(spaces, n.Origin.Space)) &&
					(clusterFilter == "" || n.Origin.Cluster == clusterFilter) &&
					(namespaceFilter == "" || n.Name == namespaceFilter)
			}
			plans, edits := nsmanager.PlanPodSecurity(fleet.clusters, fleet.postures, level, version, selected)

			ch := write.Change(changeDesc, dryRun)
			var results []*cubapi.Result
			for _, u := range edits {
				sel := cubapi.Selector{Where: cubapi.Where{}.Eq("UnitID", u.UnitID).String()}
				res, err := cub.InvokeMutations(cmd.Context(), client, labelInvocations(u.Edits), sel, ch)
				if err != nil {
					return fmt.Errorf("label %s/%s: %w", u.Space, u.UnitSlug, err)
				}
				results = append(results, res)
			}

			report := applyEnvelopeReport{
				Level: level, Version: version, Namespaces: plans, Edits: edits,
				Result: write.Summarize("apply-envelope", "", dryRun, results...),
			}
			if report.Level == "" {
				report.Level = "achievable"
			}
			if done, err := managerkit.Render(cmd.OutOrStdout(), output, report); done || err != nil {
				return err
			}
			printApplyEnvelope(cmd, report, dryRun)
			return nil
		},
	}
	addOutputFlag(cmd, &output)
	addFilterFlags(cmd, &filter)
	commit.Bind(cmd)
	cmd.Flags().StringSliceVar(&spaces, "space", nil, "label the namespaces whose Namespace Unit is in this Space (repeat, or comma-separate, for several)")
	cmd.Flags().BoolVar(&bases, "bases", false, "label base (Variant=base) Spaces instead of deployed ones, each judged by its own workloads; variants then pick the labels up by upgrade")
	cmd.Flags().StringVar(&clusterFilter, "cluster", "", "label the namespaces on this cluster (Target slug, or None for Units whose Space has no release Target)")
	cmd.Flags().StringVar(&namespaceFilter, "namespace", "", "label this namespace")
	cmd.Flags().StringVar(&level, "level", "", "level to apply: privileged | baseline | restricted (default: the most restrictive one each namespace's workloads meet)")
	cmd.Flags().StringVar(&version, "version", "latest", `Pod Security Standards version to check against and label: "latest" or a Kubernetes minor version such as "v1.30"`)
	return cmd
}

// labelInvocations turns label edits into set-label invocations, each on the
// resources its WhereResource selects.
func labelInvocations(edits []nsmanager.LabelEdit) []api.FunctionInvocation {
	invocations := make([]api.FunctionInvocation, 0, len(edits))
	for _, e := range edits {
		invocations = append(invocations, api.FunctionInvocation{
			FunctionName: "set-label",
			Arguments: []api.FunctionArgument{
				{ParameterName: "label-key", Value: e.Key},
				{ParameterName: "label-value", Value: e.Value},
			},
			WhereResource: e.WhereResource,
		})
	}
	return invocations
}

func describeSelection(spaces []string, cluster, namespace string) string {
	var parts []string
	if namespace != "" {
		parts = append(parts, "namespace "+namespace)
	}
	if cluster != "" {
		parts = append(parts, "cluster "+cluster)
	}
	if len(spaces) > 0 {
		parts = append(parts, "the Namespace Units in "+strings.Join(spaces, ", "))
	}
	return strings.Join(parts, " on ")
}

func printApplyEnvelope(cmd *cobra.Command, r applyEnvelopeReport, dryRun bool) {
	out := cmd.OutOrStdout()
	if len(r.Namespaces) == 0 {
		fprintln(out, "No Namespace Units match the selection.")
		return
	}
	tw := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "CLUSTER\tNAMESPACE\tWORKLOADS\tENFORCE\tACHIEVABLE\tAPPLY")
	for _, p := range r.Namespaces {
		apply := p.Level
		switch {
		case p.Skip != "":
			apply = "skip: " + p.Skip
		case p.Kept:
			apply = "keep " + p.Level
		}
		fmt.Fprintf(tw, "%s\t%s\t%d\t%s\t%s\t%s\n", p.Cluster, p.Namespace, p.Workloads, dash(p.Enforce), dash(p.Achievable), apply)
	}
	_ = tw.Flush()
	for _, p := range r.Namespaces {
		if p.Note != "" {
			fprintln(out, fmt.Sprintf("  %s/%s: %s", p.Cluster, p.Namespace, p.Note))
		}
	}
	verb := "changed"
	if dryRun {
		verb = "would change"
	}
	fprintln(out, fmt.Sprintf("\napply-envelope: %d of %d Unit(s) %s%s",
		r.Result.Mutated, len(r.Edits), verb, write.DryRunSuffix(dryRun)))
	if !dryRun && r.Result.Committed > 0 {
		fprintln(out, "Not published — rolling out is a separate `cub release publish`.")
	}
	write.WriteDiffs(out, "apply-envelope", r.Result)
}
