// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/confighub/examples/scheduling-manager/internal/cub"
	"github.com/confighub/examples/scheduling-manager/internal/scheduling"
	"github.com/confighub/examples/scheduling-manager/internal/snapshot"
)

type placementRow struct {
	Cluster        string   `json:"cluster"`
	Namespace      string   `json:"namespace,omitempty"`
	Kind           string   `json:"kind"`
	Name           string   `json:"name"`
	Space          string   `json:"space"`
	UnitSlug       string   `json:"unitSlug"`
	NodeSelector   []string `json:"nodeSelector,omitempty"`
	Tolerations    []string `json:"tolerations,omitempty"`
	NodeAffinity   string   `json:"nodeAffinity"` // none | preferred | required
	Constrained    bool     `json:"constrained"`
}

type placementReport struct {
	Workloads []placementRow `json:"workloads"`
	Totals    struct {
		Workloads     int `json:"workloads"`
		Constrained   int `json:"constrained"`
		Unconstrained int `json:"unconstrained"`
	} `json:"totals"`
	Filter string `json:"filter,omitempty"`
}

func newPlacementCmd() *cobra.Command {
	var output string
	var filter filterFlags
	var clusterFilter, namespaceFilter string
	var unconstrainedOnly bool
	cmd := &cobra.Command{
		Use:   "placement",
		Short: "Per-workload placement: nodeSelector, tolerations, node affinity, and whether it's constrained",
		Long: `placement reports, for every workload, where it is allowed to land:

  - nodeSelector : the node label key=value pins
  - tolerations  : the taint keys it tolerates
  - nodeAffinity : none | preferred | required
  - constrained  : whether it actually restricts nodes (a nodeSelector or a
                   required node affinity) — tolerations alone do NOT constrain
                   placement, they only permit scheduling onto tainted nodes

Filter with --cluster / --namespace; --unconstrained-only shows workloads that
pin nothing.`,
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
			report := buildPlacementReport(snap, clusterFilter, namespaceFilter, unconstrainedOnly)
			if output == outputTable {
				printPlacementTable(cmd, report)
				return nil
			}
			return printJSON(cmd.OutOrStdout(), report)
		},
	}
	addOutputFlag(cmd, &output)
	addFilterFlags(cmd, &filter)
	cmd.Flags().StringVar(&clusterFilter, "cluster", "", "restrict output to this cluster (Target or Space slug)")
	cmd.Flags().StringVar(&namespaceFilter, "namespace", "", "filter by namespace")
	cmd.Flags().BoolVar(&unconstrainedOnly, "unconstrained-only", false, "only workloads that don't constrain placement")
	return cmd
}

func nodeAffinityLabel(w *scheduling.WorkloadPlacement) string {
	switch {
	case w.HasRequiredNodeAffinity:
		return "required"
	case w.HasNodeAffinity:
		return "preferred"
	default:
		return "none"
	}
}

func nodeSelectorPairs(w *scheduling.WorkloadPlacement) []string {
	pairs := make([]string, 0, len(w.NodeSelector))
	for k, v := range w.NodeSelector {
		pairs = append(pairs, k+"="+v)
	}
	sort.Strings(pairs)
	return pairs
}

func buildPlacementReport(snap *snapshot.Snapshot, clusterFilter, namespaceFilter string, unconstrainedOnly bool) placementReport {
	var report placementReport
	report.Filter = snap.Filter
	for _, c := range snap.Clusters {
		for _, w := range c.Workloads {
			if clusterFilter != "" && w.Origin.Cluster != clusterFilter {
				continue
			}
			if namespaceFilter != "" && w.Namespace != namespaceFilter {
				continue
			}
			report.Totals.Workloads++
			if w.Constrained() {
				report.Totals.Constrained++
			} else {
				report.Totals.Unconstrained++
			}
			if unconstrainedOnly && w.Constrained() {
				continue
			}
			report.Workloads = append(report.Workloads, placementRow{
				Cluster:      w.Origin.Cluster,
				Namespace:    w.Namespace,
				Kind:         w.Kind,
				Name:         w.Name,
				Space:        w.Origin.Space,
				UnitSlug:     w.Origin.UnitSlug,
				NodeSelector: nodeSelectorPairs(w),
				Tolerations:  w.TolerationKeys(),
				NodeAffinity: nodeAffinityLabel(w),
				Constrained:  w.Constrained(),
			})
		}
	}
	sort.Slice(report.Workloads, func(i, j int) bool {
		a, b := report.Workloads[i], report.Workloads[j]
		if a.Cluster != b.Cluster {
			return a.Cluster < b.Cluster
		}
		if a.Namespace != b.Namespace {
			return a.Namespace < b.Namespace
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.Name < b.Name
	})
	return report
}

func printPlacementTable(cmd *cobra.Command, r placementReport) {
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "CLUSTER\tNAMESPACE\tKIND\tNAME\tNODESELECTOR\tTOLERATIONS\tNODEAFFINITY\tCONSTRAINED")
	for _, w := range r.Workloads {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			w.Cluster, nsOrDash(w.Namespace), w.Kind, w.Name,
			dashList(w.NodeSelector), dashList(w.Tolerations), w.NodeAffinity, yesNo(w.Constrained))
	}
	_ = tw.Flush()
	fprintln(cmd.OutOrStdout(), fmt.Sprintf("\n%d workloads (%d constrained, %d unconstrained)",
		r.Totals.Workloads, r.Totals.Constrained, r.Totals.Unconstrained))
}

func dashList(s []string) string {
	if len(s) == 0 {
		return "-"
	}
	return strings.Join(s, ",")
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
