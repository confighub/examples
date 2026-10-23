// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/confighub/examples/managerkit"
	"github.com/confighub/examples/namespace-manager/internal/cub"
	"github.com/confighub/examples/namespace-manager/internal/nsmanager"
	"github.com/confighub/examples/namespace-manager/internal/snapshot"
)

type podSecurityReport struct {
	// Version is the Pod Security Standards version checked against.
	Version    string                           `json:"version"`
	Namespaces []nsmanager.NamespacePodSecurity `json:"namespaces"`
	// Unchecked lists the Units the function could not check, and why.
	Unchecked []uncheckedUnit `json:"unchecked,omitempty"`
	Totals    struct {
		Namespaces  int `json:"namespaces"`
		Restricted  int `json:"restricted"`
		Baseline    int `json:"baseline"`
		Privileged  int `json:"privileged"`
		Violating   int `json:"violating"`
		Tightenable int `json:"tightenable"`
		Unlabeled   int `json:"unlabeled"`
	} `json:"totals"`
}

type uncheckedUnit struct {
	Space    string `json:"space"`
	UnitSlug string `json:"unitSlug"`
	UnitID   string `json:"unitId"`
	Error    string `json:"error"`
}

var podSecurityStatuses = []string{
	nsmanager.PodSecurityViolating, nsmanager.PodSecurityUnlabeled,
	nsmanager.PodSecurityTightenable, nsmanager.PodSecurityAtLevel,
}

