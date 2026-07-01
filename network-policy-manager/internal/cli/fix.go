// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/confighub/sdk/cliutil"
	"github.com/confighub/sdk/core/cubapi"

	"github.com/confighub/examples/network-policy-manager/internal/cub"
	"github.com/confighub/examples/network-policy-manager/internal/netpol"
	"github.com/confighub/examples/network-policy-manager/internal/snapshot"
)

// set-yq expressions. Idempotency is enforced by the caller's client-side guard
// (it only runs when the fix actually applies), so these unconditionally edit.
const (
	// Add Egress to policyTypes and append a DNS-to-kube-dns egress rule.
	yqAddDNSEgress = `.spec.policyTypes = ((.spec.policyTypes // []) + ["Egress"] | unique) | ` +
		`.spec.egress = ((.spec.egress // []) + [{"to": [{"namespaceSelector": {"matchLabels": {"kubernetes.io/metadata.name": "kube-system"}}, "podSelector": {"matchLabels": {"k8s-app": "kube-dns"}}}], "ports": [{"protocol": "UDP", "port": 53}, {"protocol": "TCP", "port": 53}]}])`
	// Add an except for the cloud-metadata IP to every egress ipBlock.
	yqExceptMetadata = `(.spec.egress[]?.to[]? | select(has("ipBlock")) | .ipBlock) |= ` +
		`(.except = ((.except // []) + ["169.254.169.254/32"] | unique))`
)

func newFixCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fix",
		Short: "Patch an existing NetworkPolicy Unit (dns | metadata) via a server-side set-yq edit",
		Long: `fix applies a surgical, idempotent edit to an existing NetworkPolicy Unit through
the mutating set-yq function (a clean revision, no client-side rewrite):

  fix dns <space>/<unit>       add a DNS egress allowance (kube-dns :53) — the
                               allowance an egress default-deny needs to not
                               break name resolution
  fix metadata <space>/<unit>  add an except for the cloud-metadata IP
                               (169.254.169.254/32) to permissive egress ipBlocks

Each is a no-op (and says so) when the fix is already in place. Dry run unless
--commit --change-desc.`,
	}
	cmd.AddCommand(newFixDNSCmd(), newFixMetadataCmd())
	return cmd
}

func newFixDNSCmd() *cobra.Command {
	return newFixSubCmd(
		"dns",
		"Add a DNS egress allowance (kube-dns :53) to a NetworkPolicy Unit",
		yqAddDNSEgress,
		"add DNS egress allowance",
		func(np *netpol.NetworkPolicyEntity) (skip bool, reason string) {
			if np.AllowsDNSEgress() {
				return true, "already allows DNS egress (port 53)"
			}
			return false, ""
		},
	)
}

func newFixMetadataCmd() *cobra.Command {
	return newFixSubCmd(
		"metadata",
		"Add an except for the cloud-metadata IP to a NetworkPolicy Unit's egress ipBlocks",
		yqExceptMetadata,
		"except the cloud-metadata IP from egress ipBlocks",
		func(np *netpol.NetworkPolicyEntity) (skip bool, reason string) {
			if !np.ExposesMetadataEgress() {
				return true, "no permissive egress ipBlock exposes the metadata IP"
			}
			return false, ""
		},
	)
}

func newFixSubCmd(use, short, yqExpr, action string, guard func(*netpol.NetworkPolicyEntity) (bool, string)) *cobra.Command {
	var output string
	var filter filterFlags
	var commit cliutil.CommitFlags
	cmd := &cobra.Command{
		Use:   use + " <space>/<unit>",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			space, unit, err := splitSpaceUnit(args[0])
			if err != nil {
				return err
			}
			changeDesc, dryRun, err := commit.Validate(fmt.Sprintf("%s on %s/%s", action, space, unit))
			if err != nil {
				return err
			}
			client, err := cub.Preflight(cmd.Context())
			if err != nil {
				return err
			}
			sp, err := cubapi.ResolveSpace(cmd.Context(), client, space)
			if err != nil {
				return fmt.Errorf("resolve space %q: %w", space, err)
			}
			snap, err := snapshot.Load(cmd.Context(), client, filter.predicate())
			if err != nil {
				return err
			}
			np := findPolicy(snap, space, unit)
			if np == nil {
				return fmt.Errorf("no NetworkPolicy Unit %q found in space %q", unit, space)
			}
			if skip, reason := guard(np); skip {
				fprintln(cmd.OutOrStdout(), fmt.Sprintf("%s/%s: %s — nothing to do.", space, unit, reason))
				return nil
			}

			ch := cubapi.Change{}
			if !dryRun {
				ch = cubapi.Change{Description: changeDesc}
			}
			res, err := cub.MutateUnitYQ(cmd.Context(), client, sp.SpaceID, unit, yqExpr, ch)
			if err != nil {
				return err
			}
			return reportFix(cmd, res, space, unit, action, dryRun, output)
		},
	}
	addOutputFlag(cmd, &output)
	addFilterFlags(cmd, &filter)
	commit.Bind(cmd)
	return cmd
}

type fixResult struct {
	Action  string `json:"action"`
	Space   string `json:"space"`
	Unit    string `json:"unit"`
	DryRun  bool   `json:"dryRun"`
	Mutated bool   `json:"mutated"`
	Error   string `json:"error,omitempty"`
}

func reportFix(cmd *cobra.Command, res *cubapi.Result, space, unit, action string, dryRun bool, output string) error {
	r := fixResult{Action: action, Space: space, Unit: unit, DryRun: dryRun}
	for _, o := range res.Outcomes {
		if !o.Success {
			r.Error = o.Error
		}
		if o.HasMutations {
			r.Mutated = true
		}
	}
	if output == outputJSON {
		return printJSON(cmd.OutOrStdout(), r)
	}
	out := cmd.OutOrStdout()
	switch {
	case r.Error != "":
		return fmt.Errorf("fix failed on %s/%s: %s", space, unit, r.Error)
	case dryRun && r.Mutated:
		fprintln(out, fmt.Sprintf("Dry run — would %s on %s/%s. Re-run with --commit --change-desc \"…\".", action, space, unit))
	case dryRun:
		fprintln(out, fmt.Sprintf("Dry run — no change needed on %s/%s.", space, unit))
	default:
		fprintln(out, fmt.Sprintf("Patched %s/%s (%s).", space, unit, action))
	}
	return nil
}

// findPolicy locates a NetworkPolicy entity by its Space slug and Unit slug.
func findPolicy(snap *snapshot.Snapshot, space, unit string) *netpol.NetworkPolicyEntity {
	for _, c := range snap.Clusters {
		for _, np := range c.NetworkPolicies {
			if np.Origin.Space == space && np.Origin.UnitSlug == unit {
				return np
			}
		}
	}
	return nil
}

func splitSpaceUnit(s string) (space, unit string, err error) {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("expected <space>/<unit>, got %q", s)
	}
	return parts[0], parts[1], nil
}
