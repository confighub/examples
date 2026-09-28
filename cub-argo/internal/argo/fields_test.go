package argo

import (
	"strings"
	"testing"
)

const liveDeploy = `{
 "apiVersion":"apps/v1","kind":"Deployment",
 "metadata":{"name":"frontend","namespace":"apptique-dev",
   "managedFields":[
     {"manager":"argocd-controller","operation":"Apply","time":"2026-09-28T10:00:00Z"},
     {"manager":"kubectl-scale","operation":"Update","time":"2026-09-28T11:00:00Z"}]},
 "spec":{"replicas":4,"progressDeadlineSeconds":600,
   "template":{"spec":{"containers":[{"name":"frontend","image":"nginx:1.27-alpine"}]}}},
 "status":{"readyReplicas":4}}`

// The case an object-set comparison cannot see: the objects all match, but
// someone scaled the Deployment by hand and the release still says 1.
func TestHandEditIsCaught(t *testing.T) {
	release := []byte(`
apiVersion: apps/v1
kind: Deployment
metadata: {name: frontend, namespace: apptique-dev}
spec:
  replicas: 1
  template:
    spec:
      containers:
        - name: frontend
          image: nginx:1.27-alpine
`)
	diffs, err := CompareFields(fake(map[string]string{"get deployment": liveDeploy}), "apptique-dev", release)
	if err != nil {
		t.Fatal(err)
	}
	if len(diffs) != 1 {
		t.Fatalf("only replicas differs; got %d: %v", len(diffs), diffs)
	}
	d := diffs[0]
	if d.Path != ".spec.replicas" || d.Live != "4" || d.Release != "1" {
		t.Errorf("want .spec.replicas 4 vs 1, got %+v", d)
	}
	if h := ByHand(d.Managers); len(h) != 1 || h[0] != "kubectl-scale" {
		t.Errorf("kubectl-scale wrote it last and should be named a hand edit, got %v", d.Managers)
	}
	if !strings.Contains(d.String(), "kubectl-scale") {
		t.Errorf("the message should name who wrote it: %s", d)
	}
}

// Only the fields the release sets are compared. Kubernetes fills in defaults
// the release never mentions, and reporting those would drown the real ones.
func TestServerDefaultsAreNotDrift(t *testing.T) {
	release := []byte(`
apiVersion: apps/v1
kind: Deployment
metadata: {name: frontend, namespace: apptique-dev}
spec:
  replicas: 4
  template:
    spec:
      containers:
        - name: frontend
          image: nginx:1.27-alpine
`)
	diffs, err := CompareFields(fake(map[string]string{"get deployment": liveDeploy}), "apptique-dev", release)
	if err != nil {
		t.Fatal(err)
	}
	if len(diffs) != 0 {
		t.Errorf("progressDeadlineSeconds and status are the cluster's, not drift: %v", diffs)
	}
}

// A controller's own write is not a hand edit, and saying so would train
// people to ignore the warning.
func TestControllerWriteIsNotAHandEdit(t *testing.T) {
	if h := ByHand([]string{"argocd-controller", "kube-controller-manager"}); len(h) != 0 {
		t.Errorf("neither is a person at a terminal, got %v", h)
	}
	if h := ByHand([]string{"kubectl-client-side-apply"}); len(h) != 1 {
		t.Error("kubectl is a person at a terminal")
	}
}

// An image changed on the cluster is the other case worth catching.
func TestChangedImageIsCaught(t *testing.T) {
	release := []byte(`
apiVersion: apps/v1
kind: Deployment
metadata: {name: frontend, namespace: apptique-dev}
spec:
  replicas: 4
  template:
    spec:
      containers:
        - name: frontend
          image: nginx:1.26-alpine
`)
	diffs, err := CompareFields(fake(map[string]string{"get deployment": liveDeploy}), "apptique-dev", release)
	if err != nil {
		t.Fatal(err)
	}
	if len(diffs) != 1 || !strings.Contains(diffs[0].Path, "image") {
		t.Fatalf("want the image difference, got %v", diffs)
	}
	if !strings.Contains(diffs[0].String(), "1.27-alpine") || !strings.Contains(diffs[0].String(), "1.26-alpine") {
		t.Errorf("both values should be shown: %s", diffs[0])
	}
}
