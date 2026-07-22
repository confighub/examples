// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/confighub/sdk/core/cubapi"

	"github.com/confighub/examples/eks-manager/internal/cub"
	"github.com/confighub/examples/eks-manager/internal/eks"
	"github.com/confighub/examples/eks-manager/internal/snapshot"
)

type plannedUnit struct {
	Cluster       string               `json:"cluster"`
	Space         string               `json:"space"`
	Unit          string               `json:"unit"`
	FromRevision  int64                `json:"fromRevision"`
	ToRevision    int64                `json:"toRevision"`
	Resources     []eks.ResourceChange `json:"resources,omitempty"`
	MaxDisruption eks.Disruption       `json:"maxDisruption"`
	MaxScore      string               `json:"maxScore,omitempty"`
	Blocks        bool                 `json:"blocks"`
	Remediation   string               `json:"remediation,omitempty"`
	Error         string               `json:"error,omitempty"`
}

type planReport struct {
	Units  []plannedUnit `json:"units"`
	Totals struct {
		UnitsWithChanges int `json:"unitsWithChanges"`
		Blocking         int `json:"blocking"`
		Rolling          int `json:"rolling"`
		InPlace          int `json:"inPlace"`
		NeverApplied     int `json:"neverApplied"`
	} `json:"totals"`
	Filter string `json:"filter,omitempty"`
}

func newPlanCmd() *cobra.Command {
	var output string
	var filter filterFlags
	var blockingOnly bool

	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Grade pending changes: in-place, rolling, or a replacement that cannot apply",
		Long: `plan compares each Unit's head revision against its last-applied revision and
grades every changed field by what applying it will actually do:

  in-place            an ordinary update
  in-place-disruptive in place, but service-affecting (a control-plane upgrade,
                      an addon restart)
  rolling             in place, but EKS drains and replaces every node in the group
  replace             the resource must be destroyed and recreated
  replace-cluster     the EKS control plane must be replaced

The last two are the reason this command exists. Terraform destroys and
recreates a resource when an immutable field changes; Crossplane refuses — it
returns "refuse to update the external resource because the following update
requires replacing it", the managed resource goes Synced=False, and it retries
forever while AWS is never touched.

So under a source of record, editing a node group's instanceTypes gives you a
committed revision, a clean diff, and a successful apply — and then nothing
happens, permanently. Every signal reads as success while the record and reality
silently diverge. There is no other place in the pipeline where that is visible
before it happens.

For a Crossplane managed resource, immutable fields are identity, not
configuration. A change to one is not an edit; it is a different resource.

Units that have never been applied are reported as such and not graded: with no
baseline there is nothing to disrupt, because creating a resource is not
replacing one.`,
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
			report := buildPlanReport(cmd.Context(), client, snap, blockingOnly)
			if output == outputTable {
				printPlanTable(cmd, report)
				return nil
			}
			return printJSON(cmd.OutOrStdout(), report)
		},
	}
	addOutputFlag(cmd, &output)
	addFilterFlags(cmd, &filter)
	cmd.Flags().BoolVar(&blockingOnly, "blocking-only", false, "report only changes that cannot be applied in place")
	return cmd
}

