// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/confighub/examples/network-policy-manager/internal/cub"
	"github.com/confighub/examples/network-policy-manager/internal/netpol"
	"github.com/confighub/examples/network-policy-manager/internal/snapshot"
)

type workloadRef struct {
	Cluster   string `json:"cluster"`
	Namespace string `json:"namespace"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
}

func refOf(cluster string, w *netpol.WorkloadEntity) workloadRef {
	return workloadRef{Cluster: cluster, Namespace: netpol.NamespaceOf(w.Namespace), Kind: w.Kind, Name: w.Name}
}

// reachResult is one resolved target workload and its connectivity peers.
type reachResult struct {
	Target    workloadRef   `json:"target"`
	Direction string        `json:"direction"` // "inbound" (who-can-reach) | "outbound" (reachable-from)
	Peers     []workloadRef `json:"peers"`
}

// matchedWorkload pairs a workload with the cluster it belongs to so the
// connectivity queries can be run with the cluster's own policy set.
type matchedWorkload struct {
	cluster  string
	clusterC *netpol.ClusterNetpol
	workload *netpol.WorkloadEntity
}

func newWhoCanReachCmd() *cobra.Command {
	return newReachCmd(
		"who-can-reach",
		"inbound",
		"List workloads allowed to send traffic TO the named workload",
		`who-can-reach resolves the named workload and lists every other workload in
its cluster that the NetworkPolicy set allows to reach it (its effective ingress
sources, intersected with each source's egress).`,
	)
}

func newReachableFromCmd() *cobra.Command {
	return newReachCmd(
		"reachable-from",
		"outbound",
		"List workloads the named workload is allowed to send traffic to",
		`reachable-from resolves the named workload and lists every other workload in
its cluster it is allowed to reach (its effective egress destinations,
intersected with each destination's ingress).`,
	)
}

func newReachCmd(use, direction, short, long string) *cobra.Command {
	var output string
	var scope scopeFlags
	var clusterFilter, namespaceFilter, kindFilter string
	cmd := &cobra.Command{
		Use:   use + " <workload-name>",
		Short: short,
		Long:  long + "\n\nDisambiguate with --cluster, --namespace, and --kind when a name is not unique.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cub.Preflight(cmd.Context())
			if err != nil {
				return err
			}
			snap, err := snapshot.Load(cmd.Context(), client, scope.scope())
			if err != nil {
				return err
			}
			matches := resolveWorkloads(snap, args[0], clusterFilter, namespaceFilter, kindFilter)
			if len(matches) == 0 {
				return fmt.Errorf("no workload named %q matches the given filters", args[0])
			}
			results := make([]reachResult, 0, len(matches))
			for _, m := range matches {
				var peers []*netpol.WorkloadEntity
				if direction == "inbound" {
					peers = netpol.WhoCanReach(m.clusterC, m.workload)
				} else {
					peers = netpol.ReachableFrom(m.clusterC, m.workload)
				}
				rr := reachResult{Target: refOf(m.cluster, m.workload), Direction: direction}
				for _, p := range peers {
					rr.Peers = append(rr.Peers, refOf(m.cluster, p))
				}
				sortRefs(rr.Peers)
				results = append(results, rr)
			}
			if output == outputTable {
				printReachTable(cmd, results)
				return nil
			}
			return printJSON(cmd.OutOrStdout(), results)
		},
	}
	addOutputFlag(cmd, &output)
	addScopeFlags(cmd, &scope)
	cmd.Flags().StringVar(&clusterFilter, "cluster", "", "restrict to this cluster (Target or Space slug)")
	cmd.Flags().StringVar(&namespaceFilter, "namespace", "", "restrict to this namespace")
	cmd.Flags().StringVar(&kindFilter, "kind", "", "restrict to this workload kind (Deployment, StatefulSet, ...)")
	return cmd
}

func resolveWorkloads(snap *snapshot.Snapshot, name, clusterFilter, namespaceFilter, kindFilter string) []matchedWorkload {
	var matches []matchedWorkload
	names := make([]string, 0, len(snap.Clusters))
	for n := range snap.Clusters {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, cn := range names {
		if clusterFilter != "" && cn != clusterFilter {
			continue
		}
		c := snap.Clusters[cn]
		for _, w := range c.Workloads {
			if w.Name != name {
				continue
			}
			if namespaceFilter != "" && netpol.NamespaceOf(w.Namespace) != namespaceFilter {
				continue
			}
			if kindFilter != "" && !strings.EqualFold(w.Kind, kindFilter) {
				continue
			}
			matches = append(matches, matchedWorkload{cluster: cn, clusterC: c, workload: w})
		}
	}
	return matches
}

func printReachTable(cmd *cobra.Command, results []reachResult) {
	out := cmd.OutOrStdout()
	for _, rr := range results {
		verb := "can be reached by"
		if rr.Direction == "outbound" {
			verb = "can reach"
		}
		fprintln(out, fmt.Sprintf("%s %s/%s (%s) %s %d workload(s):",
			rr.Target.Kind, rr.Target.Cluster, rr.Target.Name, rr.Target.Namespace, verb, len(rr.Peers)))
		tw := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
		for _, p := range rr.Peers {
			fmt.Fprintf(tw, "  %s\t%s\t%s\n", p.Namespace, p.Kind, p.Name)
		}
		_ = tw.Flush()
	}
}

func sortRefs(refs []workloadRef) {
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].Namespace != refs[j].Namespace {
			return refs[i].Namespace < refs[j].Namespace
		}
		return refs[i].Name < refs[j].Name
	})
}
