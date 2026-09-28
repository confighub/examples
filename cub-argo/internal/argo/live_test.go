package argo

import (
	"fmt"
	"strings"
	"testing"
)

// fake stands in for kubectl and cub.
func fake(out map[string]string) Runner {
	return func(name string, args ...string) ([]byte, error) {
		key := name + " " + strings.Join(args, " ")
		for k, v := range out {
			if strings.Contains(key, k) {
				return []byte(v), nil
			}
		}
		return nil, fmt.Errorf("no fake for %q", key)
	}
}

const appStatus = `{"spec":{"syncPolicy":{"automated":{"prune":true}}},"status":{"resources":[
 {"group":"apps","kind":"Deployment","namespace":"storefront-prod","name":"frontend","requiresPruning":true},
 {"group":"","kind":"Service","namespace":"storefront-prod","name":"frontend","requiresPruning":true},
 {"group":"","kind":"ConfigMap","namespace":"storefront-prod","name":"legacy-tuning","requiresPruning":true},
 {"group":"batch","kind":"Job","namespace":"storefront-prod","name":"migrate","hook":true}
]}}`

// The release holds the Deployment and Service, but not the ConfigMap someone
// added to the cluster through an earlier Git state. Argo prunes, so moving
// the source would delete it. This is what a render-side check cannot see.
func TestInventoryFindsWhatWouldBePruned(t *testing.T) {
	live, err := LiveInventory(fake(map[string]string{"get application": appStatus}), "argocd", "prod-1-apptique")
	if err != nil {
		t.Fatal(err)
	}
	stored, err := ObjectsIn([]byte(`
apiVersion: apps/v1
kind: Deployment
metadata: {name: frontend, namespace: storefront-prod}
---
apiVersion: v1
kind: Service
metadata: {name: frontend, namespace: storefront-prod}
`))
	if err != nil {
		t.Fatal(err)
	}
	c := CompareInventory(live, stored, "storefront-prod")
	if c.OK() {
		t.Fatal("the ConfigMap would be deleted; the comparison should not pass")
	}
	if c.Same != 2 {
		t.Errorf("Deployment and Service match, got Same = %d", c.Same)
	}
	if len(c.WouldPrune) != 1 || !strings.Contains(c.WouldPrune[0], "legacy-tuning") ||
		!strings.Contains(c.WouldPrune[0], "DELETED") {
		t.Errorf("want the pruned ConfigMap named and the consequence spelled out, got %v", c.WouldPrune)
	}
	if len(c.Notes) != 1 || !strings.Contains(c.Notes[0], "sync hook") {
		t.Errorf("the hook Job is not part of the desired state and should be a note, got %v", c.Notes)
	}
}

// An object the release holds that Argo does not own would appear on the
// cluster. That is a change too, and the handover should say so first.
func TestInventoryFindsWhatWouldBeAdded(t *testing.T) {
	live, err := LiveInventory(fake(map[string]string{"get application": appStatus}), "argocd", "a")
	if err != nil {
		t.Fatal(err)
	}
	stored, err := ObjectsIn([]byte(`
apiVersion: apps/v1
kind: Deployment
metadata: {name: frontend, namespace: storefront-prod}
---
apiVersion: v1
kind: Service
metadata: {name: frontend, namespace: storefront-prod}
---
apiVersion: v1
kind: ConfigMap
metadata: {name: legacy-tuning, namespace: storefront-prod}
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata: {name: frontend, namespace: storefront-prod}
`))
	if err != nil {
		t.Fatal(err)
	}
	c := CompareInventory(live, stored, "storefront-prod")
	if len(c.WouldPrune) != 0 {
		t.Errorf("everything Argo owns is held now, got %v", c.WouldPrune)
	}
	if len(c.WouldAdd) != 1 || !strings.Contains(c.WouldAdd[0], "Ingress") {
		t.Errorf("want the Ingress named as an addition, got %v", c.WouldAdd)
	}
}

// A rendered object with no namespace lands in the Application's destination,
// which is where Argo reports it.
func TestNamespaceFromTheDestination(t *testing.T) {
	live, err := LiveInventory(fake(map[string]string{"get application": appStatus}), "argocd", "a")
	if err != nil {
		t.Fatal(err)
	}
	stored, err := ObjectsIn([]byte(`
apiVersion: apps/v1
kind: Deployment
metadata: {name: frontend}
---
apiVersion: v1
kind: Service
metadata: {name: frontend}
---
apiVersion: v1
kind: ConfigMap
metadata: {name: legacy-tuning}
`))
	if err != nil {
		t.Fatal(err)
	}
	c := CompareInventory(live, stored, "storefront-prod")
	if !c.OK() {
		t.Errorf("all three match once the destination namespace is applied: %+v", c)
	}
	if c.Same != 3 {
		t.Errorf("Same = %d, want 3", c.Same)
	}
}

// An Application that has never synced reports nothing, and an empty
// inventory must not read as "nothing would change".
func TestEmptyInventoryIsRefused(t *testing.T) {
	_, err := LiveInventory(fake(map[string]string{"get application": `{"status":{}}`}), "argocd", "a")
	if err == nil || !strings.Contains(err.Error(), "owning nothing") {
		t.Errorf("an empty inventory should be refused, got %v", err)
	}
}

// An Application that does not prune leaves an object it owns behind rather
// than deleting it. The two are different enough to say differently, and the
// per-resource requiresPruning flag cannot tell them apart: it describes what
// is out of sync today, and is absent while everything is in sync.
func TestPruneComesFromTheSyncPolicy(t *testing.T) {
	const noPrune = `{"spec":{"syncPolicy":{"automated":{"prune":false}}},"status":{"resources":[
	 {"group":"","kind":"ConfigMap","namespace":"n","name":"left-behind"}]}}`
	live, err := LiveInventory(fake(map[string]string{"get application": noPrune}), "argocd", "a")
	if err != nil {
		t.Fatal(err)
	}
	if live.Prunes {
		t.Error("this Application does not prune")
	}
	c := CompareInventory(live, nil, "n")
	if len(c.WouldPrune) != 1 || strings.Contains(c.WouldPrune[0], "DELETED") {
		t.Errorf("without prune the object is left behind, not deleted: %v", c.WouldPrune)
	}
	if !strings.Contains(c.WouldPrune[0], "managed by nothing") {
		t.Errorf("want the consequence named, got %v", c.WouldPrune)
	}
}
