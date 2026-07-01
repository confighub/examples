// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/confighub/sdk/cliutil"
	"github.com/confighub/sdk/core/cubapi"
	goclientnew "github.com/confighub/sdk/core/openapi/goclient-new"

	"github.com/confighub/examples/network-policy-manager/internal/cub"
	"github.com/confighub/examples/network-policy-manager/internal/netpol"
	"github.com/confighub/examples/network-policy-manager/internal/snapshot"
)

// unitIndex maps a Unit ID to the pod-bearing resources it contains. A Unit may
// bundle several resources (e.g. a Deployment + Service, or a service and its
// Redis), so each list can hold more than one entry.
type unitIndex struct {
	workloads map[string][]*netpol.WorkloadEntity
	services  map[string][]*netpol.ServiceEntity
	cluster   map[string]string
}

// allowFromLink is one planned allow policy. In the default (consolidated) mode
// it is one policy per destination admitting all Sources; with --per-edge it is
// one policy per source→destination pair (Sources has a single entry).
type allowFromLink struct {
	Sources   []string `json:"sources"`
	To        string   `json:"to"`
	Namespace string   `json:"namespace"`
	Space     string   `json:"space"`
	Unit      string   `json:"unit"`
	manifest  string
	dest      createDest
	srcWork   []*netpol.WorkloadEntity
}

func newAllowFromLinksCmd() *cobra.Command {
	var output, spaceFilter, clusterFilter, port string
	var perEdge bool
	var filter filterFlags
	var commit cliutil.CommitFlags
	cmd := &cobra.Command{
		Use:   "allow-from-links",
		Short: "Generate allow NetworkPolicies from ConfigHub Links (the dependency graph)",
		Long: `allow-from-links reads ConfigHub Links — the producer/consumer dependency edges
between Units — and authors an ingress allow NetworkPolicy for each one: the
consumer (From) is admitted to the producer (To). Paired with a default-deny,
these restore exactly the intended pod-to-pod connectivity, derived from the
fleet's own dependency graph rather than guessed.

By default it produces one consolidated policy per destination workload (all
admitted sources as ingress from-peers) — the idiomatic shape. Pass --per-edge
for one policy per source->destination pair instead.

A Link points to a Unit, which may bundle several resources; the resource whose
name matches the Unit slug is targeted (e.g. a Link to "cartservice" targets the
cartservice Service, not a Redis bundled in the same Unit). A To Unit's Service
selector is used to target backing pods; Links to namespaces, selector-less
Services, or ambiguous multi-resource Units are skipped. Restrict with --space
and --cluster.

This is a dry run unless you pass --commit --change-desc "…".`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			summary := "generate allow NetworkPolicies from ConfigHub Links"
			changeDesc, dryRun, err := commit.Validate(summary)
			if err != nil {
				return err
			}
			client, err := cub.Preflight(cmd.Context())
			if err != nil {
				return err
			}
			snap, err := snapshot.Load(cmd.Context(), client, filter.predicate())
			if err != nil {
				return err
			}

			idx := buildUnitIndex(snap)
			links, err := cubapi.ListLinks(cmd.Context(), client, cubapi.NewWhere(""),
				cubapi.ListOpts{Include: "SpaceID,FromUnitID,ToUnitID,ToSpaceID"})
			if err != nil {
				return fmt.Errorf("list links: %w", err)
			}

			plans, err := planAllowsFromLinks(snap, links, idx, spaceFilter, clusterFilter, port, perEdge)
			if err != nil {
				return err
			}

			if dryRun {
				if output == outputJSON {
					return printJSON(cmd.OutOrStdout(), plans)
				}
				printAllowPlansTable(cmd, plans)
				return nil
			}
			return commitAllowPlans(cmd, client, snap, plans, changeDesc, output)
		},
	}
	addOutputFlag(cmd, &output)
	addFilterFlags(cmd, &filter)
	cmd.Flags().StringVar(&spaceFilter, "space", "", "only consider Links whose consumer (From) Unit is in this Space")
	cmd.Flags().StringVar(&clusterFilter, "cluster", "", "only consider Links in this cluster")
	cmd.Flags().StringVar(&port, "port", "", "restrict each rule to this port (numeric or named; protocol TCP)")
	cmd.Flags().BoolVar(&perEdge, "per-edge", false, "author one policy per source→destination pair instead of one consolidated policy per destination")
	commit.Bind(cmd)
	return cmd
}

