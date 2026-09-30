package flux

import (
	"fmt"
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
	fc, err := CompareFields(fake(map[string]string{"get deployment": liveDeploy}), "apptique-dev", release)
	diffs := fc.Diffs
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
	fc, err := CompareFields(fake(map[string]string{"get deployment": liveDeploy}), "apptique-dev", release)
	diffs := fc.Diffs
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
	fc, err := CompareFields(fake(map[string]string{"get deployment": liveDeploy}), "apptique-dev", release)
	diffs := fc.Diffs
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

// A read that fails for any reason but absence says nothing about the object's
// fields, so it can never add up to a clean check (confighub/helm-expt#2020).
func TestFailedReadsAreNotClean(t *testing.T) {
	release := []byte(`
apiVersion: apps/v1
kind: Deployment
metadata: {name: frontend, namespace: apptique-dev}
spec: {replicas: 4}
---
apiVersion: v1
kind: ConfigMap
metadata: {name: tuning, namespace: apptique-dev}
data: {a: "1"}
---
apiVersion: v1
kind: Service
metadata: {name: frontend, namespace: apptique-dev}
spec: {type: ClusterIP}
`)
	for _, tc := range []struct {
		name   string
		err    string
		absent bool
	}{
		{"forbidden", `Error from server (Forbidden): configmaps "tuning" is forbidden: User "ci" cannot get resource "configmaps"`, false},
		{"timeout", `Unable to connect to the server: net/http: TLS handshake timeout`, false},
		{"unknown kind", `error: the server doesn't have a resource type "configmap"`, false},
		{"not found", `Error from server (NotFound): configmaps "tuning" not found`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := func(name string, args ...string) ([]byte, error) {
				key := strings.Join(args, " ")
				switch {
				case strings.Contains(key, "get deployment"):
					return []byte(liveDeploy), nil
				case strings.Contains(key, "get service"):
					return []byte(`{"kind":"Service","metadata":{"name":"frontend"},"spec":{"type":"ClusterIP"}}`), nil
				}
				return nil, fmt.Errorf("%s", tc.err)
			}
			fc, err := CompareFields(run, "apptique-dev", release)
			if err != nil {
				t.Fatal(err)
			}
			if fc.Clean() {
				t.Fatalf("one of three objects was not read; this cannot be clean: %+v", fc)
			}
			if fc.Total != 3 || fc.Compared != 2 {
				t.Errorf("want 2 of 3 compared, got %d of %d", fc.Compared, fc.Total)
			}
			if len(fc.Diffs) != 0 {
				t.Errorf("the objects that were read match: %v", fc.Diffs)
			}
			if tc.absent != (len(fc.Absent) == 1) || tc.absent == (len(fc.Unreadable) == 1) {
				t.Errorf("absent=%v, want %v; unreadable=%v", fc.Absent, tc.absent, fc.Unreadable)
			}
		})
	}
}

// An object the inventory lists and the cluster does not have is a stale
// record, not a match; one the inventory does not list is the release adding
// it, which the inventory comparison already reports.
func TestAbsentObjectMarksAStaleInventory(t *testing.T) {
	live := Live{Owned: []Owned{{Kind: "ConfigMap", Namespace: "apptique-dev", Name: "tuning"}}}
	absent := []Owned{
		{Kind: "ConfigMap", Namespace: "apptique-dev", Name: "tuning"},
		{Kind: "ConfigMap", Namespace: "apptique-dev", Name: "new-one"},
	}
	got := staleIn(live, absent)
	if len(got) != 1 || !strings.Contains(got[0], "tuning") {
		t.Errorf("only tuning is listed and missing: %v", got)
	}
	r := Result{Inventory: InventoryComparison{Same: 1}, Fields: &FieldCheck{Total: 1, Compared: 1}, Stale: got}
	if r.OK() {
		t.Error("a stale inventory cannot pass")
	}
}
