package flux

import (
	"strings"
	"testing"
)

func checked(fields FieldCheck, inv InventoryComparison) Result {
	return Result{
		Health:    "Healthy",
		Check:     Check{Kustomization: "apps", Space: "flux-apps-dev", Unit: "apps"},
		Release:   Release{Num: 2, ManifestDigest: "sha256:m2", UnitRevision: 5},
		Inventory: inv,
		Fields:    &fields,
	}
}

// A clean check records a Pass on the revision the release bundled, with the
// same type cub kubara uses.
func TestRecordCheckPass(t *testing.T) {
	hub := &fakeHub{}
	id, err := RecordCheck(hub, checked(FieldCheck{Total: 4, Compared: 4}, InventoryComparison{Same: 4}))
	if err != nil {
		t.Fatal(err)
	}
	if len(hub.attested) != 1 {
		t.Fatalf("want one attestation, got %d", len(hub.attested))
	}
	got := hub.attested[0]
	if got.Space != "flux-apps-dev" || got.Unit != "apps" || got.Revision != 5 || got.Type != "LiveCheck" ||
		got.Claims["kustomize.toolkit.fluxcd.io/kustomization"] != "flux-system/apps" || got.Claims["confighub.com/release"] != "sha256:m2" ||
		!strings.Contains(got.Note, "every field the release sets matches on all 4") {
		t.Errorf("the attestation should name the unit, revision, type, claims and what matched: %+v", got)
	}
	if got.Reject || id != "0f0e0d0c-0b0a-4908-8706-050403020100" {
		t.Errorf("a clean check is a Pass, and its ID comes back: %+v, %s", got, id)
	}
}

// A difference is recorded as a rejection that names it.
func TestRecordCheckRejects(t *testing.T) {
	hub := &fakeHub{}
	f := FieldCheck{Total: 1, Compared: 1, Diffs: []FieldDiff{{Object: "Deployment apptique-dev/frontend", Path: ".spec.replicas", Live: "4", Release: "1"}}}
	if _, err := RecordCheck(hub, checked(f, InventoryComparison{Same: 1})); err != nil {
		t.Fatal(err)
	}
	if got := hub.attested[0]; !got.Reject || !strings.Contains(got.Note, ".spec.replicas: cluster has 4, the release holds 1") {
		t.Errorf("want a rejection naming the field: %+v", got)
	}
}

// Without a field comparison, or without a release, there is nothing to claim.
func TestRecordCheckNeedsFieldsAndARelease(t *testing.T) {
	hub := &fakeHub{}
	r := checked(FieldCheck{}, InventoryComparison{})
	r.Fields = nil
	if _, err := RecordCheck(hub, r); err == nil || len(hub.attested) != 0 {
		t.Errorf("no --fields, no record: %v %v", err, hub.attested)
	}
	r = checked(FieldCheck{Total: 1, Compared: 1}, InventoryComparison{Same: 1})
	r.Release = Release{}
	if _, err := RecordCheck(hub, r); err == nil || len(hub.attested) != 0 {
		t.Errorf("no release, no record: %v %v", err, hub.attested)
	}
}

// Health is part of what a recorded check claims: a layer Flux reports
// stalled or not ready is a rejection that says so, one still reconciling is
// neither a Pass nor a rejection, and a Pass names the health it rests on.
func TestRecordCheckJudgesHealth(t *testing.T) {
	for _, tc := range []struct {
		name     string
		mutate   func(k map[string]any)
		health   string
		recorded bool
		reject   bool
		says     string
	}{
		{"ready, workloads checked", nil, "Healthy", true, false, "Flux reports it ready"},
		{"ready, workloads not checked", func(k map[string]any) { spec(k)["wait"] = false }, "Unknown", true, false, "that means applied, not healthy"},
		{"not ready", func(k map[string]any) {
			status(k)["conditions"] = []any{map[string]any{"type": "Ready", "status": "False", "observedGeneration": 3, "message": "health check failed"}}
		}, "Degraded", true, true, "Flux reports apps not ready: health check failed"},
		{"stalled", func(k map[string]any) {
			status(k)["conditions"] = append(status(k)["conditions"].([]any), map[string]any{"type": "Stalled", "status": "True", "message": "path not found"})
		}, "Degraded", true, true, "Flux reports apps stalled: path not found"},
		{"reconciling", func(k map[string]any) {
			status(k)["conditions"] = append(status(k)["conditions"].([]any), map[string]any{"type": "Reconciling", "status": "True"})
		}, "Progressing", false, false, ""},
		{"a generation Flux has not seen", func(k map[string]any) { k["metadata"].(map[string]any)["generation"] = 4 }, "Progressing", false, false, ""},
		{"suspended", func(k map[string]any) { spec(k)["suspend"] = true }, "Suspended", false, false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			live, err := LiveInventory(fake(map[string]string{"get kustomization": ks(t, tc.mutate)}), "flux-system", "apps")
			if err != nil {
				t.Fatal(err)
			}
			if live.Health != tc.health {
				t.Fatalf("health: want %s, got %s", tc.health, live.Health)
			}
			hub := &fakeHub{}
			r := checked(FieldCheck{Total: 4, Compared: 4}, InventoryComparison{Same: 4})
			r.Health, r.Unhealthy, r.NotYet = live.Health, live.Unhealthy, live.NotYet
			id, err := RecordCheck(hub, r)
			if err != nil {
				t.Fatal(err)
			}
			if (len(hub.attested) == 1) != tc.recorded || (id != "") != tc.recorded {
				t.Fatalf("recorded: want %v, got %d attestations and ID %q", tc.recorded, len(hub.attested), id)
			}
			if !tc.recorded {
				if r.NotYet == "" {
					t.Error("say why nothing is recorded yet")
				}
				return
			}
			if got := hub.attested[0]; got.Reject != tc.reject || !strings.Contains(got.Note, tc.says) || got.Claims["kustomize.toolkit.fluxcd.io/health"] != tc.health {
				t.Errorf("want reject=%v saying %q with the health claimed: %+v", tc.reject, tc.says, got)
			}
		})
	}
}
