// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/confighub/examples/rbac-manager-for-agents/internal/cub"
	"github.com/confighub/examples/rbac-manager-for-agents/internal/rbac"
)

// fleetFlags scope a fleet-wide mutation to a set of Units and carry the
// dry-run/commit controls. --where is required so a fleet mutation is always a
// deliberate, scoped action.
type fleetFlags struct {
	where string
	commitFlags
}

func (f *fleetFlags) bind(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.where, "where", "",
		"ConfigHub filter selecting the Units to change (required). ANDed with ToolchainType='Kubernetes/YAML'. "+
			"e.g. \"Space.Labels.Environment = 'prod'\" or \"Slug = 'rbac'\"")
	addCommitFlags(cmd, &f.commitFlags)
}

const k8sWhere = "ToolchainType = 'Kubernetes/YAML'"

// mutationArgs appends the dry-run or commit suffix to a base cub argv,
// enforcing that a commit carries a change description.
func mutationArgs(base []string, c commitFlags) ([]string, error) {
	args := append([]string{}, base...)
	if !c.commit {
		return append(args, "--dry-run"), nil
	}
	if strings.TrimSpace(c.changeDesc) == "" {
		return nil, fmt.Errorf("--change-desc is required with --commit (describe the change and why)")
	}
	return append(args, "--change-desc", c.changeDesc), nil
}

// --- fleet-edit ---

func newFleetEditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fleet-edit",
		Short: "Apply a structured RBAC edit across many Units in one server request",
		Long: `fleet-edit applies the same structured change (add/remove a verb or subject) to
every Unit matching a --where selector, in a single server-side invocation —
useful for persona roles replicated across clusters.

Dry-run by default: it reports the per-Unit changes and writes nothing. Re-run
with --commit and a --change-desc to apply. Like edit, it never bypasses
ApplyGates and does not apply to clusters.`,
	}
	cmd.AddCommand(
		newFleetVerbCmd("add-verb", true),
		newFleetVerbCmd("remove-verb", false),
		newFleetSubjectCmd("add-subject", true),
		newFleetSubjectCmd("remove-subject", false),
	)
	return cmd
}

func newFleetVerbCmd(use string, add bool) *cobra.Command {
	var f verbEditFlags
	var fl fleetFlags
	cmd := &cobra.Command{
		Use:   use,
		Short: fmt.Sprintf("%s on a role across every matching Unit", use),
		Example: `  cub-rbac fleet-edit ` + use + ` --where "Space.Labels.Environment = 'dev'" \
    --role-kind ClusterRole --role developer --rule 0 --verb deletecollection`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			edit, err := f.compile(add)
			if err != nil {
				return err
			}
			return runFleetEdit(cmd, fl, edit)
		},
	}
	f.bind(cmd)
	fl.bind(cmd)
	return cmd
}

func newFleetSubjectCmd(use string, add bool) *cobra.Command {
	var f subjectEditFlags
	var fl fleetFlags
	cmd := &cobra.Command{
		Use:   use,
		Short: fmt.Sprintf("%s on a binding across every matching Unit", use),
		Example: `  cub-rbac fleet-edit ` + use + ` --where "Space.Labels.Environment = 'prod'" \
    --binding-kind ClusterRoleBinding --binding viewers --subject-kind Group --subject-name oncall`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			edit, err := f.compile(add)
			if err != nil {
				return err
			}
			return runFleetEdit(cmd, fl, edit)
		},
	}
	f.bind(cmd)
	fl.bind(cmd)
	return cmd
}

// fleetEditResp is the subset of cub's function-invocation response we read.
// (Org-scoped function do must use -o json, not -o mutations: the mutations
// renderer panics on --space "*".)
type fleetEditResp struct {
	Success         bool   `json:"Success"`
	HasNewMutations bool   `json:"HasNewMutations"`
	SpaceSlug       string `json:"SpaceSlug"`
	UnitSlug        string `json:"UnitSlug"`
}

func runFleetEdit(cmd *cobra.Command, fl fleetFlags, edit rbac.EditInvocation) error {
	if strings.TrimSpace(fl.where) == "" {
		return fmt.Errorf("--where is required to scope a fleet edit (use a deliberate selector, e.g. \"Space.Labels.Environment = 'prod'\")")
	}
	if err := cub.Preflight(cmd.Context()); err != nil {
		return err
	}
	where := k8sWhere + " AND " + fl.where
	// Org-scoped invoke must use -o json (the mutations renderer panics on --space "*").
	base := []string{"invocation", "invoke", "set", rbac.EditLibrarySpace + "/" + edit.Slug,
		"--space", "*", "--where", where, "-o", "json"}
	for _, p := range edit.Params {
		base = append(base, "--param", p)
	}
	args, err := mutationArgs(base, fl.commitFlags)
	if err != nil {
		return err
	}
	out, err := cub.Run(cmd.Context(), args...)
	if err != nil {
		return err
	}
	var resps []fleetEditResp
	if out != "" {
		if err := json.Unmarshal([]byte(out), &resps); err != nil {
			return fmt.Errorf("parse function response: %w", err)
		}
	}
	var changed []string
	for _, r := range resps {
		if r.Success && r.HasNewMutations {
			changed = append(changed, r.SpaceSlug+"/"+r.UnitSlug)
		}
	}
	sort.Strings(changed)
	return reportFleet(cmd, fl.commitFlags, edit.Summary+" — fleet: "+fl.where, changed)
}

