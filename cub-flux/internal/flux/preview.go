package flux

import (
	"path/filepath"
	"sort"
	"strings"
)

const previewAPIVersion = "confighub.com/plugin-preview/v1"

// PreviewEnvelope is the stable, read-only UI projection of a Flux plan.
// It intentionally excludes account credentials and plan-only implementation
// fields such as repoRoot, source URLs, and handover settings.
type PreviewEnvelope struct {
	APIVersion   string          `json:"apiVersion"`
	Kind         string          `json:"kind"`
	Producer     PreviewProducer `json:"producer"`
	ExportedAt   string          `json:"exportedAt"`
	Scope        PreviewScope    `json:"scope"`
	Capabilities []string        `json:"capabilities"`
	Inventory    PreviewGraph    `json:"inventory"`
	Proposal     PreviewGraph    `json:"proposal"`
	Issues       []PreviewIssue  `json:"issues"`
}

type PreviewProducer struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type PreviewScope struct {
	Description string `json:"description"`
}

type PreviewGraph struct {
	Nodes []PreviewNode `json:"nodes"`
	Edges []PreviewEdge `json:"edges"`
}

type PreviewNode struct {
	ID        string            `json:"id"`
	Kind      string            `json:"kind"`
	Name      string            `json:"name"`
	Namespace string            `json:"namespace,omitempty"`
	Details   map[string]string `json:"details,omitempty"`
	Hub       *PreviewHub       `json:"hub,omitempty"`
}

type PreviewHub struct {
	SpaceSlug    string `json:"spaceSlug,omitempty"`
	UnitSlug     string `json:"unitSlug,omitempty"`
	TargetSlug   string `json:"targetSlug,omitempty"`
	TargetsSpace string `json:"targetsSpace,omitempty"`
}

type PreviewEdge struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Relation string `json:"relation"`
}

type PreviewIssue struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

const previewScope = "Static cub flux plan derived from the supplied Git fleet repository. Inventory nodes describe plan inputs only; this is not live cluster discovery, rendered Kubernetes inventory, or proof of delivery."

