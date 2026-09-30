package flux

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// handedOver copies a fleet and makes one cluster's directory look the way a
// bootstrapped handover leaves it: the layer files gone, the root in.
func handedOver(t *testing.T, fleet, clusterDir, layersSpace string, keep ...string) string {
	t.Helper()
	root := t.TempDir()
	src := filepath.Join(repoRoot(t), filepath.FromSlash(fleet))
	dst := filepath.Join(root, filepath.FromSlash(fleet))
	if err := os.CopyFS(dst, os.DirFS(src)); err != nil {
		t.Fatal(err)
	}
	cdir := filepath.Join(dst, "clusters", clusterDir)
	entries, err := os.ReadDir(cdir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") || e.Name() == "kustomization.yaml" {
			continue
		}
		kept := false
		for _, k := range keep {
			kept = kept || e.Name() == k
		}
		if !kept {
			if err := os.Remove(filepath.Join(cdir, e.Name())); err != nil {
				t.Fatal(err)
			}
		}
	}
	// A root as made before roots named their cluster: the plan reads which
	// cluster it is from the Space's name.
	rootYAML := strings.NewReplacer(gatewayMarker, "gw.example:5000", insecureMarker, "false",
		DeliverySpace("flux", "__CLUSTER__"), layersSpace, "  labels:\n    "+RootClusterLabel+": __CLUSTER__\n", "").Replace(RootManifests("flux", "__CLUSTER__"))
	if err := os.WriteFile(filepath.Join(cdir, RootName+".yaml"), []byte(rootYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func planAt(t *testing.T, root, fleet string) *Plan {
	t.Helper()
	in, err := Load(nil, []string{filepath.Join(root, filepath.FromSlash(fleet))})
	if err != nil {
		t.Fatal(err)
	}
	p, err := Build(in, Options{Prefix: "flux", RepoRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// Found on the live bootstrapped run: after the handover commit, the plan
// took the root for a layer, with a base rendered from the repository root.
func TestHandedOverClusterIsNotReplanned(t *testing.T) {
	const fleet = "gitops/flux/beginner"
	root := handedOver(t, fleet, "dev", "flux-dev-layers")
	p := planAt(t, root, fleet)
	if len(p.Problems) > 0 {
		t.Fatalf("a finished handover is not a problem: %v", p.Problems)
	}
	var dev *Cluster
	for i := range p.Clusters {
		if p.Clusters[i].Name == "dev" {
			dev = &p.Clusters[i]
		}
	}
	if dev == nil || dev.LayersSpace != "flux-dev-layers" {
		t.Fatalf("dev should be kept, marked handed over: %+v", p.Clusters)
	}
	for _, c := range p.Components {
		if c.Name == RootName {
			t.Errorf("the root is not a layer")
		}
		for _, n := range c.Notes {
			if strings.HasPrefix(n, "only on") {
				t.Errorf("dev runs %s too, from ConfigHub: %q", c.Name, n)
			}
		}
		for _, st := range c.Stages {
			for _, v := range st.Variants {
				if v.Cluster == "dev" {
					t.Errorf("dev's variants are ConfigHub's now, not re-planned: %s", v.Space)
				}
			}
		}
	}
	if !strings.Contains(Render(p), "dev is handed over: its layers are Units in flux-dev-layers") {
		t.Errorf("the plan should say so:\n%s", Render(p))
	}
	h := HandoverScript(p, "flux", ".")
	if !strings.Contains(h, `[ "$cluster" != 'dev' ] || { echo "dev is handed over already`) {
		t.Error("handover.sh should stop for a cluster already handed over")
	}
}

// The expert fleet names its clusters through cluster_name in the layers; with
// the layers gone, the root's Space is what still says dev-1.
func TestHandedOverClusterKeepsItsName(t *testing.T) {
	const fleet = "gitops/flux/expert-fleet"
	p := planAt(t, handedOver(t, fleet, "dev", "flux-dev-1-layers"), fleet)
	found := false
	for _, c := range p.Clusters {
		found = found || (c.Name == "dev-1" && c.LayersSpace == "flux-dev-1-layers")
	}
	if !found {
		t.Errorf("want dev-1, handed over, got %+v", p.Clusters)
	}
}

// A root beside layer files still in Git is a commit half made.
func TestHalfMadeHandoverCommitIsAProblem(t *testing.T) {
	const fleet = "gitops/flux/beginner"
	p := planAt(t, handedOver(t, fleet, "dev", "flux-dev-layers", "apps.yaml"), fleet)
	if len(p.Problems) != 1 || !strings.Contains(p.Problems[0], "half made") || !strings.Contains(p.Problems[0], "apps") {
		t.Errorf("want one problem naming apps: %v", p.Problems)
	}
}

func TestChecksFromLayersSpace(t *testing.T) {
	unit := `apiVersion: source.toolkit.fluxcd.io/v1
kind: OCIRepository
metadata: {name: apps, namespace: flux-system}
spec: {url: "oci://gw.example:5000/space/flux-apps-dev"}
---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata: {name: apps, namespace: flux-system}
spec: {targetNamespace: apptique-dev, path: ./}
`
	run := fake(map[string]string{
		"unit list":                               `[{"Unit":{"Slug":"apps"}},{"Unit":{"Slug":"unreleased"}}]`,
		"release get":                             `{"Release":{"ReleaseNum":4,"ManifestDigest":"sha256:l4","TagID":"t4"}}`,
		"list --space flux-dev-layers apps":       `[{"Revision":{"RevisionNum":2}}]`,
		"list --space flux-dev-layers unreleased": `[]`,
		"revision data":                           unit,
		"unit get":                                `{"Unit":{"HeadRevisionNum":3}}`,
	})
	checks, err := ChecksFromLayersSpace(run, "dev", "flux-dev-layers")
	if err != nil {
		t.Fatal(err)
	}
	want := Check{Kustomization: "apps", Space: "flux-apps-dev", Unit: "apps", Namespace: "apptique-dev", Cluster: "dev"}
	if len(checks) != 1 || checks[0] != want {
		t.Errorf("want %+v, got %+v", want, checks)
	}
	if _, err := ChecksFromLayersSpace(fake(map[string]string{"unit list": `[]`, "release get": `{"Release":{"ReleaseNum":1,"ManifestDigest":"sha256:x","TagID":"t"}}`}), "dev", "flux-dev-layers"); err == nil {
		t.Error("an empty layers Space should say so")
	}
}

// From review on #256: after a handover, re-running apply.sh must not rewrite
// a shared workflow from a view that no longer shows the handed-over stage.
func TestApplyLeavesWorkflowsAloneAfterAHandover(t *testing.T) {
	const fleet = "gitops/flux/beginner"
	root := handedOver(t, fleet, "dev", "flux-dev-layers")
	in, err := Load(nil, []string{filepath.Join(root, fleet)})
	if err != nil {
		t.Fatal(err)
	}
	p, err := Build(in, Options{Prefix: "flux", RepoRoot: root, Require: []string{"Healthy"}})
	if err != nil {
		t.Fatal(err)
	}
	s := ApplyScript(p, "flux", ".")
	if strings.Contains(s, "\nstages_are ") || strings.Contains(s, "rollout --filename apps/change-workflow.yaml --quiet") {
		t.Errorf("no workflow may be rewritten or replaced while a cluster is handed over")
	}
	// A stage the plan has and the workflow may lack is only ever added, after
	// the nearest earlier stage in the fleet's order, the handed-over one too.
	if !strings.Contains(s, "add_stage flux-apps-base 'prod' 'dev' ") {
		t.Errorf("prod should be added after dev if missing, keeping dev:\n%s", s)
	}
	// And the handed-over cluster's stage is still promoted, so later stages
	// find it has taken the order.
	if !strings.Contains(s, "if cub space get flux-apps-dev >/dev/null 2>&1; then  # dev is handed over") {
		t.Errorf("dev's existing variant should still take the order")
	}
}

// Found live: after a handover, cleanup.sh left the handed-over cluster's
// variant Spaces, whose releases hold tags in the bases, so the bases could
// not be deleted either.
func TestCleanupRemovesHandedOverVariants(t *testing.T) {
	const fleet = "gitops/flux/beginner"
	p := planAt(t, handedOver(t, fleet, "dev", "flux-dev-layers"), fleet)
	s := CleanupScript(p, "flux")
	v := strings.Index(s, "cub space get flux-apps-dev >/dev/null 2>&1 && { cub space delete flux-apps-dev")
	b := strings.Index(s, "cub space delete flux-apps-base ")
	if v < 0 || b < 0 || v > b {
		t.Errorf("the handed-over variant must go, and before its base:\n%s", s)
	}
}

// From review on #262: a handed-over cluster's Target names the Space its root
// reads, even when that is not the one this prefix would name, and step 5 fills
// that Space rather than making another; and it names the Secret the root
// pulls with, not the one this prefix would make. The annotation waits for step 5, so a
// Flux-aware `cub variant create` never meets a layers Space that is not there.
func TestTargetNamesTheLayersSpaceTheRootReads(t *testing.T) {
	const fleet = "gitops/flux/beginner"
	root := handedOver(t, fleet, "dev", "old-dev-layers")
	// A root made under another prefix pulls with that prefix's Secret.
	file := filepath.Join(root, fleet, "clusters", "dev", RootName+".yaml")
	y, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(strings.Replace(string(y), "name: confighub-flux-targets", "name: confighub-old-targets", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	p := planAt(t, root, fleet)
	s := ApplyScript(p, "flux", ".")
	if !strings.Contains(s, `echo '{"Annotations":{"confighub.com/flux-layers-space":"old-dev-layers","confighub.com/flux-pull-secret":"confighub-old-targets"}}' | cub target update --patch --space flux-targets dev `) {
		t.Errorf("dev's Target should name old-dev-layers and the Secret its root pulls with:\n%s", s)
	}
	if strings.Contains(s, "flux-dev-layers") {
		t.Errorf("nothing should make or name flux-dev-layers for a root reading old-dev-layers")
	}
	step5 := strings.Index(s, `step "5/5`)
	if a := strings.Index(s, `"confighub.com/flux-layers-space":`); a < step5 {
		t.Errorf("the Target is annotated before step 5 makes the layers Space")
	}
	if !strings.Contains(s, " $no_flux_layer --allow-exists") {
		t.Errorf("apply.sh makes the layer Units itself, so variant create must be told not to")
	}
}

// From review on #262: a root that pulls anonymously has no Secret, and one an
// earlier run named is removed, since the patch merges.
func TestAnonymousRootClearsThePullSecret(t *testing.T) {
	const fleet = "gitops/flux/beginner"
	root := handedOver(t, fleet, "dev", "flux-dev-layers")
	file := filepath.Join(root, fleet, "clusters", "dev", RootName+".yaml")
	y, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	anon := strings.Replace(string(y), "  secretRef:\n    name: confighub-flux-targets\n", "", 1)
	if anon == string(y) {
		t.Fatal("the root should have had a secretRef to remove")
	}
	if err := os.WriteFile(file, []byte(anon), 0o644); err != nil {
		t.Fatal(err)
	}
	s := ApplyScript(planAt(t, root, fleet), "flux", ".")
	if !strings.Contains(s, `{"Annotations":{"confighub.com/flux-layers-space":"flux-dev-layers","confighub.com/flux-pull-secret":null}}`) {
		t.Errorf("an anonymous root's Target should lose its pull Secret:\n%s", s)
	}
}

// From review on #262: a fleet planned again under another prefix keeps the
// name its root carries, not its directory's, so the Target it annotates is
// the one its variants use.
func TestRootKeepsItsClusterNameUnderAnotherPrefix(t *testing.T) {
	const fleet = "gitops/flux/expert-fleet"
	root := handedOver(t, fleet, "dev", "old-dev-1-layers")
	file := filepath.Join(root, fleet, "clusters", "dev", RootName+".yaml")
	y := strings.NewReplacer(gatewayMarker, "gw.example:5000", insecureMarker, "false", namespaceMarker, "flux-system").
		Replace(RootManifests("old", "dev-1"))
	if err := os.WriteFile(file, []byte(y), 0o644); err != nil {
		t.Fatal(err)
	}
	p := planAt(t, root, fleet)
	var found bool
	for _, c := range p.Clusters {
		found = found || (c.Name == "dev-1" && c.LayersSpace == "old-dev-1-layers" && c.PullSecret == "confighub-old-targets")
	}
	if !found {
		t.Fatalf("clusters/dev is dev-1, reading old-dev-1-layers: %+v", p.Clusters)
	}
	if s := ApplyScript(p, "flux", "."); !strings.Contains(s, "cub target update --patch --space flux-targets dev-1 ") {
		t.Errorf("dev-1's Target should be the one annotated")
	}
}
