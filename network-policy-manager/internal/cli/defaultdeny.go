// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/confighub/sdk/cliutil"

	"github.com/confighub/examples/network-policy-manager/internal/cub"
	"github.com/confighub/examples/network-policy-manager/internal/netpol"
	"github.com/confighub/examples/network-policy-manager/internal/snapshot"
)

func newDefaultDenyCmd() *cobra.Command {
	var output, clusterFilter, spaceOverride string
	var egress bool
	var scope scopeFlags
	var commit cliutil.CommitFlags
	cmd := &cobra.Command{
		Use:   "default-deny <namespace>",
		Short: "Create a default-deny NetworkPolicy Unit for a namespace (dry-run unless --commit)",
		Long: `default-deny generates a default-deny NetworkPolicy (podSelector {}, selecting
all pods) for a namespace and creates it as a ConfigHub Unit alongside the
namespace's existing workloads — closing the "namespace has no default-deny"
coverage gap as data, with no cluster drift.

By default it denies all ingress. With --egress it also denies egress but allows
DNS to kube-dns, since a bare egress default-deny breaks name resolution.

The Space (and cluster Target) are inferred from the namespace's managed
workloads; use --cluster / --space to disambiguate. The Unit is created but NOT
applied — deploy it with 'cub unit apply' when ready.

This is a dry run unless you pass --commit --change-desc "…".`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			namespace := args[0]
			summary := fmt.Sprintf("create default-deny NetworkPolicy for namespace %s", namespace)
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
			dest, err := resolveCreateDest(snap, namespace, clusterFilter, spaceOverride)
			if err != nil {
				return err
			}
			slug, manifest := netpol.DefaultDenyYAML(namespace, egress)
			return runCreate(cmd, client, dest, slug, manifest, namespace, dryRun, changeDesc, output)
		},
	}
	addOutputFlag(cmd, &output)
	addScopeFlags(cmd, &scope)
	cmd.Flags().StringVar(&clusterFilter, "cluster", "", "cluster (Target or Space slug) when the namespace exists in more than one")
	cmd.Flags().StringVar(&spaceOverride, "space", "", "Space slug to create the Unit in when the namespace spans more than one")
	cmd.Flags().BoolVar(&egress, "egress", false, "also deny egress (allowing DNS egress to kube-dns)")
	commit.Bind(cmd)
	return cmd
}
