package argo

import (
	"encoding/json"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// An Application as the ApplicationSet controller made it and Argo CD runs it.
const generatedApp = `{
  "apiVersion": "argoproj.io/v1alpha1",
  "kind": "Application",
  "metadata": {
    "name": "dev-1-apptique",
    "namespace": "argocd",
    "uid": "u-1",
    "resourceVersion": "42",
    "generation": 3,
    "creationTimestamp": "2026-09-30T10:00:00Z",
    "labels": {"rollout-phase": "canary"},
    "annotations": {
      "argocd.argoproj.io/refresh": "hard",
      "argocd.argoproj.io/sync-options": "ServerSideApply=true",
      "kubectl.kubernetes.io/last-applied-configuration": "{}",
      "team": "storefront"
    },
    "finalizers": ["resources-finalizer.argocd.argoproj.io"],
    "ownerReferences": [{"kind": "ApplicationSet", "name": "apptique", "blockOwnerDeletion": true}],
    "managedFields": [{"manager": "argocd-applicationset-controller"}]
  },
  "spec": {
    "project": "storefront",
    "destination": {"server": "https://rh-dev1-control-plane:6443", "namespace": "storefront-dev"},
    "source": {"repoURL": "https://github.com/confighub/examples", "path": "gitops/argo/expert-app-of-apps/apps/apptique/overlays/dev", "targetRevision": "main", "kustomize": {"version": "v5"}},
    "syncPolicy": {"automated": {"prune": true, "selfHeal": true}, "syncOptions": ["CreateNamespace=true"]}
  },
  "operation": {"sync": {}},
  "status": {"sync": {"status": "Synced"}}
}`

func TestApplicationUnitKeepsWhatAPersonWrote(t *testing.T) {
	var live map[string]any
	if err := json.Unmarshal([]byte(generatedApp), &live); err != nil {
		t.Fatal(err)
	}
	u, err := applicationUnit(live, "oci://gw.example:5000/", "argo-apptique-dev-1")
	if err != nil {
		t.Fatal(err)
	}
	if got := get(u, "spec", "source"); !jsonEq(got, map[string]any{"repoURL": "oci://gw.example:5000/space/argo-apptique-dev-1", "path": ".", "targetRevision": "latest"}) {
		t.Errorf("source should read the variant's Space: %v", got)
	}
	// Kept: the name Argo tracks it by, and what the ApplicationSet set.
	for _, keep := range [][]string{{"metadata", "name"}, {"spec", "project"}, {"spec", "destination", "server"}, {"spec", "destination", "namespace"}, {"spec", "syncPolicy"}, {"metadata", "labels", "rollout-phase"}, {"metadata", "annotations", "team"}} {
		if get(u, keep...) == nil {
			t.Errorf("%s was dropped", strings.Join(keep, "."))
		}
	}
	// Dropped: what the cluster wrote, and the owner being retired.
	for _, drop := range [][]string{{"metadata", "uid"}, {"metadata", "resourceVersion"}, {"metadata", "generation"}, {"metadata", "creationTimestamp"}, {"metadata", "ownerReferences"}, {"metadata", "managedFields"}, {"status"}, {"operation"},
		{"metadata", "annotations", "argocd.argoproj.io/refresh"}, {"metadata", "annotations", "kubectl.kubernetes.io/last-applied-configuration"}} {
		if get(u, drop...) != nil {
			t.Errorf("%s should not be in the Unit", strings.Join(drop, "."))
		}
	}
	if got := str(get(u, "metadata", "annotations", "argocd.argoproj.io/sync-options")); got != "Prune=false,Replace=true,ServerSideApply=true" {
		t.Errorf("Prune=false and Replace=true should join the options it had: %q", got)
	}
	if fin := get(u, "metadata", "finalizers"); !jsonEq(fin, []any{"resources-finalizer.argocd.argoproj.io"}) {
		t.Errorf("the finalizer is the estate's choice and a replace would drop it: %v", fin)
	}
	// The template's own source settings go: the bundle is rendered already.
	if get(u, "spec", "source", "kustomize") != nil {
		t.Errorf("source should hold nothing but the Space")
	}
	if got := str(get(u, "metadata", "annotations", DeliversAnnotation)); got != "argo-apptique-dev-1" {
		t.Errorf("the Application should say which Space it reads: %q", got)
	}
}

func TestApplicationUnitRefusesMultiSource(t *testing.T) {
	var live map[string]any
	_ = json.Unmarshal([]byte(generatedApp), &live)
	obj(live["spec"])["sources"] = []any{map[string]any{"repoURL": "a"}, map[string]any{"repoURL": "b"}}
	if _, err := applicationUnit(live, "gw", "s"); err == nil || !strings.Contains(err.Error(), "several sources") {
		t.Errorf("a multi-source Application has no one source to replace: %v", err)
	}
}

func TestApplicationUnitReadsTheCluster(t *testing.T) {
	var asked []string
	run := func(name string, args ...string) ([]byte, error) {
		asked = append(asked, name+" "+strings.Join(args, " "))
		return []byte(generatedApp), nil
	}
	b, err := ApplicationUnit(run, "dev-1-apptique", "gw.example:5000", "argo-apptique-dev-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(asked) != 1 || asked[0] != "kubectl -n argocd get application dev-1-apptique -o json" {
		t.Errorf("should read the one Application: %v", asked)
	}
	var u map[string]any
	if err := yaml.Unmarshal(b, &u); err != nil {
		t.Fatalf("the Unit should be YAML: %v\n%s", err, b)
	}
	if str(get(u, "kind")) != "Application" || str(get(u, "spec", "source", "repoURL")) != "oci://gw.example:5000/space/argo-apptique-dev-1" {
		t.Errorf("unexpected Unit:\n%s", b)
	}
}

func jsonEq(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

// A joined cluster's Application comes from its own rendered template, so
// every templated field (a project picked by a cluster label, say) is its own;
// the namespace the controller would give it is filled in.
func TestApplicationUnitFromTheRenderedTemplate(t *testing.T) {
	rendered := []byte("metadata:\n  name: prod-1-apptique\n  labels: {rollout-phase: primary}\nspec:\n  project: team-prod\n  destination: {server: https://prod, namespace: storefront-prod}\n  source: {repoURL: https://github.com/x, path: apps/prod, kustomize: {version: v5}}\n")
	b, err := ApplicationUnitRendered(rendered, "argocd", "gw:5000", "argo-apptique-prod-1")
	if err != nil {
		t.Fatal(err)
	}
	var u map[string]any
	if err := yaml.Unmarshal(b, &u); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{"metadata.namespace": "argocd", "spec.project": "team-prod", "spec.destination.server": "https://prod", "metadata.labels.rollout-phase": "primary", "spec.source.repoURL": "oci://gw:5000/space/argo-apptique-prod-1", "kind": "Application"} {
		if got := str(get(u, strings.Split(path, ".")...)); got != want {
			t.Errorf("%s = %q, want %q", path, got, want)
		}
	}
	if get(u, "spec", "source", "kustomize") != nil {
		t.Errorf("the source should hold nothing but the Space")
	}
}
