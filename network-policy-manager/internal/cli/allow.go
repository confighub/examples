// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/confighub/sdk/cliutil"

	"github.com/confighub/examples/network-policy-manager/internal/cub"
	"github.com/confighub/examples/network-policy-manager/internal/netpol"
	"github.com/confighub/examples/network-policy-manager/internal/snapshot"
)

func newAllowCmd() *cobra.Command {
	var output, clusterFilter, srcNamespace, dstNamespace, port, spaceOverride string
	var egress bool
	var scope scopeFlags
	var commit cliutil.CommitFlags
	cmd := &cobra.Command{
		Use:   "allow <src-workload> <dst-workload>",
		Short: "Create an allow NetworkPolicy Unit between two workloads (dry-run unless --commit)",
		Long: `allow generates a NetworkPolicy that admits traffic from a source workload to a
destination workload and creates it as a ConfigHub Unit. By default it is an
ingress policy in the destination's namespace (selecting the destination,
admitting the source); with --egress it is an egress policy in the source's
namespace instead. Restrict to a port with --port.

Both workloads are resolved by name; disambiguate with --cluster,
--src-namespace, and --dst-namespace. They must live in the same cluster.

This is a dry run unless you pass --commit --change-desc "…".`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			srcName, dstName := args[0], args[1]
			summary := fmt.Sprintf("allow %s -> %s via NetworkPolicy", srcName, dstName)
			changeDesc, dryRun, err := commit.Validate(summary)
			if err != nil {
				return err
			}
			client, err := cub.Preflight(cmd.Context())
			if err != nil {
				return err
			}
			snap, err := snapshot.Load(cmd.Context(), client, scope.scope())
			if err != nil {
				return err
			}
			src, err := uniqueWorkload(resolveWorkloads(snap, srcName, clusterFilter, srcNamespace, ""), "source", srcName)
			if err != nil {
				return err
			}
			dst, err := uniqueWorkload(resolveWorkloads(snap, dstName, clusterFilter, dstNamespace, ""), "destination", dstName)
			if err != nil {
				return err
			}
			if src.cluster != dst.cluster {
				return fmt.Errorf("source %q (cluster %s) and destination %q (cluster %s) are in different clusters; both must be in one cluster",
					srcName, src.cluster, dstName, dst.cluster)
			}
			slug, manifest := netpol.AllowYAML(src.workload, dst.workload, egress, port)

			// The policy lives with the workload it selects: the destination for an
			// ingress policy, the source for an egress policy.
			policyNS := netpol.NamespaceOf(dst.workload.Namespace)
			if egress {
				policyNS = netpol.NamespaceOf(src.workload.Namespace)
			}
			dest, err := resolveCreateDest(snap, policyNS, src.cluster, spaceOverride)
			if err != nil {
				return err
			}
			return runCreate(cmd, client, dest, slug, manifest, policyNS, dryRun, changeDesc, output)
		},
	}
	addOutputFlag(cmd, &output)
	addScopeFlags(cmd, &scope)
	cmd.Flags().StringVar(&clusterFilter, "cluster", "", "cluster (Target or Space slug) to resolve both workloads in")
	cmd.Flags().StringVar(&srcNamespace, "src-namespace", "", "namespace of the source workload (disambiguation)")
	cmd.Flags().StringVar(&dstNamespace, "dst-namespace", "", "namespace of the destination workload (disambiguation)")
	cmd.Flags().StringVar(&port, "port", "", "restrict the rule to this port (numeric or named; protocol TCP)")
	cmd.Flags().BoolVar(&egress, "egress", false, "author an egress policy on the source instead of ingress on the destination")
	cmd.Flags().StringVar(&spaceOverride, "space", "", "Space slug to create the Unit in when ambiguous")
	commit.Bind(cmd)
	return cmd
}

// uniqueWorkload requires exactly one match, with a helpful error otherwise.
func uniqueWorkload(matches []matchedWorkload, role, name string) (matchedWorkload, error) {
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return matchedWorkload{}, fmt.Errorf("no %s workload named %q found", role, name)
	default:
		var where []string
		for _, m := range matches {
			where = append(where, fmt.Sprintf("%s/%s", m.cluster, netpol.NamespaceOf(m.workload.Namespace)))
		}
		sort.Strings(where)
		return matchedWorkload{}, fmt.Errorf("%s workload %q is ambiguous (%s); narrow with --cluster / --%s-namespace",
			role, name, strings.Join(where, ", "), nsFlagFor(role))
	}
}

func nsFlagFor(role string) string {
	if role == "source" {
		return "src"
	}
	return "dst"
}