// PreviewPlan converts the actual planner result into plugin-preview v1.
// exportedAt is the export time, not an observation time. The caller supplies
// the producer version so release builds can use the same value as `version`.
func PreviewPlan(plan *Plan, version, exportedAt string) PreviewEnvelope {
	preview := PreviewEnvelope{
		APIVersion:   previewAPIVersion,
		Kind:         "PluginPreview",
		Producer:     PreviewProducer{Name: "cub-flux", Version: version},
		ExportedAt:   exportedAt,
		Scope:        PreviewScope{Description: previewScope},
		Capabilities: []string{"preview"},
		Inventory:    PreviewGraph{Nodes: []PreviewNode{}, Edges: []PreviewEdge{}},
		Proposal:     PreviewGraph{Nodes: []PreviewNode{}, Edges: []PreviewEdge{}},
		Issues:       []PreviewIssue{},
	}
	if plan == nil {
		return preview
	}

	clusterIDs := make(map[string]string, len(plan.Clusters))
	for _, cluster := range plan.Clusters {
		id := "flux-cluster/" + cluster.Name
		clusterIDs[cluster.Name] = id
		preview.Inventory.Nodes = append(preview.Inventory.Nodes, PreviewNode{
			ID: id, Kind: "FluxCluster", Name: cluster.Name,
			Details: map[string]string{
				"stage":      cluster.Stage,
				"sourcePath": relativeClusterPath(cluster, plan.Inputs.RepoRoot),
				"evidence":   "Git plan input",
			},
		})
	}

	baseSpaces := map[string]bool{}
	variantSpaces := map[string]bool{}
	targetSpaces := map[string]bool{}
	targets := map[string]bool{}
	for _, component := range plan.Components {
		if component == nil {
			continue
		}
		for _, stage := range component.Stages {
			for _, variant := range stage.Variants {
				kustomizationName := lastPathPart(variant.Kustomization)
				if kustomizationName == "" {
					kustomizationName = component.Name
				}
				kustomizationID := "flux-kustomization/" + variant.Cluster + "/" + kustomizationName
				preview.Inventory.Nodes = append(preview.Inventory.Nodes, PreviewNode{
					ID: kustomizationID, Kind: "FluxKustomization", Name: kustomizationName, Namespace: "flux-system",
					Details: map[string]string{
						"component":  component.Name,
						"stage":      stage.Name,
						"sourcePath": variant.Path,
						"departures": strings.Join(variant.Departures, "; "),
						"evidence":   "Git plan input; not proof of live reconciliation",
					},
				})
				if clusterID := clusterIDs[variant.Cluster]; clusterID != "" {
					preview.Inventory.Edges = append(preview.Inventory.Edges, PreviewEdge{From: clusterID, To: kustomizationID, Relation: "declares"})
				}

				if !baseSpaces[component.Base] {
					baseSpaces[component.Base] = true
					baseSpaceID := "proposal-space/" + component.Base
					baseUnitID := "proposal-unit/" + component.Base + "/" + component.Name
					preview.Proposal.Nodes = append(preview.Proposal.Nodes,
						PreviewNode{ID: baseSpaceID, Kind: "Space", Name: component.Base, Details: map[string]string{"role": "base"}, Hub: &PreviewHub{SpaceSlug: component.Base}},
						PreviewNode{ID: baseUnitID, Kind: "Unit", Name: component.Name, Details: map[string]string{"role": "base", "sourcePath": component.BaseDir}, Hub: &PreviewHub{SpaceSlug: component.Base, UnitSlug: component.Name}},
					)
					preview.Proposal.Edges = append(preview.Proposal.Edges, PreviewEdge{From: baseSpaceID, To: baseUnitID, Relation: "contains"})
				}

				variantSpace := variant.Space
				targetSpace, targetSlug := splitTarget(variant.Target, variant.Cluster)
				variantSpaceID := "proposal-space/" + variantSpace
				if !variantSpaces[variantSpace] {
					variantSpaces[variantSpace] = true
					preview.Proposal.Nodes = append(preview.Proposal.Nodes, PreviewNode{
						ID: variantSpaceID, Kind: "Space", Name: variantSpace,
						Details: map[string]string{"role": "variant", "cluster": variant.Cluster, "stage": stage.Name},
						Hub:     &PreviewHub{SpaceSlug: variantSpace, TargetSlug: targetSlug, TargetsSpace: targetSpace},
					})
				}
				targetSpaceID := "proposal-space/" + targetSpace
				if !targetSpaces[targetSpace] {
					targetSpaces[targetSpace] = true
					preview.Proposal.Nodes = append(preview.Proposal.Nodes, PreviewNode{
						ID: targetSpaceID, Kind: "Space", Name: targetSpace,
						Details: map[string]string{"role": "targets"}, Hub: &PreviewHub{SpaceSlug: targetSpace},
					})
				}
				targetID := "proposal-target/" + targetSpace + "/" + targetSlug
				if !targets[targetID] {
					targets[targetID] = true
					preview.Proposal.Nodes = append(preview.Proposal.Nodes, PreviewNode{
						ID: targetID, Kind: "Target", Name: targetSlug,
						Details: map[string]string{"cluster": variant.Cluster},
						Hub:     &PreviewHub{SpaceSlug: targetSpace, TargetSlug: targetSlug},
					})
					preview.Proposal.Edges = append(preview.Proposal.Edges, PreviewEdge{From: targetSpaceID, To: targetID, Relation: "registers"})
				}
				variantUnitID := "proposal-unit/" + variantSpace + "/" + component.Name
				preview.Proposal.Nodes = append(preview.Proposal.Nodes, PreviewNode{
					ID: variantUnitID, Kind: "Unit", Name: component.Name,
					Details: map[string]string{"role": "variant", "cluster": variant.Cluster, "stage": stage.Name, "sourcePath": variant.Path},
					Hub:     &PreviewHub{SpaceSlug: variantSpace, UnitSlug: component.Name},
				})
				preview.Proposal.Edges = append(preview.Proposal.Edges,
					PreviewEdge{From: variantSpaceID, To: variantUnitID, Relation: "contains"},
					PreviewEdge{From: variantUnitID, To: "proposal-unit/" + component.Base + "/" + component.Name, Relation: "variantOf"},
					PreviewEdge{From: variantSpaceID, To: targetID, Relation: "targets"},
				)
			}
		}
	}

	for _, skipped := range plan.Inputs.Skipped {
		preview.Issues = append(preview.Issues, PreviewIssue{Severity: "warning", Code: "flux-input-skipped", Message: skipped})
	}
	for _, problem := range plan.Problems {
		preview.Issues = append(preview.Issues, PreviewIssue{Severity: "error", Code: "flux-plan-problem", Message: problem})
	}
	sort.Slice(preview.Inventory.Nodes, func(i, j int) bool { return preview.Inventory.Nodes[i].ID < preview.Inventory.Nodes[j].ID })
	sort.Slice(preview.Proposal.Nodes, func(i, j int) bool { return preview.Proposal.Nodes[i].ID < preview.Proposal.Nodes[j].ID })
	sort.Slice(preview.Inventory.Edges, func(i, j int) bool { return edgeKey(preview.Inventory.Edges[i]) < edgeKey(preview.Inventory.Edges[j]) })
	sort.Slice(preview.Proposal.Edges, func(i, j int) bool { return edgeKey(preview.Proposal.Edges[i]) < edgeKey(preview.Proposal.Edges[j]) })
	sort.Slice(preview.Issues, func(i, j int) bool {
		left := preview.Issues[i].Code + "\x00" + preview.Issues[i].Message
		right := preview.Issues[j].Code + "\x00" + preview.Issues[j].Message
		return left < right
	})
	return preview
}

func edgeKey(edge PreviewEdge) string { return edge.From + "\x00" + edge.To + "\x00" + edge.Relation }

func lastPathPart(path string) string {
	path = strings.TrimRight(filepath.ToSlash(path), "/")
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[i+1:]
	}
	return path
}

func splitTarget(target, fallback string) (space, slug string) {
	parts := strings.Split(strings.Trim(target, "/"), "/")
	if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
		return parts[0], parts[1]
	}
	return "flux-targets", fallback
}

func relativeClusterPath(cluster Cluster, repoRoot string) string {
	path := filepath.FromSlash(cluster.Path)
	if filepath.IsAbs(path) {
		root := filepath.FromSlash(repoRoot)
		if root != "" {
			if !filepath.IsAbs(root) {
				if absolute, err := filepath.Abs(root); err == nil {
					root = absolute
				}
			}
			if relative, err := filepath.Rel(root, path); err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
				return filepath.ToSlash(relative)
			}
		}
		return filepath.ToSlash(filepath.Join("clusters", cluster.Dir))
	}
	return filepath.ToSlash(path)
}
