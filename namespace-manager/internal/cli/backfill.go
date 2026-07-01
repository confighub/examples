// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/confighub/sdk/cliutil"
	"github.com/confighub/sdk/core/cubapi"
	api "github.com/confighub/sdk/core/function/api"

	"github.com/confighub/examples/namespace-manager/internal/cub"
	"github.com/confighub/examples/namespace-manager/internal/nsmanager"
	"github.com/confighub/examples/namespace-manager/internal/snapshot"
)

type backfillReport struct {
	Command    string   `json:"command"`
	From       string   `json:"from"`
	Space      string   `json:"space"`
	Namespace  string   `json:"namespace"`
	DryRun     bool     `json:"dryRun"`
	WouldClone []string `json:"wouldClone,omitempty"`
	Cloned     []string `json:"cloned,omitempty"`
	Rehomed    int      `json:"rehomed"`
}

func newBackfillCmd() *cobra.Command {
	var output, spaceSlug, fromSpace, namespace string
	var commit cliutil.CommitFlags
	cmd := &cobra.Command{
		Use:   "backfill --space <dest> --from <base-space> --namespace <ns>",
		Short: "Clone a base envelope's Units into an existing Space and re-home them with set-namespace",
		Long: `backfill injects a namespace policy envelope into an existing Space — one that
already holds workloads (e.g. a Helm/Kustomize-ingested app Space) but is missing
its default-deny NetworkPolicy and baseline RBAC. It clones every deployable Unit
from the base Space (--from) into the destination Space (--space) via the
BulkCreateUnits API, then runs set-namespace to re-home the clones to the target
namespace (--namespace) — set-namespace also rewrites RBAC subject namespaces and
Service DNS references, so the whole envelope lands correctly.

This is the half the installer's 'new'/'upload' doesn't cover (greenfield only).
Re-runs are idempotent (already-present clones are reported, not duplicated).
Unit creation has no server-side dry-run, so dry-run lists the plan and only
--commit --change-desc actually clones + re-homes. Units are created, not applied.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if spaceSlug == "" || fromSpace == "" || namespace == "" {
				return fmt.Errorf("--space, --from, and --namespace are all required")
			}
			changeDesc, dryRun, err := commit.Validate(
				fmt.Sprintf("backfill envelope from %s into %s (namespace %s)", fromSpace, spaceSlug, namespace))
			if err != nil {
				return err
			}
			client, err := cub.Preflight(cmd.Context())
			if err != nil {
				return err
			}
			base, err := cubapi.ResolveSpace(cmd.Context(), client, fromSpace)
			if err != nil {
				return fmt.Errorf("resolve --from space %q: %w", fromSpace, err)
			}
			dest, err := cubapi.ResolveSpace(cmd.Context(), client, spaceSlug)
			if err != nil {
				return fmt.Errorf("resolve --space %q: %w", spaceSlug, err)
			}

			// Classify base + dest Units so we clone only the members the dest is
			// missing. Crucially, skip the base's Namespace Unit when the dest
			// already has one — cloning it would create a duplicate-namespace
			// collision (which findings flags). The base's non-Namespace members
			// (default-deny NetworkPolicy, baseline RBAC) are what backfill adds.
			scopeWhere := fmt.Sprintf("SpaceID IN ('%s', '%s')", base.SpaceID.String(), dest.SpaceID.String())
			snap, err := snapshot.Load(cmd.Context(), client, scopeWhere)
			if err != nil {
				return fmt.Errorf("load base + dest snapshot: %w", err)
			}
			baseSlugs := map[string]bool{}
			baseNamespaceSlugs := map[string]bool{}
			destHasNamespace := false
			for _, r := range snap.Resources {
				kind, _, _, ok := nsmanager.ResourceMeta(r.Doc)
				if !ok {
					continue
				}
				switch r.Origin.Space {
				case fromSpace:
					if r.Origin.UnitSlug == "installer-record" {
						continue
					}
					baseSlugs[r.Origin.UnitSlug] = true
					if kind == "Namespace" {
						baseNamespaceSlugs[r.Origin.UnitSlug] = true
					}
				case spaceSlug:
					if kind == "Namespace" {
						destHasNamespace = true
					}
				}
			}
			var cloneSlugs []string
			for s := range baseSlugs {
				if destHasNamespace && baseNamespaceSlugs[s] {
					continue // dest already has a Namespace; don't clone another
				}
				cloneSlugs = append(cloneSlugs, s)
			}
			sort.Strings(cloneSlugs)

			report := backfillReport{Command: "backfill", From: fromSpace, Space: spaceSlug, Namespace: namespace, DryRun: dryRun}
			if len(cloneSlugs) == 0 {
				return fmt.Errorf("no envelope Units to backfill from base Space %q (nothing missing)", fromSpace)
			}
			srcWhere := fmt.Sprintf("SpaceID = '%s' AND Slug IN (%s)", base.SpaceID.String(), inList(cloneSlugs))
			if dryRun {
				report.WouldClone = cloneSlugs
				return reportBackfill(cmd, report, output)
			}

			// Commit: clone base -> dest (idempotent), then re-home the clones.
			destWhere := fmt.Sprintf("SpaceID = '%s'", dest.SpaceID.String())
			cloned, err := cub.BulkCloneUnits(cmd.Context(), client, srcWhere, destWhere, true)
			if err != nil {
				return fmt.Errorf("clone envelope into %q: %w", spaceSlug, err)
			}
			var clonedSlugs []string
			for _, r := range cloned {
				if r.Unit != nil {
					clonedSlugs = append(clonedSlugs, r.Unit.Slug)
				}
			}
			sort.Strings(clonedSlugs)
			report.Cloned = clonedSlugs

			if len(clonedSlugs) > 0 {
				sel := cubapi.Selector{Where: fmt.Sprintf("SpaceID = '%s' AND Slug IN (%s)", dest.SpaceID.String(), inList(clonedSlugs))}
				res, err := cub.InvokeMutation(cmd.Context(), client, "set-namespace",
					[]api.FunctionArgument{{ParameterName: "namespace-name", Value: namespace}},
					sel, cubapi.Change{Description: changeDesc})
				if err != nil {
					return fmt.Errorf("re-home cloned Units to namespace %q: %w", namespace, err)
				}
				for _, o := range res.Outcomes {
					if o.HasMutations {
						report.Rehomed++
					}
				}
			}
			return reportBackfill(cmd, report, output)
		},
	}
	addOutputFlag(cmd, &output)
	commit.Bind(cmd)
	cmd.Flags().StringVar(&spaceSlug, "space", "", "destination Space to backfill the envelope into (required)")
	cmd.Flags().StringVar(&fromSpace, "from", "", "base Space holding the envelope template to clone (required)")
	cmd.Flags().StringVar(&namespace, "namespace", "", "namespace to re-home the cloned envelope to (required)")
	return cmd
}

// inList renders slugs as a quoted, comma-separated list for a ConfigHub
// `Slug IN (...)` predicate.
func inList(slugs []string) string {
	quoted := make([]string, len(slugs))
	for i, s := range slugs {
		quoted[i] = "'" + strings.ReplaceAll(s, "'", "''") + "'"
	}
	return strings.Join(quoted, ", ")
}

func reportBackfill(cmd *cobra.Command, r backfillReport, output string) error {
	if output != outputTable {
		return printJSON(cmd.OutOrStdout(), r)
	}
	if r.DryRun {
		fprintln(cmd.OutOrStdout(), fmt.Sprintf("Would clone %d Unit(s) from %s into %s, then set-namespace to %q:",
			len(r.WouldClone), r.From, r.Space, r.Namespace))
		for _, s := range r.WouldClone {
			fprintln(cmd.OutOrStdout(), "  "+s)
		}
		fprintln(cmd.OutOrStdout(), "\n(dry-run — pass --commit --change-desc to write)")
		return nil
	}
	fprintln(cmd.OutOrStdout(), fmt.Sprintf("Cloned %d Unit(s) into %s and re-homed %d to namespace %q:",
		len(r.Cloned), r.Space, r.Rehomed, r.Namespace))
	for _, s := range r.Cloned {
		fprintln(cmd.OutOrStdout(), "  "+s)
	}
	return nil
}
