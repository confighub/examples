package flux

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

const previewExportedAt = "2026-09-30T00:00:00Z"

func TestExpertFleetPreviewContract(t *testing.T) {
	root := repoRoot(t)
	p := planOf(t, example, root)
	preview := PreviewPlan(p, "dev", previewExportedAt)
	encoded, err := json.Marshal(preview)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatal(err)
	}
	if got["apiVersion"] != "confighub.com/plugin-preview/v1" || got["kind"] != "PluginPreview" {
		t.Fatalf("preview identity = %v / %v", got["apiVersion"], got["kind"])
	}
	if got["exportedAt"] != previewExportedAt {
		t.Fatalf("exportedAt = %v", got["exportedAt"])
	}
	producer := got["producer"].(map[string]any)
	if producer["name"] != "cub-flux" || producer["version"] != "dev" {
		t.Fatalf("producer = %v", producer)
	}
	if got["capabilities"].([]any)[0] != "preview" {
		t.Fatalf("capabilities = %v", got["capabilities"])
	}
	if len(preview.Inventory.Nodes) != 13 || len(preview.Inventory.Edges) != 10 {
		t.Errorf("inventory has %d nodes and %d edges; want 13 and 10", len(preview.Inventory.Nodes), len(preview.Inventory.Edges))
	}
	if len(preview.Proposal.Nodes) != 32 || len(preview.Proposal.Edges) != 37 {
		t.Errorf("proposal has %d nodes and %d edges; want 32 and 37", len(preview.Proposal.Nodes), len(preview.Proposal.Edges))
	}
	var registrations, deliveries int
	for _, edge := range preview.Proposal.Edges {
		if strings.HasPrefix(edge.From, "proposal-space/flux-targets") && strings.HasPrefix(edge.To, "proposal-target/") {
			if edge.Relation == "registers" {
				registrations++
			} else {
				t.Errorf("targets-space to target relation = %q; want registers", edge.Relation)
			}
		}
		if strings.HasPrefix(edge.From, "proposal-space/flux-") && edge.From != "proposal-space/flux-targets" && strings.HasPrefix(edge.To, "proposal-target/") {
			if edge.Relation == "targets" {
				deliveries++
			} else {
				t.Errorf("variant-space to target relation = %q; want targets", edge.Relation)
			}
		}
	}
	if registrations != 3 || deliveries != 10 {
		t.Errorf("target registrations/deliveries = %d/%d; want 3/10", registrations, deliveries)
	}
	if len(preview.Issues) != 0 {
		t.Errorf("clean expert fleet has issues: %+v", preview.Issues)
	}
	if !strings.Contains(preview.Scope.Description, "not live cluster discovery") || !strings.Contains(preview.Scope.Description, "or proof of delivery") {
		t.Errorf("scope does not bound claims: %q", preview.Scope.Description)
	}
	for _, graph := range []PreviewGraph{preview.Inventory, preview.Proposal} {
		assertPreviewGraph(t, graph)
	}
	for _, node := range preview.Inventory.Nodes {
		if node.Kind == "FluxCluster" && filepath.IsAbs(node.Details["sourcePath"]) {
			t.Errorf("cluster source path leaked absolute host path: %q", node.Details["sourcePath"])
		}
	}
	serialized := string(encoded)
	for _, forbidden := range []string{"repoRoot", "CONFIGHUB_OCI", "PullSecret", "secretRef", "credentials"} {
		if strings.Contains(serialized, forbidden) {
			t.Errorf("preview unexpectedly contains %q", forbidden)
		}
	}
	if !strings.Contains(serialized, "1.26.3-alpine") || !strings.Contains(serialized, "production") {
		t.Error("preview lost expert-fleet variant evidence")
	}
}

func TestPreviewIsDeterministicExceptExportedAt(t *testing.T) {
	p := planOf(t, example, repoRoot(t))
	a := PreviewPlan(p, "1.2.3", previewExportedAt)
	b := PreviewPlan(p, "1.2.3", "2026-09-30T01:00:00Z")
	a.ExportedAt, b.ExportedAt = "", ""
	if !equalJSON(t, a, b) {
		t.Error("preview changed beyond exportedAt for identical plan and options")
	}
}

func TestPreviewSurfacesPartialPlanAndKeepsEmptyArrays(t *testing.T) {
	p := &Plan{
		Inputs:     Inputs{Skipped: []string{"ignored object"}},
		Clusters:   []Cluster{{Name: "dev", Dir: "dev", Stage: "dev", Path: "clusters/dev"}},
		Components: []*Component{},
		Problems:   []string{"layer path is missing"},
	}
	preview := PreviewPlan(p, "dev", previewExportedAt)
	if len(preview.Issues) != 2 || preview.Issues[0].Severity != "warning" || preview.Issues[1].Severity != "error" {
		t.Fatalf("partial plan issues = %+v", preview.Issues)
	}
	if preview.Proposal.Nodes == nil || preview.Proposal.Edges == nil || preview.Inventory.Edges == nil {
		t.Fatal("empty graph members must encode as arrays")
	}
	if preview.Proposal.Nodes == nil || len(preview.Proposal.Nodes) != 0 {
		t.Fatalf("empty proposal nodes = %#v", preview.Proposal.Nodes)
	}
}

func TestPreviewIgnoresUnmodeledSensitivePlanFields(t *testing.T) {
	p := planOf(t, example, repoRoot(t))
	p.Inputs.RepoRoot = "/private/host/path"
	p.Clusters[0].PullSecret = "credential-shaped-value"
	preview := PreviewPlan(p, "dev", previewExportedAt)
	data, err := json.Marshal(preview)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "private/host/path") || strings.Contains(string(data), "credential-shaped-value") {
		t.Fatalf("sensitive/unneeded plan detail was exported: %s", data)
	}
}

func assertPreviewGraph(t *testing.T, graph PreviewGraph) {
	t.Helper()
	ids := map[string]bool{}
	last := ""
	for _, node := range graph.Nodes {
		if node.ID == "" || node.Kind == "" || node.Name == "" {
			t.Errorf("node is missing a required field: %+v", node)
		}
		if ids[node.ID] {
			t.Errorf("duplicate node id %q", node.ID)
		}
		ids[node.ID] = true
		if last != "" && last > node.ID {
			t.Errorf("nodes are not sorted by id: %q before %q", last, node.ID)
		}
		last = node.ID
	}
	edges := map[string]bool{}
	last = ""
	for _, edge := range graph.Edges {
		if !ids[edge.From] || !ids[edge.To] || edge.Relation == "" {
			t.Errorf("edge references a missing node or relation: %+v", edge)
		}
		key := edge.From + "\x00" + edge.To + "\x00" + edge.Relation
		if edges[key] {
			t.Errorf("duplicate edge %q", key)
		}
		edges[key] = true
		if last != "" && last > key {
			t.Errorf("edges are not sorted: %q before %q", last, key)
		}
		last = key
	}
}

func equalJSON(t *testing.T, a, b any) bool {
	t.Helper()
	x, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	y, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	return string(x) == string(y)
}
