// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"encoding/base64"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/confighub/sdk/core/cubapi"
	goclientnew "github.com/confighub/sdk/core/openapi/goclient-new"

	"github.com/confighub/examples/network-policy-manager/internal/cub"
	"github.com/confighub/examples/network-policy-manager/internal/netpol"
	"github.com/confighub/examples/network-policy-manager/internal/snapshot"
)

// unitManagedByLabel marks Units this tool authors.
const unitManagedByLabel = "cub-netpol"

// createDest is the resolved Space (and Target) a generated policy Unit should
// be created in, so it sits alongside the workloads it governs and deploys to
// the same cluster.
type createDest struct {
	spaceID    goclientnew.UUID
	spaceSlug  string
	targetID   *goclientnew.UUID
	targetSlug string
	cluster    string
}

// resolveCreateDest infers the Space/Target for a namespace from the managed
// workloads in it. It requires the namespace to contain at least one managed
// workload, and a single Space (use --cluster / --space to disambiguate).
func resolveCreateDest(snap *snapshot.Snapshot, namespace, clusterFilter, spaceOverride string) (createDest, error) {
	type cand struct{ cluster, spaceID, spaceSlug, targetID, targetSlug string }
	var cands []cand
	seenSpace := map[string]bool{}
	for clusterName, c := range snap.Clusters {
		if clusterFilter != "" && clusterName != clusterFilter {
			continue
		}
		for _, w := range c.Workloads {
			if netpol.NamespaceOf(w.Namespace) != namespace {
				continue
			}
			meta, ok := snap.Units[w.Origin.UnitID]
			if !ok {
				continue
			}
			if spaceOverride != "" && meta.SpaceSlug != spaceOverride {
				continue
			}
			if seenSpace[meta.SpaceID] {
				continue
			}
			seenSpace[meta.SpaceID] = true
			cands = append(cands, cand{clusterName, meta.SpaceID, meta.SpaceSlug, meta.TargetID, meta.TargetSlug})
		}
	}
	if len(cands) == 0 {
		return createDest{}, fmt.Errorf("no managed workloads found in namespace %q%s — cannot infer the Space to create the policy in (try --space, or --cluster)",
			namespace, clusterNote(clusterFilter, spaceOverride))
	}
	clusters := map[string]bool{}
	for _, c := range cands {
		clusters[c.cluster] = true
	}
	if clusterFilter == "" && len(clusters) > 1 {
		return createDest{}, fmt.Errorf("namespace %q exists in multiple clusters (%s); specify --cluster", namespace, strings.Join(sortedKeys(clusters), ", "))
	}
	if len(cands) > 1 {
		slugs := make([]string, 0, len(cands))
		for _, c := range cands {
			slugs = append(slugs, c.spaceSlug)
		}
		sort.Strings(slugs)
		return createDest{}, fmt.Errorf("namespace %q spans multiple Spaces (%s); specify --space", namespace, strings.Join(slugs, ", "))
	}

	c := cands[0]
	sid, err := uuid.Parse(c.spaceID)
	if err != nil {
		return createDest{}, fmt.Errorf("parse space id %q: %w", c.spaceID, err)
	}
	dest := createDest{spaceID: sid, spaceSlug: c.spaceSlug, targetSlug: c.targetSlug, cluster: c.cluster}
	if c.targetID != "" {
		tid, err := uuid.Parse(c.targetID)
		if err != nil {
			return createDest{}, fmt.Errorf("parse target id %q: %w", c.targetID, err)
		}
		dest.targetID = &tid
	}
	return dest, nil
}

// createPlan is the structured result of a create command (dry-run or commit).
type createPlan struct {
	Action    string `json:"action"`
	DryRun    bool   `json:"dryRun"`
	Space     string `json:"space"`
	Target    string `json:"target,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Unit      string `json:"unit"`
	UnitID    string `json:"unitId,omitempty"`
	Revision  int64  `json:"revision,omitempty"`
	Manifest  string `json:"manifest"`
}

// runCreate executes the dry-run/commit flow shared by the create commands.
func runCreate(cmd *cobra.Command, client *cubapi.Client, dest createDest, slug, manifest, namespace string, dryRun bool, changeDesc, output string) error {
	plan := createPlan{
		Action: "create-unit", DryRun: dryRun, Space: dest.spaceSlug, Target: dest.targetSlug,
		Namespace: namespace, Unit: slug, Manifest: manifest,
	}
	out := cmd.OutOrStdout()
	if dryRun {
		if output == outputJSON {
			return printJSON(out, plan)
		}
		fprintln(out, fmt.Sprintf("Dry run — would create Unit %s/%s (target %s) for namespace %q:",
			dest.spaceSlug, slug, orDash(dest.targetSlug), namespace))
		fprintln(out, "")
		fprintln(out, manifest)
		fprintln(out, "Re-run with --commit --change-desc \"…\" to create the Unit. It is not applied to the cluster until you apply it.")
		return nil
	}

	created, err := cub.CreateUnit(cmd.Context(), client, buildUnit(dest, slug, manifest, changeDesc))
	if err != nil {
		return err
	}
	plan.UnitID = created.UnitID.String()
	plan.Revision = created.HeadRevisionNum
	if output == outputJSON {
		return printJSON(out, plan)
	}
	fprintln(out, fmt.Sprintf("Created Unit %s/%s (revision %d). Apply it to deploy: cub unit apply --space %s %s",
		dest.spaceSlug, slug, created.HeadRevisionNum, dest.spaceSlug, slug))
	return nil
}

// buildUnit assembles the Unit body for creation. The API expects Unit.Data
// base64-encoded (matching `cub unit create`).
func buildUnit(dest createDest, slug, manifest, changeDesc string) goclientnew.Unit {
	u := goclientnew.Unit{
		Slug:                  slug,
		DisplayName:           slug,
		Data:                  base64.StdEncoding.EncodeToString([]byte(manifest)),
		ToolchainType:         "Kubernetes/YAML",
		SpaceID:               dest.spaceID,
		Labels:                map[string]string{"managed-by": unitManagedByLabel},
		LastChangeDescription: changeDesc,
	}
	if dest.targetID != nil {
		u.TargetID = dest.targetID
	}
	return u
}

// destFromUnitMeta builds a createDest from an existing Unit's metadata — used
// when the target Space/Target are already known (e.g. the producer Unit a Link
// points to), so no namespace inference is needed.
func destFromUnitMeta(meta snapshot.UnitMeta) (createDest, error) {
	sid, err := uuid.Parse(meta.SpaceID)
	if err != nil {
		return createDest{}, fmt.Errorf("parse space id %q: %w", meta.SpaceID, err)
	}
	dest := createDest{spaceID: sid, spaceSlug: meta.SpaceSlug, targetSlug: meta.TargetSlug}
	if meta.TargetID != "" {
		tid, err := uuid.Parse(meta.TargetID)
		if err != nil {
			return createDest{}, fmt.Errorf("parse target id %q: %w", meta.TargetID, err)
		}
		dest.targetID = &tid
	}
	return dest, nil
}

func clusterNote(clusterFilter, spaceOverride string) string {
	var parts []string
	if clusterFilter != "" {
		parts = append(parts, "cluster "+clusterFilter)
	}
	if spaceOverride != "" {
		parts = append(parts, "space "+spaceOverride)
	}
	if len(parts) == 0 {
		return ""
	}
	return " (" + strings.Join(parts, ", ") + ")"
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
