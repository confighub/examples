// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/confighub/examples/workload-manager/internal/cub"
	"github.com/confighub/examples/workload-manager/internal/snapshot"
	"github.com/confighub/examples/workload-manager/internal/workload"
)

type readinessReport struct {
	Workloads []workload.WorkloadScore `json:"workloads"`
	Totals    struct {
		Workloads int `json:"workloads"`
		Pass      int `json:"pass"`
		Warn      int `json:"warn"`
		Fail      int `json:"fail"`
	} `json:"totals"`
	Dimensions []string `json:"dimensions"`
	Filter     string   `json:"filter,omitempty"`
}

func newReadinessCmd() *cobra.Command {
	var output string
	var filter filterFlags
	var dimension, clusterFilter, namespaceFilter string
	var failingOnly bool
	cmd := &cobra.Command{
		Use:   "readiness",
		Short: "Per-workload production-readiness scorecard: security / resources / probes / hygiene",
		Long: `readiness scores every workload in the fleet across the readiness dimensions:

  - security  : runAsNonRoot, not privileged, no privilege escalation, readonly
                root fs, drop ALL capabilities, seccomp RuntimeDefault, and
                automountServiceAccountToken: false
  - resources : cpu+memory requests and limits set (missing memory limit fails)
  - probes    : readiness (fail) and liveness (warn) probes on long-running
                controllers
  - hygiene   : terminationMessagePolicy: FallbackToLogsOnError
  - availability : multi-replica workloads have a matching PodDisruptionBudget
                   (a cross-Unit join), the PDB doesn't block all evictions, and
                   pod anti-affinity / topology spread is present

Each dimension is scored pass / warn / fail; the workload's overall status is the
worst of its dimensions.

Restrict to one dimension with --dimension, and to a cluster / namespace with
--cluster / --namespace. Use --failing-only to show just warn+fail workloads.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			var dims []string
			if dimension != "" {
				if !isKnownDimension(dimension) {
					return fmt.Errorf("unknown --dimension %q (want one of: %s)",
						dimension, strings.Join(workload.AllDimensions, ", "))
				}
				dims = []string{dimension}
			}
			client, err := cub.Preflight(cmd.Context())
			if err != nil {
				return err
			}
			snap, err := snapshot.Load(cmd.Context(), client, filter.predicate())
			if err != nil {
				return err
			}
			report := buildReadinessReport(snap, dims, clusterFilter, namespaceFilter, failingOnly)
			if output == outputTable {
				printReadinessTable(cmd, report)
				return nil
			}
			return printJSON(cmd.OutOrStdout(), report)
		},
	}
	addOutputFlag(cmd, &output)
	addFilterFlags(cmd, &filter)
	cmd.Flags().StringVar(&dimension, "dimension", "", "score only one dimension: security | resources | probes | hygiene | availability")
	cmd.Flags().StringVar(&clusterFilter, "cluster", "", "restrict output to this cluster (Target or Space slug)")
	cmd.Flags().StringVar(&namespaceFilter, "namespace", "", "filter by namespace")
	cmd.Flags().BoolVar(&failingOnly, "failing-only", false, "only workloads with a warn or fail dimension")
	return cmd
}

func isKnownDimension(d string) bool {
	for _, k := range workload.AllDimensions {
		if k == d {
			return true
		}
	}
	return false
}

func buildReadinessReport(snap *snapshot.Snapshot, dims []string, clusterFilter, namespaceFilter string, failingOnly bool) readinessReport {
	var report readinessReport
	if len(dims) == 0 {
		report.Dimensions = workload.AllDimensions
	} else {
		report.Dimensions = dims
	}
	report.Filter = snap.Filter
	for _, s := range workload.ScoreFleet(snap.Clusters, dims) {
		if clusterFilter != "" && s.Cluster != clusterFilter {
			continue
		}
		if namespaceFilter != "" && s.Namespace != namespaceFilter {
			continue
		}
		report.Totals.Workloads++
		switch s.Overall {
		case workload.StatusPass:
			report.Totals.Pass++
		case workload.StatusWarn:
			report.Totals.Warn++
		case workload.StatusFail:
			report.Totals.Fail++
		}
		if failingOnly && s.Overall == workload.StatusPass {
			continue
		}
		report.Workloads = append(report.Workloads, s)
	}
	return report
}

func printReadinessTable(cmd *cobra.Command, r readinessReport) {
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
	header := []string{"CLUSTER", "NAMESPACE", "KIND", "NAME"}
	for _, d := range r.Dimensions {
		header = append(header, strings.ToUpper(d))
	}
	header = append(header, "OVERALL")
	fmt.Fprintln(tw, strings.Join(header, "\t"))
	for _, s := range r.Workloads {
		byDim := map[string]workload.Status{}
		for _, d := range s.Dimensions {
			byDim[d.Dimension] = d.Status
		}
		ns := s.Namespace
		if ns == "" {
			ns = "-"
		}
		cells := []string{s.Cluster, ns, s.Kind, s.Name}
		for _, d := range r.Dimensions {
			if st, ok := byDim[d]; ok {
				cells = append(cells, string(st))
			} else {
				cells = append(cells, "-")
			}
		}
		cells = append(cells, string(s.Overall))
		fmt.Fprintln(tw, strings.Join(cells, "\t"))
	}
	_ = tw.Flush()
	fprintln(cmd.OutOrStdout(), fmt.Sprintf(
		"\n%d workloads (%d pass, %d warn, %d fail)",
		r.Totals.Workloads, r.Totals.Pass, r.Totals.Warn, r.Totals.Fail))

	// In table mode, surface the issue detail for warn+fail workloads.
	for _, s := range r.Workloads {
		if s.Overall == workload.StatusPass {
			continue
		}
		var lines []string
		for _, d := range s.Dimensions {
			for _, issue := range d.Issues {
				lines = append(lines, fmt.Sprintf("    [%s] %s", d.Dimension, issue.Message))
			}
		}
		if len(lines) == 0 {
			continue
		}
		sort.Strings(lines)
		fprintln(cmd.OutOrStdout(), fmt.Sprintf("  %s/%s %s/%s:", s.Cluster, nsOrDash(s.Namespace), s.Kind, s.Name))
		for _, l := range lines {
			fprintln(cmd.OutOrStdout(), l)
		}
	}
}

func nsOrDash(ns string) string {
	if ns == "" {
		return "-"
	}
	return ns
}