func buildPlanReport(ctx context.Context, client *cubapi.Client, snap *snapshot.Snapshot, blockingOnly bool) planReport {
	var report planReport

	// Map each Unit to the cluster it belongs to, for reporting.
	clusterOf := map[string]string{}
	for _, r := range snap.Resources {
		clusterOf[r.Origin.UnitID] = r.Origin.Cluster
	}

	for _, meta := range snap.Units {
		// Nothing pending.
		if meta.HeadRevisionNum <= meta.LastAppliedRevisionNum {
			continue
		}
		pu := plannedUnit{
			Cluster:      clusterOf[meta.UnitID],
			Space:        meta.SpaceSlug,
			Unit:         meta.Slug,
			FromRevision: meta.LastAppliedRevisionNum,
			ToRevision:   meta.HeadRevisionNum,
		}
		if pu.Cluster == "" {
			pu.Cluster = meta.SpaceSlug
		}

		// Never applied: no baseline, so nothing to disrupt. Creating a resource
		// is not replacing one.
		if meta.LastAppliedRevisionNum == 0 {
			report.Totals.NeverApplied++
			continue
		}

		oldDocs, err := cub.RevisionDocs(ctx, client, meta.SpaceID, meta.UnitID, meta.LastAppliedRevisionNum)
		if err != nil {
			pu.Error = fmt.Sprintf("read revision %d: %v", meta.LastAppliedRevisionNum, err)
			report.Units = append(report.Units, pu)
			continue
		}
		newDocs, err := cub.RevisionDocs(ctx, client, meta.SpaceID, meta.UnitID, meta.HeadRevisionNum)
		if err != nil {
			pu.Error = fmt.Sprintf("read revision %d: %v", meta.HeadRevisionNum, err)
			report.Units = append(report.Units, pu)
			continue
		}

		keys := map[string]bool{}
		for k := range oldDocs {
			keys[k] = true
		}
		for k := range newDocs {
			keys[k] = true
		}
		for key := range keys {
			old, inOld := oldDocs[key]
			nw, inNew := newDocs[key]
			// A resource added or removed from the Unit is a create/delete, not
			// an in-place edit, so there is no disruption to grade.
			if !inOld || !inNew {
				continue
			}
			paths := eks.DiffPaths(old, nw)
			if len(paths) == 0 {
				continue
			}
			apiVersion, kind, name, _ := eks.ResourceMeta(nw)
			rc := eks.ClassifyResource(apiVersion+"/"+kind, name, paths)
			pu.Resources = append(pu.Resources, rc)
			pu.MaxDisruption = eks.MaxDisruption(pu.MaxDisruption, rc.MaxDisruption)
			if rc.Blocks {
				pu.Remediation = eks.Remediation(rc.MaxDisruption, kind)
			}
		}
		if len(pu.Resources) == 0 {
			continue
		}
		sort.Slice(pu.Resources, func(i, j int) bool {
			return pu.Resources[i].MaxDisruption.Rank() > pu.Resources[j].MaxDisruption.Rank()
		})
		pu.MaxScore = pu.MaxDisruption.Score()
		pu.Blocks = pu.MaxDisruption.Blocks()
		if blockingOnly && !pu.Blocks {
			continue
		}
		report.Units = append(report.Units, pu)
	}

	sort.Slice(report.Units, func(i, j int) bool {
		if report.Units[i].MaxDisruption.Rank() != report.Units[j].MaxDisruption.Rank() {
			return report.Units[i].MaxDisruption.Rank() > report.Units[j].MaxDisruption.Rank()
		}
		if report.Units[i].Cluster != report.Units[j].Cluster {
			return report.Units[i].Cluster < report.Units[j].Cluster
		}
		return report.Units[i].Unit < report.Units[j].Unit
	})
	for _, u := range report.Units {
		report.Totals.UnitsWithChanges++
		switch {
		case u.Blocks:
			report.Totals.Blocking++
		case u.MaxDisruption == eks.DisruptionRolling:
			report.Totals.Rolling++
		default:
			report.Totals.InPlace++
		}
	}
	report.Filter = snap.Filter
	return report
}

func printPlanTable(cmd *cobra.Command, r planReport) {
	out := cmd.OutOrStdout()
	if len(r.Units) == 0 {
		fprintln(out, "No pending changes to grade.")
		if r.Totals.NeverApplied > 0 {
			fprintln(out, fmt.Sprintf("%d Unit(s) have never been applied (no baseline to compare against).",
				r.Totals.NeverApplied))
		}
		return
	}

	tw := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "DISRUPTION\tCLUSTER\tUNIT\tREV\tRESOURCE\tCHANGED")
	for _, u := range r.Units {
		if u.Error != "" {
			fmt.Fprintf(tw, "error\t%s\t%s\t-\t-\t%s\n", u.Cluster, u.Unit, u.Error)
			continue
		}
		for _, rc := range u.Resources {
			level := string(rc.MaxDisruption)
			if level == "" {
				level = "in-place"
			}
			paths := make([]string, 0, len(rc.Changes))
			for _, c := range rc.Changes {
				paths = append(paths, shortPath(c.Path))
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%d->%d\t%s\t%s\n",
				level, u.Cluster, u.Unit, u.FromRevision, u.ToRevision,
				rc.ResourceName, strings.Join(paths, ", "))
		}
	}
	_ = tw.Flush()

	// Blocking changes get the full explanation; a one-line table cell is not
	// enough for "this will silently never apply".
	for _, u := range r.Units {
		if !u.Blocks {
			continue
		}
		fprintln(out, "")
		fprintln(out, fmt.Sprintf("  %s/%s — %s", u.Space, u.Unit, strings.ToUpper(string(u.MaxDisruption))))
		for _, rc := range u.Resources {
			for _, c := range rc.Changes {
				if !c.Disruption.Blocks() {
					continue
				}
				fprintln(out, fmt.Sprintf("    %s: %s", shortPath(c.Path), c.Reason))
			}
		}
		fprintln(out, "    This cannot be reconciled in place. Crossplane will refuse the update and")
		fprintln(out, "    retry forever; the Unit will read as applied while AWS is never changed.")
		if u.Remediation != "" {
			fprintln(out, "    -> "+u.Remediation)
		}
	}

	fprintln(out, "")
	fprintln(out, fmt.Sprintf("%d Unit(s) with pending changes: %d blocking, %d rolling, %d in-place.",
		r.Totals.UnitsWithChanges, r.Totals.Blocking, r.Totals.Rolling, r.Totals.InPlace))
	if r.Totals.NeverApplied > 0 {
		fprintln(out, fmt.Sprintf("%d Unit(s) have never been applied (no baseline to compare against).",
			r.Totals.NeverApplied))
	}
}

// shortPath trims the spec.forProvider prefix, which is on nearly every path and
// carries no information in a table.
func shortPath(p string) string {
	return strings.TrimPrefix(p, "spec.forProvider.")
}
