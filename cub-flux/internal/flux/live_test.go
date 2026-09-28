package flux

import (
	"fmt"
	"strings"
	"testing"
)

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

// Flux's own inventory, in its own id form.
const inv = `{"status":{"inventory":{"entries":[
 {"id":"apptique-prod_frontend_apps_Deployment","v":"v1"},
 {"id":"apptique-prod_frontend__Service","v":"v1"},
 {"id":"apptique-prod_legacy-tuning__ConfigMap","v":"v1"},
 {"id":"_apptique-prod__Namespace","v":"v1"}
]}}}`

func TestInventoryIDsAreRead(t *testing.T) {
	owned, err := LiveInventory(fake(map[string]string{"get kustomization": inv}), "flux-system", "apps")
	if err != nil {
		t.Fatal(err)
	}
	if len(owned) != 4 {
		t.Fatalf("want 4 entries, got %d", len(owned))
	}
	want := map[string]bool{
		"apps|Deployment|apptique-prod|frontend": true,
		"|Service|apptique-prod|frontend":        true,
		"|ConfigMap|apptique-prod|legacy-tuning": true,
		"|Namespace||apptique-prod":              true, // cluster-scoped: no namespace
	}
	for _, o := range owned {
		if !want[o.Key()] {
			t.Errorf("unexpected key %q", o.Key())
		}
	}
}

// The layer applied a ConfigMap the release does not hold. Every layer prunes,
// so swapping the source deletes it. No render-side check can see this.
func TestInventoryFindsWhatWouldBePruned(t *testing.T) {
	owned, err := LiveInventory(fake(map[string]string{"get kustomization": inv}), "flux-system", "apps")
	if err != nil {
		t.Fatal(err)
	}
	stored, err := ObjectsIn([]byte(`
apiVersion: apps/v1
kind: Deployment
metadata: {name: frontend, namespace: apptique-prod}
---
apiVersion: v1
kind: Service
metadata: {name: frontend, namespace: apptique-prod}
---
apiVersion: v1
kind: Namespace
metadata: {name: apptique-prod}
`))
	if err != nil {
		t.Fatal(err)
	}
	c := CompareInventory(owned, stored, "")
	if c.OK() {
		t.Fatal("the ConfigMap would be deleted; this must not pass")
	}
	if c.Same != 3 {
		t.Errorf("Same = %d, want 3", c.Same)
	}
	if len(c.WouldPrune) != 1 || !strings.Contains(c.WouldPrune[0], "legacy-tuning") || !strings.Contains(c.WouldPrune[0], "DELETE") {
		t.Errorf("want the ConfigMap named with the consequence, got %v", c.WouldPrune)
	}
}

// A Kustomization with targetNamespace puts namespace-less objects there.
func TestTargetNamespace(t *testing.T) {
	owned, err := LiveInventory(fake(map[string]string{"get kustomization": inv}), "flux-system", "apps")
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
---
apiVersion: v1
kind: Namespace
metadata: {name: apptique-prod}
`))
	if err != nil {
		t.Fatal(err)
	}
	c := CompareInventory(owned, stored, "apptique-prod")
	if !c.OK() {
		t.Errorf("all four match once targetNamespace applies: %+v", c)
	}
}

// An inventory that is empty must not read as "nothing would change".
func TestEmptyInventoryIsRefused(t *testing.T) {
	_, err := LiveInventory(fake(map[string]string{"get kustomization": `{"status":{}}`}), "flux-system", "apps")
	if err == nil || !strings.Contains(err.Error(), "empty inventory") {
		t.Errorf("an empty inventory should be refused, got %v", err)
	}
}

// A malformed id is named, not guessed at.
func TestMalformedInventoryID(t *testing.T) {
	_, err := LiveInventory(fake(map[string]string{"get kustomization": `{"status":{"inventory":{"entries":[{"id":"nope","v":"v1"}]}}}`}), "flux-system", "apps")
	if err == nil || !strings.Contains(err.Error(), "is not <namespace>_<name>_<group>_<kind>") {
		t.Errorf("want the id form named, got %v", err)
	}
}