// --- promote (variant propagation) ---

func newPromoteCmd() *cobra.Command {
	var fl fleetFlags
	cmd := &cobra.Command{
		Use:   "promote",
		Short: "Upgrade downstream Units to their upstream (override-preserving)",
		Long: `promote upgrades downstream (cloned) Units to the latest revision of their
upstream Unit, preserving intentional local overrides — the way a base RBAC
change is propagated to its per-environment variants.

Only Units cloned from an upstream (UpstreamRevisionNum > 0) and matching --where
are affected; Units already at their upstream head are no-ops. Dry-run by
default; re-run with --commit and a --change-desc to apply. Applying the
upgraded revisions to clusters is a separate step (cub unit apply).`,
		Example: `  cub-rbac promote --where "Space.Labels.Environment = 'staging'"
  cub-rbac promote --where "Space.Labels.Environment = 'prod'" --commit --change-desc "propagate base RBAC update (OPS-12)"`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(fl.where) == "" {
				return fmt.Errorf("--where is required to scope the promotion (e.g. \"Space.Labels.Environment = 'prod'\")")
			}
			if err := cub.Preflight(cmd.Context()); err != nil {
				return err
			}
			where := k8sWhere + " AND UpstreamRevisionNum > 0 AND " + fl.where
			base := []string{"unit", "update", "--patch", "--space", "*", "--where", where, "--upgrade", "-o", "json"}
			args, err := mutationArgs(base, fl.commitFlags)
			if err != nil {
				return err
			}
			out, err := cub.Run(cmd.Context(), args...)
			if err != nil {
				return err
			}
			changed, err := parsePromoteChanged(cmd.Context(), out)
			if err != nil {
				return err
			}
			return reportFleet(cmd, fl.commitFlags, "Upgrade downstream Units from upstream — fleet: "+fl.where, changed)
		},
	}
	fl.bind(cmd)
	return cmd
}

// parsePromoteChanged extracts space/unit labels from a bulk unit-update
// response. The elements carry a Unit (with SpaceID); slugs are resolved to a
// readable space/unit via the space-id map only when needed.
func parsePromoteChanged(ctx context.Context, out string) ([]string, error) {
	var rows []struct {
		Unit struct {
			Slug    string `json:"Slug"`
			SpaceID string `json:"SpaceID"`
		} `json:"Unit"`
	}
	if out != "" {
		if err := json.Unmarshal([]byte(out), &rows); err != nil {
			return nil, fmt.Errorf("parse unit-update response: %w", err)
		}
	}
	if len(rows) == 0 {
		return nil, nil
	}
	spaceSlug := spaceIDToSlug(ctx)
	changed := make([]string, 0, len(rows))
	for _, r := range rows {
		slug := spaceSlug[r.Unit.SpaceID]
		if slug == "" {
			slug = r.Unit.SpaceID
		}
		changed = append(changed, slug+"/"+r.Unit.Slug)
	}
	sort.Strings(changed)
	return changed, nil
}

// spaceIDToSlug builds a SpaceID→Slug map; best-effort (returns empty on error).
func spaceIDToSlug(ctx context.Context) map[string]string {
	out, err := cub.Run(ctx, "space", "list", "--select", "Slug,SpaceID", "-o", "json")
	if err != nil {
		return map[string]string{}
	}
	var rows []struct {
		Space struct {
			SpaceID string `json:"SpaceID"`
			Slug    string `json:"Slug"`
		} `json:"Space"`
	}
	_ = json.Unmarshal([]byte(out), &rows)
	m := make(map[string]string, len(rows))
	for _, r := range rows {
		m[r.Space.SpaceID] = r.Space.Slug
	}
	return m
}

// reportFleet prints the dry-run or commit summary for a fleet mutation.
func reportFleet(cmd *cobra.Command, c commitFlags, summary string, changed []string) error {
	out := cmd.OutOrStdout()
	if !c.commit {
		fprintln(out, "Dry run — "+summary)
		if len(changed) == 0 {
			fprintln(out, "No changes: already in effect, or no matching Units.")
			return nil
		}
		fprintln(out, fmt.Sprintf("Would change %d Unit(s):", len(changed)))
		for _, u := range changed {
			fprintln(out, "  "+u)
		}
		fprintln(out, "\nRe-run with --commit --change-desc \"...\" to apply.")
		return nil
	}
	fprintln(out, "Committed — "+summary)
	if len(changed) == 0 {
		fprintln(out, "No Units changed (already in effect, or no matching Units).")
		return nil
	}
	fprintln(out, fmt.Sprintf("Changed %d Unit(s):", len(changed)))
	for _, u := range changed {
		fprintln(out, "  "+u)
	}
	return nil
}