func buildUnitIndex(snap *snapshot.Snapshot) unitIndex {
	idx := unitIndex{
		workloads: map[string][]*netpol.WorkloadEntity{},
		services:  map[string][]*netpol.ServiceEntity{},
		cluster:   map[string]string{},
	}
	for cluster, c := range snap.Clusters {
		for _, w := range c.Workloads {
			idx.workloads[w.Origin.UnitID] = append(idx.workloads[w.Origin.UnitID], w)
			idx.cluster[w.Origin.UnitID] = cluster
		}
		for _, s := range c.Services {
			idx.services[s.Origin.UnitID] = append(idx.services[s.Origin.UnitID], s)
			idx.cluster[s.Origin.UnitID] = cluster
		}
	}
	return idx
}

// dstGroup accumulates the sources that should be admitted to one destination.
type dstGroup struct {
	dst     *netpol.WorkloadEntity
	dest    createDest
	space   string
	sources []*netpol.WorkloadEntity
	seenSrc map[string]bool
}

func planAllowsFromLinks(snap *snapshot.Snapshot, links []*goclientnew.ExtendedLink, idx unitIndex, spaceFilter, clusterFilter, port string, perEdge bool) ([]allowFromLink, error) {
	// First resolve every Link edge to (source, destination) and group by
	// destination — the unit of consolidation.
	groups := map[string]*dstGroup{}
	var order []string
	for _, el := range links {
		if el == nil || el.Link == nil {
			continue
		}
		fromID := el.Link.FromUnitID.String()
		toID := el.Link.ToUnitID.String()
		fromSlug, toSlug := "", ""
		if el.FromUnit != nil {
			fromSlug = el.FromUnit.Slug
		}
		if el.ToUnit != nil {
			toSlug = el.ToUnit.Slug
		}

		src := pickWorkload(idx.workloads[fromID], fromSlug)
		if src == nil {
			continue // consumer is not a (single, identifiable) workload
		}
		if spaceFilter != "" {
			if meta, ok := snap.Units[fromID]; !ok || meta.SpaceSlug != spaceFilter {
				continue
			}
		}
		if clusterFilter != "" && idx.cluster[fromID] != clusterFilter {
			continue
		}
		dst := pickDst(idx.services[toID], idx.workloads[toID], toSlug)
		if dst == nil {
			continue // producer target not identifiable (namespace, headless, ambiguous)
		}
		if src.Name == dst.Name && netpol.NamespaceOf(src.Namespace) == netpol.NamespaceOf(dst.Namespace) {
			continue
		}
		meta, ok := snap.Units[toID]
		if !ok {
			continue
		}

		key := toID + "/" + dst.Name
		g := groups[key]
		if g == nil {
			dest, err := destFromUnitMeta(meta)
			if err != nil {
				return nil, err
			}
			g = &dstGroup{dst: dst, dest: dest, space: meta.SpaceSlug, seenSrc: map[string]bool{}}
			groups[key] = g
			order = append(order, key)
		}
		sk := netpol.NamespaceOf(src.Namespace) + "/" + src.Name
		if !g.seenSrc[sk] {
			g.seenSrc[sk] = true
			g.sources = append(g.sources, src)
		}
	}

	var plans []allowFromLink
	for _, key := range order {
		g := groups[key]
		if perEdge {
			for _, src := range g.sources {
				slug, manifest := netpol.AllowYAML(src, g.dst, false, port)
				plans = append(plans, allowFromLink{
					Sources: []string{src.Name}, To: g.dst.Name, Namespace: netpol.NamespaceOf(g.dst.Namespace),
					Space: g.space, Unit: slug, manifest: manifest, dest: g.dest,
					srcWork: []*netpol.WorkloadEntity{src},
				})
			}
			continue
		}
		slug, manifest := netpol.AllowIngressYAML(g.dst, g.sources, port)
		names := make([]string, 0, len(g.sources))
		for _, s := range g.sources {
			names = append(names, s.Name)
		}
		sort.Strings(names)
		plans = append(plans, allowFromLink{
			Sources: names, To: g.dst.Name, Namespace: netpol.NamespaceOf(g.dst.Namespace),
			Space: g.space, Unit: slug, manifest: manifest, dest: g.dest,
			srcWork: g.sources,
		})
	}
	sort.Slice(plans, func(i, j int) bool {
		if plans[i].Space != plans[j].Space {
			return plans[i].Space < plans[j].Space
		}
		return plans[i].Unit < plans[j].Unit
	})
	return plans, nil
}

// pickWorkload selects the workload a Link's From Unit refers to: the one whose
// name matches the Unit slug, or the sole workload if the Unit has exactly one.
// Returns nil when the Unit has several workloads and none matches the slug
// (ambiguous), or none at all.
func pickWorkload(ws []*netpol.WorkloadEntity, slug string) *netpol.WorkloadEntity {
	if slug != "" {
		for _, w := range ws {
			if w.Name == slug {
				return w
			}
		}
	}
	if len(ws) == 1 {
		return ws[0]
	}
	return nil
}

