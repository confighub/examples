package flux

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

const (
	d1 = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	d2 = "sha256:2222222222222222222222222222222222222222222222222222222222222222"
	dX = "sha256:9999999999999999999999999999999999999999999999999999999999999999"
)

var releasesJSON = fmt.Sprintf(`[
 {"Release":{"ReleaseNum":1,"ManifestDigest":"%s","Published":true}},
 {"Release":{"ReleaseNum":2,"ManifestDigest":"%s","Published":true}},
 {"Release":{"ReleaseNum":3,"ManifestDigest":"%s","Published":false}}]`, d1, d2, dX)

// ks builds a Kustomization's JSON. Ready is for the current generation unless
// a test says otherwise.
func ks(t *testing.T, mutate func(k map[string]any)) string {
	t.Helper()
	k := map[string]any{
		"metadata": map[string]any{"generation": 3},
		"spec": map[string]any{
			"sourceRef": map[string]any{"kind": "OCIRepository", "name": "apps"},
			"wait":      true,
		},
		"status": map[string]any{
			"observedGeneration":    3,
			"lastAppliedRevision":   "latest@" + d2,
			"lastAttemptedRevision": "latest@" + d2,
			"conditions": []any{
				map[string]any{"type": "Ready", "status": "True", "observedGeneration": 3, "message": "Applied revision: latest@" + d2},
			},
			"inventory": map[string]any{"entries": []any{
				map[string]any{"id": "apptique-dev_frontend_apps_Deployment", "v": "v1"},
			}},
		},
	}
	if mutate != nil {
		mutate(k)
	}
	b, err := json.Marshal(k)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func spec(k map[string]any) map[string]any   { return k["spec"].(map[string]any) }
func status(k map[string]any) map[string]any { return k["status"].(map[string]any) }

func readWith(t *testing.T, kjson, url string) Reading {
	t.Helper()
	run := fake(map[string]string{
		"get kustomization": kjson,
		"get ocirepository": url,
		"release list":      releasesJSON,
	})
	r, err := ReadStatus(run, Check{Kustomization: "apps", Space: "flux-apps-dev"}, time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

const spaceURL = "oci://gw.example/space/flux-apps-dev"

func TestStatusMapping(t *testing.T) {
	for _, tc := range []struct {
		name                     string
		mutate                   func(k map[string]any)
		sync, health, phase, rev string
		gate                     bool
		says                     string
	}{
		{"newest release applied, workloads checked", nil,
			"Synced", "Healthy", "Succeeded", d2, true, "release 2 applied"},
		{"an older release applied", func(k map[string]any) {
			status(k)["lastAppliedRevision"] = "latest@" + d1
			status(k)["lastAttemptedRevision"] = "latest@" + d1
		}, "OutOfSync", "Healthy", "Succeeded", d1, false, "release 1 is applied; release 2 is published and not applied yet"},
		{"an unpublished digest applied", func(k map[string]any) {
			status(k)["lastAppliedRevision"] = "latest@" + dX
			status(k)["lastAttemptedRevision"] = "latest@" + dX
		}, "Unknown", "Healthy", "Succeeded", dX, false, "no published release"},
		{"workloads nobody checked", func(k map[string]any) {
			spec(k)["wait"] = false
		}, "Synced", "Unknown", "Succeeded", d2, false, "Ready means applied, not healthy"},
		{"healthChecks covering every workload count as checked", func(k map[string]any) {
			spec(k)["wait"] = false
			spec(k)["healthChecks"] = []any{map[string]any{"kind": "Deployment", "name": "frontend", "namespace": "apptique-dev"}}
		}, "Synced", "Healthy", "Succeeded", d2, true, "release 2 applied"},
		{"healthChecks in the targetNamespace by default", func(k map[string]any) {
			spec(k)["wait"] = false
			spec(k)["targetNamespace"] = "apptique-dev"
			spec(k)["healthChecks"] = []any{map[string]any{"kind": "Deployment", "name": "frontend"}}
		}, "Synced", "Healthy", "Succeeded", d2, true, "release 2 applied"},
		{"healthChecks covering one workload of two", func(k map[string]any) {
			spec(k)["wait"] = false
			spec(k)["healthChecks"] = []any{map[string]any{"kind": "Deployment", "name": "frontend", "namespace": "apptique-dev"}}
			status(k)["inventory"] = map[string]any{"entries": []any{
				map[string]any{"id": "apptique-dev_frontend_apps_Deployment", "v": "v1"},
				map[string]any{"id": "apptique-dev_cart_apps_Deployment", "v": "v1"},
			}}
		}, "Synced", "Unknown", "Succeeded", d2, false, "Deployment apptique-dev/cart"},
		{"no workloads, nothing to check", func(k map[string]any) {
			spec(k)["wait"] = false
			status(k)["inventory"] = map[string]any{"entries": []any{map[string]any{"id": "_apptique-dev__Namespace", "v": "v1"}}}
		}, "Synced", "Healthy", "Succeeded", d2, true, "release 2 applied"},
		{"applying a newer digest", func(k map[string]any) {
			status(k)["lastAttemptedRevision"] = "latest@" + dX
		}, "OutOfSync", "Progressing", "Running", d2, false, "applying " + dX},
		{"Ready False", func(k map[string]any) {
			status(k)["conditions"] = []any{map[string]any{"type": "Ready", "status": "False", "observedGeneration": 3, "message": "health check failed"}}
		}, "OutOfSync", "Degraded", "Failed", "", false, "health check failed"},
		{"stalled", func(k map[string]any) {
			status(k)["conditions"] = append(status(k)["conditions"].([]any), map[string]any{"type": "Stalled", "status": "True", "message": "path not found"})
		}, "OutOfSync", "Degraded", "Failed", "", false, "stalled: path not found"},
		{"reconciling", func(k map[string]any) {
			status(k)["conditions"] = append(status(k)["conditions"].([]any), map[string]any{"type": "Reconciling", "status": "True", "message": "applying"})
		}, "OutOfSync", "Progressing", "Running", "", false, "applying"},
		{"a generation Flux has not seen", func(k map[string]any) {
			k["metadata"].(map[string]any)["generation"] = 4
		}, "Unknown", "Progressing", "Running", "", false, "current generation"},
		{"Ready from an old generation", func(k map[string]any) {
			status(k)["conditions"] = []any{map[string]any{"type": "Ready", "status": "True", "observedGeneration": 2}}
		}, "Unknown", "Progressing", "Running", "", false, "current generation"},
		{"suspended", func(k map[string]any) {
			spec(k)["suspend"] = true
		}, "Unknown", "Suspended", "", "", false, "suspended"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := readWith(t, ks(t, tc.mutate), spaceURL).Status
			if s.SyncStatus != tc.sync || s.HealthStatus != tc.health || s.OperationPhase != tc.phase || s.Revision != tc.rev {
				t.Errorf("want %s/%s/%s at %q, got %s/%s/%s at %q", tc.sync, tc.health, tc.phase, tc.rev, s.SyncStatus, s.HealthStatus, s.OperationPhase, s.Revision)
			}
			if s.Gate() != tc.gate {
				t.Errorf("gate: want %v, got %v for %s", tc.gate, s.Gate(), s)
			}
			if !strings.Contains(s.Message, tc.says) {
				t.Errorf("message %q should say %q", s.Message, tc.says)
			}
			if s.Source != StatusSource || s.App != "apps" || s.ObservedAt != "2026-09-30T12:00:00Z" {
				t.Errorf("source, app or time wrong: %+v", s)
			}
		})
	}
}

// Only a layer reading this Space's release is reported: one still on Git, or
// reading another Space, is not ConfigHub's to speak for.
func TestStatusSkipsLayersNotReadingTheSpace(t *testing.T) {
	onGit := ks(t, func(k map[string]any) {
		spec(k)["sourceRef"] = map[string]any{"kind": "GitRepository", "name": "fleet-repo"}
	})
	if r := readWith(t, onGit, spaceURL); !strings.Contains(r.Skip, "GitRepository fleet-repo") {
		t.Errorf("a layer on Git should be skipped: %+v", r)
	}
	if r := readWith(t, ks(t, nil), "oci://gw.example/space/flux-apps-prod"); !strings.Contains(r.Skip, "not Space flux-apps-dev") {
		t.Errorf("a layer reading another Space should be skipped: %+v", r)
	}
}

// A read that fails writes nothing: a guess could open a gate.
func TestStatusReadFailureIsNotAReading(t *testing.T) {
	run := fake(map[string]string{"get kustomization": ks(t, nil), "get ocirepository": spaceURL})
	if _, err := ReadStatus(run, Check{Kustomization: "apps", Space: "flux-apps-dev"}, time.Now()); err == nil {
		t.Error("a failed release list must be an error, not a reading")
	}
}

func TestStatusMessageIsClipped(t *testing.T) {
	long := strings.Repeat("x", 2000)
	r := readWith(t, ks(t, func(k map[string]any) {
		status(k)["conditions"] = []any{map[string]any{"type": "Ready", "status": "False", "observedGeneration": 3, "message": long}}
	}), spaceURL)
	p, err := StatusPatch(r.Status)
	if err != nil {
		t.Fatal(err)
	}
	var body struct{ Annotations map[string]string }
	if err := json.Unmarshal(p, &body); err != nil {
		t.Fatal(err)
	}
	if v := body.Annotations[LiveStatusAnnotation]; len(v) > 1024 {
		t.Errorf("ConfigHub allows 1024 bytes an annotation; this is %d", len(v))
	}
}

func heldSpace(t *testing.T, s *LiveStatus) string {
	t.Helper()
	if s == nil {
		return `{"Space":{"Slug":"flux-apps-dev","Annotations":{"other":"kept"}}}`
	}
	doc, _ := json.Marshal(s)
	b, _ := json.Marshal(map[string]any{"Space": map[string]any{"Annotations": map[string]string{LiveStatusAnnotation: string(doc)}}})
	return string(b)
}

func TestReportStatusWritesOnlyWhatChanged(t *testing.T) {
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	fresh := LiveStatus{Source: StatusSource, App: "apps", SyncStatus: "Synced", HealthStatus: "Healthy", OperationPhase: "Succeeded", Revision: d2, ObservedAt: now.Format(time.RFC3339)}
	for _, tc := range []struct {
		name string
		held *LiveStatus
		dry  bool
		did  string
	}{
		{"nothing held", nil, false, "written"},
		{"same, recent", &LiveStatus{Source: fresh.Source, App: "apps", SyncStatus: "Synced", HealthStatus: "Healthy", OperationPhase: "Succeeded", Revision: d2, ObservedAt: now.Add(-time.Minute).Format(time.RFC3339)}, false, "unchanged"},
		{"same, old enough to refresh", &LiveStatus{Source: fresh.Source, App: "apps", SyncStatus: "Synced", HealthStatus: "Healthy", OperationPhase: "Succeeded", Revision: d2, ObservedAt: now.Add(-time.Hour).Format(time.RFC3339)}, false, "written"},
		{"changed", &LiveStatus{Source: fresh.Source, App: "apps", SyncStatus: "OutOfSync", HealthStatus: "Healthy", OperationPhase: "Succeeded", Revision: d1, ObservedAt: now.Add(-time.Minute).Format(time.RFC3339)}, false, "written"},
		{"dry run", nil, true, "dry-run"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var wrote []string
			write := func(space string, patch []byte) error {
				wrote = append(wrote, space+" "+string(patch))
				return nil
			}
			run := fake(map[string]string{"space get": heldSpace(t, tc.held)})
			outs, err := ReportStatus(run, write, []Reading{{Check: Check{Kustomization: "apps", Space: "flux-apps-dev"}, Status: fresh}}, 10*time.Minute, tc.dry, now)
			if err != nil {
				t.Fatal(err)
			}
			if outs[0].Did != tc.did {
				t.Errorf("want %s, got %s", tc.did, outs[0].Did)
			}
			if (tc.did == "written") != (len(wrote) == 1) {
				t.Errorf("wrote %d times for %s", len(wrote), tc.did)
			}
			if len(wrote) == 1 && (!strings.Contains(wrote[0], `"Annotations":{"confighub.com/live-status":`) || strings.Contains(wrote[0], "Slug")) {
				t.Errorf("the patch must set only the live-status annotation: %s", wrote[0])
			}
		})
	}
}

// --require Healthy: the onboarding releases run before anything reads
// ConfigHub, so they cannot wait for a live status. They are made under a
// workflow without it, and the workflow is replaced only after every one of
// them, so each change from then on waits.
func TestRequireHealthyAfterOnboarding(t *testing.T) {
	in, err := Load(nil, []string{"../../../gitops/flux/beginner"})
	if err != nil {
		t.Fatal(err)
	}
	p, err := Build(in, Options{Prefix: "flux", RepoRoot: repoRoot(t), Require: []string{"Healthy"}})
	if err != nil {
		t.Fatal(err)
	}
	c := p.Components[0]
	if !strings.Contains(Workflow(c), "- Healthy") || strings.Contains(Workflow(c.onboarding()), "- Healthy") {
		t.Error("the workflow should require Healthy, and the onboarding one should not")
	}
	s := ApplyScript(p, "flux", ".")
	create := strings.Index(s, "--filename "+c.Name+"/"+onboardingWorkflow)
	lastPublish := strings.LastIndex(s, "\n  publish ")
	replace := strings.Index(s, "cub changeworkflow update --space "+c.Base+" rollout --filename "+c.Name+"/change-workflow.yaml")
	if create < 0 || lastPublish < 0 || replace < 0 || !(create < lastPublish && lastPublish < replace) {
		t.Errorf("want create from the onboarding file, every publish, then the replacement; got %d, %d, %d", create, lastPublish, replace)
	}
	if _, err := Build(in, Options{Prefix: "flux", RepoRoot: repoRoot(t), Require: []string{"Validated"}}); err == nil {
		t.Error("only Healthy is offered")
	}
}

// After a handback to Git, the Space still holds the last reading, and the
// Healthy gate would pass on it. A reading this reporter wrote is replaced;
// another reporter's is left alone.
func TestLeftSpaceReplacesOurStaleReading(t *testing.T) {
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	gone := Reading{Check: Check{Kustomization: "apps", Space: "flux-apps-dev"}, Skip: "reads GitRepository fleet-repo, not ConfigHub"}
	ours := &LiveStatus{Source: StatusSource, App: "apps", SyncStatus: "Synced", HealthStatus: "Healthy", OperationPhase: "Succeeded", Revision: d2, ObservedAt: now.Format(time.RFC3339)}
	theirs := &LiveStatus{Source: "argobot", App: "apps", SyncStatus: "Synced", HealthStatus: "Healthy", OperationPhase: "Succeeded", ObservedAt: now.Format(time.RFC3339)}
	for _, tc := range []struct {
		name string
		held *LiveStatus
		did  string
	}{{"ours", ours, "written"}, {"another reporter's", theirs, "skipped"}, {"nothing held", nil, "skipped"}} {
		t.Run(tc.name, func(t *testing.T) {
			var wrote []byte
			write := func(_ string, patch []byte) error { wrote = patch; return nil }
			outs, err := ReportStatus(fake(map[string]string{"space get": heldSpace(t, tc.held)}), write, []Reading{gone}, 10*time.Minute, false, now)
			if err != nil {
				t.Fatal(err)
			}
			if outs[0].Did != tc.did {
				t.Fatalf("want %s, got %s", tc.did, outs[0].Did)
			}
			if tc.did == "written" {
				if outs[0].Status.Gate() || !strings.Contains(string(wrote), "no longer reads this Space") {
					t.Errorf("the replacement must not pass the gate and must say why: %s", wrote)
				}
			}
		})
	}
}

// One Space that cannot be read must not stop the others being reported.
func TestReportStatusCarriesOnPastAnUnreadableSpace(t *testing.T) {
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	st := LiveStatus{Source: StatusSource, SyncStatus: "Synced", HealthStatus: "Healthy", OperationPhase: "Succeeded", ObservedAt: now.Format(time.RFC3339)}
	run := func(name string, args ...string) ([]byte, error) {
		if strings.Contains(strings.Join(args, " "), "space get gone") {
			return nil, fmt.Errorf("space gone not found")
		}
		return []byte(heldSpace(t, nil)), nil
	}
	var wrote []string
	write := func(space string, _ []byte) error { wrote = append(wrote, space); return nil }
	outs, err := ReportStatus(run, write, []Reading{
		{Check: Check{Kustomization: "a", Space: "gone"}, Status: st},
		{Check: Check{Kustomization: "b", Space: "here"}, Status: st},
	}, time.Minute, false, now)
	if err == nil || !strings.Contains(err.Error(), "gone") {
		t.Errorf("the unreadable Space should be reported: %v", err)
	}
	if len(wrote) != 1 || wrote[0] != "here" || len(outs) != 1 {
		t.Errorf("the next Space should still be written: wrote %v, outcomes %v", wrote, outs)
	}
}
