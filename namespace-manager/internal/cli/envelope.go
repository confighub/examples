// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/confighub/examples/namespace-manager/internal/cub"
	"github.com/confighub/examples/namespace-manager/internal/nsmanager"
	"github.com/confighub/examples/namespace-manager/internal/snapshot"
)

type envelopeReport struct {
	Namespaces []nsmanager.NamespaceEnvelope  `json:"namespaces"`
	Duplicates []nsmanager.DuplicateNamespace `json:"duplicates,omitempty"`
	Totals     struct {
		Namespaces int `json:"namespaces"`
		Complete   int `json:"complete"`
		Incomplete int `json:"incomplete"`
		Duplicates int `json:"duplicates"`
	} `json:"totals"`
}

func newEnvelopeCmd() *cobra.Command {
	var output string
	var filter filterFlags
	var clusterFilter, namespaceFilter string
	var incompleteOnly bool
	cmd := &cobra.Command{
		Use:   "envelope",
		Short: "Per-namespace envelope completeness: which namespaces lack pod-security / default-deny / baseline RBAC",
		Long: `envelope reports, for every namespace in the fleet, which members of the policy
envelope are present and which are missing:

  - namespace-object : a v1/Namespace object exists for the namespace
  - pod-security     : the Namespace carries a pod-security.kubernetes.io/enforce label
  - default-deny     : a namespace-wide default-deny NetworkPolicy (empty podSelector) exists
  - baseline-rbac    : a RoleBinding exists in the namespace

This is the fleet-wide read a per-resource validator or a runtime tenancy
controller (Capsule, HNC) cannot do: envelope completeness is a property of the
whole set of resources in a namespace, joined across types.

It also flags duplicate Namespace objects that resolve to the same name on the
same Target (would collide in one cluster); base Units still carrying the
'confighubplaceholder' name are exempt (vet-placeholders gates them).

Filter with --cluster, --namespace, and --incomplete-only.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cub.Preflight(cmd.Context())
			if err != nil {
				return err
			}
			snap, err := snapshot.Load(cmd.Context(), client, filter.predicate())
			if err != nil {
				return err
			}
			report := buildEnvelopeReport(snap, clusterFilter, namespaceFilter, incompleteOnly)
			if output == outputTable {
				printEnvelopeTable(cmd, report)
				return nil
			}
			return printJSON(cmd.OutOrStdout(), report)
		},
	}
	addOutputFlag(cmd, &output)
	addFilterFlags(cmd, &filter)
	cmd.Flags().StringVar(&clusterFilter, "cluster", "", "restrict output to this cluster (Target or Space slug)")
	cmd.Flags().StringVar(&namespaceFilter, "namespace", "", "filter by namespace")
	cmd.Flags().BoolVar(&incompleteOnly, "incomplete-only", false, "only namespaces missing one or more envelope members")
	return cmd
}

func buildEnvelopeReport(snap *snapshot.Snapshot, clusterFilter, namespaceFilter string, incompleteOnly bool) envelopeReport {
	var report envelopeReport
	for _, e := range nsmanager.AnalyzeFleet(snap.Clusters) {
		if clusterFilter != "" && e.Cluster != clusterFilter {
			continue
		}
		if namespaceFilter != "" && e.Namespace != namespaceFilter {
			continue
		}
		report.Totals.Namespaces++
		if e.Complete {
			report.Totals.Complete++
		} else {
			report.Totals.Incomplete++
		}
		if incompleteOnly && e.Complete {
			continue
		}
		report.Namespaces = append(report.Namespaces, e)
	}
	for _, d := range nsmanager.DuplicateNamespaces(snap.Clusters) {
		if clusterFilter != "" && d.Target != clusterFilter {
			continue
		}
		if namespaceFilter != "" && d.Namespace != namespaceFilter {
			continue
		}
		report.Duplicates = append(report.Duplicates, d)
	}
	report.Totals.Duplicates = len(report.Duplicates)
	return report
}

func printEnvelopeTable(cmd *cobra.Command, r envelopeReport) {
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "CLUSTER\tNAMESPACE\tWORKLOADS\tPOD-SECURITY\tDEFAULT-DENY\tBASELINE-RBAC\tMISSING")
	for _, e := range r.Namespaces {
		ps := e.PodSecurityEnforce
		if ps == "" {
			ps = "-"
		}
		missing := "-"
		if len(e.Missing) > 0 {
			missing = strings.Join(e.Missing, ",")
		}
		fmt.Fprintf(tw, "%s\t%s\t%d\t%s\t%s\t%s\t%s\n",
			e.Cluster, e.Namespace, e.WorkloadCount, ps,
			yesNo(e.HasDefaultDeny), yesNo(e.HasBaselineRBAC), missing)
	}
	_ = tw.Flush()
	fprintln(cmd.OutOrStdout(), fmt.Sprintf(
		"\n%d namespaces (%d complete, %d incomplete), %d duplicate-namespace collisions",
		r.Totals.Namespaces, r.Totals.Complete, r.Totals.Incomplete, r.Totals.Duplicates))
	for _, d := range r.Duplicates {
		fprintln(cmd.OutOrStdout(), fmt.Sprintf(
			"  DUPLICATE: namespace %q appears %d times on target %q (%s)",
			d.Namespace, d.Count, d.Target, strings.Join(d.UnitSlugs, ", ")))
	}
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
