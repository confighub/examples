// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/confighub/examples/workload-manager/internal/cub"
	"github.com/confighub/examples/workload-manager/internal/snapshot"
	"github.com/confighub/examples/workload-manager/internal/workload"
)

type availabilityReport struct {
	Workloads []workload.AvailabilityResult `json:"workloads"`
	Totals    struct {
		MultiReplica int `json:"multiReplica"`
		Uncovered    int `json:"uncovered"`
		BlocksAll    int `json:"blocksAllEvictions"`
		NoSpread     int `json:"noSpread"`
	} `json:"totals"`
	Filter string `json:"filter,omitempty"`
}

func newAvailabilityCmd() *cobra.Command {
	var output string
	var filter filterFlags
	var clusterFilter, namespaceFilter string
	var issuesOnly bool
	cmd := &cobra.Command{
		Use:   "availability",
		Short: "Multi-replica workloads lacking a matching PDB and/or pod anti-affinity/spread",
		Long: `availability reports the disruption-survival posture of every multi-replica
workload in the fleet:

  - PDB coverage : is there a PodDisruptionBudget whose selector matches the
                   workload's pods (a cross-Unit join — the PDB and the workload
                   live in separate Units)?
  - eviction lock: does a matching PDB block all voluntary evictions
                   (minAvailable >= replicas, or maxUnavailable: 0)?
  - spread       : does the workload declare pod anti-affinity or topology spread
                   so a node/zone loss can't take every replica?

This is the fleet-wide read a per-Unit validating Trigger cannot do under
one-resource-per-Unit: it would see a lone workload Unit and can't tell whether a
matching PDB exists in some other Unit.

Single-replica workloads and DaemonSet / Job / CronJob / Pod are out of scope
(a PDB gains a single instance nothing). Filter with --cluster / --namespace;
--issues-only hides fully-covered workloads.`,
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
			report := buildAvailabilityReport(snap, clusterFilter, namespaceFilter, issuesOnly)
			if output == outputTable {
				printAvailabilityTable(cmd, report)
				return nil
			}
			return printJSON(cmd.OutOrStdout(), report)
		},
	}
	addOutputFlag(cmd, &output)
	addFilterFlags(cmd, &filter)
	cmd.Flags().StringVar(&clusterFilter, "cluster", "", "restrict output to this cluster (Target or Space slug)")
	cmd.Flags().StringVar(&namespaceFilter, "namespace", "", "filter by namespace")
	cmd.Flags().BoolVar(&issuesOnly, "issues-only", false, "only workloads with an availability issue")
	return cmd
}

func buildAvailabilityReport(snap *snapshot.Snapshot, clusterFilter, namespaceFilter string, issuesOnly bool) availabilityReport {
	var report availabilityReport
	report.Filter = snap.Filter
	for _, r := range workload.AnalyzeAvailability(snap.Clusters) {
		if clusterFilter != "" && r.Cluster != clusterFilter {
			continue
		}
		if namespaceFilter != "" && r.Namespace != namespaceFilter {
			continue
		}
		report.Totals.MultiReplica++
		if !r.HasPDB {
			report.Totals.Uncovered++
		}
		if r.PDBBlocksEviction {
			report.Totals.BlocksAll++
		}
		if !r.HasAntiAffinity && !r.HasTopologySpread {
			report.Totals.NoSpread++
		}
		if issuesOnly && len(r.Issues) == 0 {
			continue
		}
		report.Workloads = append(report.Workloads, r)
	}
	return report
}

func printAvailabilityTable(cmd *cobra.Command, r availabilityReport) {
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "CLUSTER\tNAMESPACE\tKIND\tNAME\tREPLICAS\tPDB\tBLOCKS-EVICT\tSPREAD")
	for _, w := range r.Workloads {
		ns := w.Namespace
		if ns == "" {
			ns = "-"
		}
		replicas := "-"
		if w.Replicas != nil {
			replicas = fmt.Sprintf("%d", *w.Replicas)
		}
		pdb := "MISSING"
		if w.HasPDB {
			pdb = w.PDBName
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			w.Cluster, ns, w.Kind, w.Name, replicas, pdb,
			yesNo(w.PDBBlocksEviction), yesNo(w.HasAntiAffinity || w.HasTopologySpread))
	}
	_ = tw.Flush()
	fprintln(cmd.OutOrStdout(), fmt.Sprintf(
		"\n%d multi-replica workloads: %d uncovered (no PDB), %d block all evictions, %d without spread",
		r.Totals.MultiReplica, r.Totals.Uncovered, r.Totals.BlocksAll, r.Totals.NoSpread))
	for _, w := range r.Workloads {
		if len(w.Issues) == 0 {
			continue
		}
		fprintln(cmd.OutOrStdout(), fmt.Sprintf("  %s/%s %s/%s: %s",
			w.Cluster, nsOrDash(w.Namespace), w.Kind, w.Name, strings.Join(w.Issues, "; ")))
	}
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