func newPodSecurityCmd() *cobra.Command {
	var output string
	var filter filterFlags
	var clusterFilter, namespaceFilter, statusFilter, version string
	cmd := &cobra.Command{
		Use:   "pod-security",
		Short: "Which Pod Security Standard each namespace's workloads meet, and which workloads block a stricter one",
		Long: `pod-security reports, for every namespace with workloads, the most restrictive
Pod Security Standard level (privileged < baseline < restricted) that all of its
workloads meet, and the workloads -- by Space, Unit and resource -- that keep it
from the next level, with the checks each fails.

The checks are the server's vet-pod-security-standard function, which runs the
same checks the PodSecurity admission controller does; only Units holding
workloads are checked, and nothing is written.

Each namespace's status compares that level with its Namespace's
pod-security.kubernetes.io/enforce label:

  violating    the label enforces a level some workload fails: admission rejects its pods
  unlabeled    no enforce label (or no Namespace Unit to carry one)
  tightenable  the workloads meet a stricter level than the label enforces
  at-level     the label enforces exactly the level the workloads meet

Filter with --cluster, --namespace and --status; --version checks against a
Kubernetes minor version of the standards instead of the latest.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if statusFilter != "" && !slices.Contains(podSecurityStatuses, statusFilter) {
				return fmt.Errorf("--status must be one of %s", strings.Join(podSecurityStatuses, ", "))
			}
			client, err := cub.Preflight(cmd.Context())
			if err != nil {
				return err
			}
			where, err := filter.Predicate()
			if err != nil {
				return err
			}
			snap, err := snapshot.Load(cmd.Context(), client, where)
			if err != nil {
				return err
			}
			unitIDs := nsmanager.PodSecurityUnitIDs(snap.Clusters)
			results := nsmanager.PodSecurityResults{Unchecked: map[string]string{}}
			if results.Restricted, err = cub.VetPodSecurity(cmd.Context(), client, nsmanager.LevelRestricted, version, unitIDs, results.Unchecked); err != nil {
				return err
			}
			if results.Baseline, err = cub.VetPodSecurity(cmd.Context(), client, nsmanager.LevelBaseline, version, unitIDs, results.Unchecked); err != nil {
				return err
			}
			report := buildPodSecurityReport(nsmanager.AnalyzePodSecurity(snap.Clusters, results), clusterFilter, namespaceFilter, statusFilter)
			report.Unchecked = uncheckedUnits(snap, results.Unchecked)
			report.Version = version
			if report.Version == "" {
				report.Version = "latest"
			}
			if done, err := managerkit.Render(cmd.OutOrStdout(), output, report); done || err != nil {
				return err
			}
			printPodSecurityTable(cmd, report)
			return nil
		},
	}
	addOutputFlag(cmd, &output)
	addFilterFlags(cmd, &filter)
	cmd.Flags().StringVar(&clusterFilter, "cluster", "", "restrict output to this cluster (Target slug, or None for Units whose Space has no release Target)")
	cmd.Flags().StringVar(&namespaceFilter, "namespace", "", "filter by namespace")
	cmd.Flags().StringVar(&statusFilter, "status", "", "filter by status: "+strings.Join(podSecurityStatuses, " | "))
	cmd.Flags().StringVar(&version, "version", "", `Pod Security Standards version to check against: "latest" (the default) or a Kubernetes minor version such as "v1.30"`)
	return cmd
}

func buildPodSecurityReport(all []nsmanager.NamespacePodSecurity, clusterFilter, namespaceFilter, statusFilter string) podSecurityReport {
	var report podSecurityReport
	for _, p := range all {
		if clusterFilter != "" && p.Cluster != clusterFilter {
			continue
		}
		if namespaceFilter != "" && p.Namespace != namespaceFilter {
			continue
		}
		if statusFilter != "" && p.Status != statusFilter {
			continue
		}
		report.Namespaces = append(report.Namespaces, p)
		report.Totals.Namespaces++
		switch p.Achievable {
		case nsmanager.LevelRestricted:
			report.Totals.Restricted++
		case nsmanager.LevelBaseline:
			report.Totals.Baseline++
		case nsmanager.LevelPrivileged:
			report.Totals.Privileged++
		}
		switch p.Status {
		case nsmanager.PodSecurityViolating:
			report.Totals.Violating++
		case nsmanager.PodSecurityTightenable:
			report.Totals.Tightenable++
		case nsmanager.PodSecurityUnlabeled:
			report.Totals.Unlabeled++
		}
	}
	return report
}

// uncheckedUnits names the Units the function could not check, sorted.
func uncheckedUnits(snap *snapshot.Snapshot, unchecked map[string]string) []uncheckedUnit {
	out := make([]uncheckedUnit, 0, len(unchecked))
	for id, reason := range unchecked {
		u := uncheckedUnit{UnitID: id, Error: reason}
		if meta, ok := snap.Units[id]; ok {
			u.Space, u.UnitSlug = meta.SpaceSlug, meta.Slug
		}
		out = append(out, u)
	}
	slices.SortFunc(out, func(a, b uncheckedUnit) int {
		return strings.Compare(a.Space+"/"+a.UnitSlug+"/"+a.UnitID, b.Space+"/"+b.UnitSlug+"/"+b.UnitID)
	})
	return out
}

func printPodSecurityTable(cmd *cobra.Command, r podSecurityReport) {
	w := cmd.OutOrStdout()
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "CLUSTER\tNAMESPACE\tWORKLOADS\tENFORCE\tACHIEVABLE\tSTATUS\tBLOCKERS")
	for _, p := range r.Namespaces {
		blockers := "-"
		if len(p.Blockers) > 0 {
			blockers = fmt.Sprintf("%d block %s", len(p.Blockers), p.BlockedLevel)
		}
		if p.Unchecked > 0 {
			blockers += fmt.Sprintf(" (+%d unchecked)", p.Unchecked)
		}
		fmt.Fprintf(tw, "%s\t%s\t%d\t%s\t%s\t%s\t%s\n",
			p.Cluster, p.Namespace, p.Workloads, dash(p.Enforce), p.Achievable, p.Status, blockers)
	}
	_ = tw.Flush()
	fprintln(w, fmt.Sprintf(
		"\n%d namespaces against Pod Security Standards %s: %d restricted, %d baseline, %d privileged (%d violating, %d tightenable, %d unlabeled)",
		r.Totals.Namespaces, r.Version, r.Totals.Restricted, r.Totals.Baseline, r.Totals.Privileged,
		r.Totals.Violating, r.Totals.Tightenable, r.Totals.Unlabeled))
	for _, p := range r.Namespaces {
		if len(p.Blockers) == 0 {
			continue
		}
		fprintln(w, fmt.Sprintf("\n%s/%s cannot be %s because of:", p.Cluster, p.Namespace, p.BlockedLevel))
		for _, b := range p.Blockers {
			reasons := make([]string, 0, len(b.Checks))
			for _, c := range b.Checks {
				reasons = append(reasons, c.Reason)
			}
			fprintln(w, fmt.Sprintf("  %s/%s %s/%s: %s", b.Space, b.UnitSlug, b.Kind, b.Name, strings.Join(reasons, "; ")))
		}
	}
	if len(r.Unchecked) > 0 {
		fprintln(w, fmt.Sprintf("\n%d Unit(s) could not be checked, so their namespaces' levels are upper bounds:", len(r.Unchecked)))
		for _, u := range r.Unchecked {
			fprintln(w, fmt.Sprintf("  %s/%s: %s", u.Space, u.UnitSlug, u.Error))
		}
	}
}
