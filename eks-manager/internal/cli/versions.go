// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/confighub/examples/eks-manager/internal/cub"
	"github.com/confighub/examples/eks-manager/internal/eks"
	"github.com/confighub/examples/eks-manager/internal/snapshot"
)

// nodeGroupVersion is one node group's version relative to its control plane.
type nodeGroupVersion struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
	// Skew is how many minor versions this node group is behind the control
	// plane. 0 means level (or ahead, or unknown).
	Skew int `json:"skew"`
	// Unpinned is true when the node group declares no version, so it tracks
	// whatever the control plane is at.
	Unpinned bool `json:"unpinned,omitempty"`
}

type clusterVersions struct {
	Cluster     string             `json:"cluster"`
	Version     string             `json:"version,omitempty"`
	Region      string             `json:"region,omitempty"`
	Environment string             `json:"environment,omitempty"`
	SupportType string             `json:"supportType,omitempty"`
	AutoMode    bool               `json:"autoMode"`
	NodeGroups  []nodeGroupVersion `json:"nodeGroups,omitempty"`
	MaxSkew     int                `json:"maxSkew"`
	Addons      map[string]string  `json:"addons,omitempty"`
	// SupportTypeRisk flags a cluster with a pinned version and STANDARD support:
	// AWS will auto-upgrade at end-of-standard-support, which then fights the pin
	// and produces perpetual drift.
	SupportTypeRisk bool `json:"supportTypeRisk,omitempty"`
}

type versionsReport struct {
	Clusters []clusterVersions `json:"clusters"`
	// Distinct control-plane versions across the fleet, most common first.
	VersionCounts map[string]int `json:"versionCounts,omitempty"`
	Totals        struct {
		Clusters         int `json:"clusters"`
		DistinctVersions int `json:"distinctVersions"`
		Skewed           int `json:"skewedClusters"`
		SupportTypeRisk  int `json:"supportTypeRisk"`
	} `json:"totals"`
	Filter string `json:"filter,omitempty"`
}

func newVersionsCmd() *cobra.Command {
	var output string
	var filter filterFlags
	cmd := &cobra.Command{
		Use:   "versions",
		Short: "Fleet version matrix: control planes, node group skew, and addon versions",
		Long: `versions reports the Kubernetes version of every EKS control plane in scope,
the version skew of each node group against its own control plane, and the addon
versions installed.

This is the fleet question eksctl structurally cannot answer — "which of my
clusters are still on 1.32?" is a question about a set of clusters' source of
record, and eksctl's record is per-cluster CloudFormation.

Two things are flagged:

  - Node group skew, in minor versions behind the control plane. A node group
    with no version of its own is reported as unpinned (it tracks the control
    plane).
  - A pinned control-plane version combined with upgradePolicy.supportType
    STANDARD. AWS auto-upgrades at end of standard support, which then fights
    the pinned version and drifts perpetually. Pin the version or accept the
    auto-upgrade — not both.

Whether a given upgrade is legal (EKS allows one minor forward, never a
downgrade) is checked by the 'plan' command, a later milestone.`,
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
			report := buildVersionsReport(snap)
			if output == outputTable {
				printVersionsTable(cmd, report)
				return nil
			}
			return printJSON(cmd.OutOrStdout(), report)
		},
	}
	addOutputFlag(cmd, &output)
	addFilterFlags(cmd, &filter)
	return cmd
}

func buildVersionsReport(snap *snapshot.Snapshot) versionsReport {
	var report versionsReport
	report.VersionCounts = map[string]int{}

	for name, cs := range snap.Clusters {
		if cs.Control == nil {
			continue // no control plane in scope; nothing to version
		}
		c := cs.Control
		cv := clusterVersions{
			Cluster:     name,
			Version:     c.Version,
			Region:      c.Region,
			SupportType: c.UpgradeSupportType,
			AutoMode:    c.AutoMode.Enabled(),
			Environment: c.Origin.SpaceLabels["Environment"],
		}
		if c.Version != "" {
			report.VersionCounts[c.Version]++
		}
		// A pinned version under STANDARD support will be auto-upgraded by AWS.
		if c.Version != "" && strings.EqualFold(c.UpgradeSupportType, "STANDARD") {
			cv.SupportTypeRisk = true
		}

		cpVersion, cpOK := eks.ParseVersion(c.Version)
		for _, n := range cs.NodeGroups {
			ngv := nodeGroupVersion{Name: n.Name, Version: n.Version}
			if n.Version == "" {
				ngv.Unpinned = true
			} else if cpOK {
				if v, ok := eks.ParseVersion(n.Version); ok {
					ngv.Skew = eks.MinorSkew(cpVersion, v)
					if ngv.Skew > cv.MaxSkew {
						cv.MaxSkew = ngv.Skew
					}
				}
			}
			cv.NodeGroups = append(cv.NodeGroups, ngv)
		}
		sort.Slice(cv.NodeGroups, func(i, j int) bool { return cv.NodeGroups[i].Name < cv.NodeGroups[j].Name })

		if len(cs.Addons) > 0 {
			cv.Addons = map[string]string{}
			for _, a := range cs.Addons {
				if a.AddonName != "" {
					cv.Addons[a.AddonName] = a.AddonVersion
				}
			}
		}
		report.Clusters = append(report.Clusters, cv)
	}

	sort.Slice(report.Clusters, func(i, j int) bool {
		return report.Clusters[i].Cluster < report.Clusters[j].Cluster
	})
	for _, c := range report.Clusters {
		if c.MaxSkew > 0 {
			report.Totals.Skewed++
		}
		if c.SupportTypeRisk {
			report.Totals.SupportTypeRisk++
		}
	}
	report.Totals.Clusters = len(report.Clusters)
	report.Totals.DistinctVersions = len(report.VersionCounts)
	report.Filter = snap.Filter
	return report
}

func printVersionsTable(cmd *cobra.Command, r versionsReport) {
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "CLUSTER\tENV\tVERSION\tSUPPORT\tMODE\tNODEGROUPS\tMAX SKEW\tADDONS")
	for _, c := range r.Clusters {
		mode := "classic"
		if c.AutoMode {
			mode = "auto"
		}
		skew := "-"
		if c.MaxSkew > 0 {
			skew = fmt.Sprintf("%d behind", c.MaxSkew)
		}
		support := dash(c.SupportType)
		if c.SupportTypeRisk {
			support += " (!)"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%d\t%s\t%d\n",
			c.Cluster, dash(c.Environment), dash(c.Version), support, mode,
			len(c.NodeGroups), skew, len(c.Addons))
	}
	_ = tw.Flush()

	out := cmd.OutOrStdout()
	if len(r.VersionCounts) > 0 {
		versions := make([]string, 0, len(r.VersionCounts))
		for v := range r.VersionCounts {
			versions = append(versions, v)
		}
		sort.Slice(versions, func(i, j int) bool {
			a, aok := eks.ParseVersion(versions[i])
			b, bok := eks.ParseVersion(versions[j])
			if aok && bok {
				return a.Compare(b) > 0
			}
			return versions[i] > versions[j]
		})
		var parts []string
		for _, v := range versions {
			parts = append(parts, fmt.Sprintf("%s x%d", v, r.VersionCounts[v]))
		}
		fprintln(out, "\nversions: "+strings.Join(parts, ", "))
	}
	fprintln(out, fmt.Sprintf("%d clusters, %d distinct versions, %d with node-group skew, %d pinned under STANDARD support",
		r.Totals.Clusters, r.Totals.DistinctVersions, r.Totals.Skewed, r.Totals.SupportTypeRisk))
}