// pickDst selects the producer target a Link's To Unit refers to. It prefers a
// Service (the ingress surface) — by slug match, then the sole Service — and
// falls back to a workload by the same rule. A Service target is represented as
// a synthetic workload whose pod labels are the Service selector.
func pickDst(svcs []*netpol.ServiceEntity, wls []*netpol.WorkloadEntity, slug string) *netpol.WorkloadEntity {
	if slug != "" {
		for _, s := range svcs {
			if s.Name == slug && len(s.Selector) > 0 {
				return serviceAsWorkload(s)
			}
		}
		for _, w := range wls {
			if w.Name == slug {
				return w
			}
		}
	}
	if len(svcs) == 1 && len(svcs[0].Selector) > 0 {
		return serviceAsWorkload(svcs[0])
	}
	if len(svcs) == 0 && len(wls) == 1 {
		return wls[0]
	}
	return nil
}

func serviceAsWorkload(s *netpol.ServiceEntity) *netpol.WorkloadEntity {
	return &netpol.WorkloadEntity{Kind: "Service", Name: s.Name, Namespace: s.Namespace, PodLabels: s.Selector}
}

func printAllowPlansTable(cmd *cobra.Command, plans []allowFromLink) {
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "SPACE\tNAMESPACE\tTO\tFROM (sources)\tUNIT")
	for _, p := range plans {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", p.Space, p.Namespace, p.To, strings.Join(p.Sources, ", "), p.Unit)
	}
	_ = tw.Flush()
	fprintln(cmd.OutOrStdout(), fmt.Sprintf("\n%d allow policies would be created from Links. Re-run with --commit --change-desc \"…\" to create them.", len(plans)))
}

// commitAllowPlans creates a new allow Unit per plan, or — when the destination
// policy already exists (a re-run / consolidation upsert) — adds only the source
// peers it is missing via set-yq, leaving existing peers untouched. A plan whose
// policy exists with all sources already present is a no-op.
func commitAllowPlans(cmd *cobra.Command, client *cubapi.Client, snap *snapshot.Snapshot, plans []allowFromLink, changeDesc, output string) error {
	type result struct {
		Space  string `json:"space"`
		Unit   string `json:"unit"`
		Action string `json:"action"` // created | updated | unchanged | error
		Added  int    `json:"added,omitempty"`
		Error  string `json:"error,omitempty"`
	}
	var results []result
	created, updated, unchanged := 0, 0, 0
	for _, p := range plans {
		existing := findPolicy(snap, p.Space, p.Unit)
		r := result{Space: p.Space, Unit: p.Unit}
		switch {
		case existing == nil:
			if _, err := cub.CreateUnit(cmd.Context(), client, buildUnit(p.dest, p.Unit, p.manifest, changeDesc)); err != nil {
				r.Action, r.Error = "error", err.Error()
			} else {
				r.Action = "created"
				created++
			}
		default:
			newPeers := missingPeers(existing, p.srcWork)
			if len(newPeers) == 0 {
				r.Action = "unchanged"
				unchanged++
				break
			}
			expr := ".spec.ingress[0].from += [" + strings.Join(newPeers, ", ") + "]"
			if _, err := cub.MutateUnitYQ(cmd.Context(), client, p.dest.spaceID, p.Unit, expr, cubapi.Change{Description: changeDesc}); err != nil {
				r.Action, r.Error = "error", err.Error()
			} else {
				r.Action, r.Added = "updated", len(newPeers)
				updated++
			}
		}
		results = append(results, r)
	}
	if output == outputJSON {
		return printJSON(cmd.OutOrStdout(), results)
	}
	out := cmd.OutOrStdout()
	for _, r := range results {
		detail := r.Error
		if r.Action == "updated" {
			detail = fmt.Sprintf("+%d source(s)", r.Added)
		}
		fprintln(out, fmt.Sprintf("%-9s %s/%s %s", r.Action, r.Space, r.Unit, detail))
	}
	fprintln(out, fmt.Sprintf("\n%d created, %d updated, %d unchanged (not applied; apply when ready).", created, updated, unchanged))
	return nil
}

// missingPeers returns the from-peer JSON for each source not already admitted
// by the existing policy's ingress rules.
func missingPeers(existing *netpol.NetworkPolicyEntity, sources []*netpol.WorkloadEntity) []string {
	have := map[string]bool{}
	for _, rule := range existing.Ingress {
		for _, peer := range rule.Peers {
			if peer.PodSelector != nil {
				have[netpol.CanonicalLabels(peer.PodSelector.MatchLabels)] = true
			}
		}
	}
	var out []string
	for _, src := range sources {
		if !have[netpol.CanonicalLabels(src.PodLabels)] {
			out = append(out, netpol.FromPeerJSON(src))
		}
	}
	return out
}
