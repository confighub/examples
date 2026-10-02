package argo

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

// Releases 1 and 2 are published; 3 is not.
var releases = []HubRelease{
	{Num: 1, ManifestDigest: d1, Published: true},
	{Num: 2, ManifestDigest: d2, Published: true},
	{Num: 3, ManifestDigest: dX},
}

const spaceURL = "oci://gw.example/space/argo-apptique-dev"

// app builds an Application's JSON, as Argo CD reports one handed over to
// ConfigHub and synced at release 2, unless a test says otherwise.
func app(t *testing.T, mutate func(a map[string]any)) string {
	t.Helper()
	src := map[string]any{"repoURL": spaceURL, "targetRevision": "latest", "path": "."}
	a := map[string]any{
		"spec": map[string]any{"source": src},
		"status": map[string]any{
			"sync": map[string]any{
				"status":     "Synced",
				"revision":   d2,
				"comparedTo": map[string]any{"source": map[string]any{"repoURL": spaceURL, "targetRevision": "latest", "path": "."}},
			},
			"health":         map[string]any{"status": "Healthy"},
			"operationState": map[string]any{"phase": "Succeeded", "message": "successfully synced (all tasks run)"},
		},
	}
	if mutate != nil {
		mutate(a)
	}
	b, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func appStatusOf(a map[string]any) map[string]any { return a["status"].(map[string]any) }
func syncOf(a map[string]any) map[string]any      { return appStatusOf(a)["sync"].(map[string]any) }

var statusCheck = StatusCheck{Application: "apptique-dev", Space: "argo-apptique-dev"}

func readApp(t *testing.T, ajson string) Reading {
	t.Helper()
	run := fake(map[string]string{"get application": ajson})
	r, err := ReadStatus(run, &fakeHub{releases: releases}, statusCheck, time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestArgoStatusMapping(t *testing.T) {
	for _, tc := range []struct {
		name                     string
		mutate                   func(a map[string]any)
		sync, health, phase, rev string
		gate                     bool
		says                     string
	}{
		{"newest release synced and healthy", nil,
			"Synced", "Healthy", "Succeeded", d2, true, "release 2 synced"},
		{"an older release synced: Argo has not read the gateway again yet", func(a map[string]any) {
			syncOf(a)["revision"] = d1
		}, "OutOfSync", "Healthy", "Succeeded", d1, false, "release 1 is synced; release 2 is published"},
		{"an unpublished digest synced", func(a map[string]any) {
			syncOf(a)["revision"] = dX
		}, "Unknown", "Healthy", "Succeeded", dX, false, "no published release"},
		{"Argo says OutOfSync", func(a map[string]any) {
			syncOf(a)["status"] = "OutOfSync"
			appStatusOf(a)["health"] = map[string]any{"status": "Progressing", "message": "Waiting for rollout"}
		}, "OutOfSync", "Progressing", "Succeeded", d2, false, "Waiting for rollout"},
		{"degraded", func(a map[string]any) {
			appStatusOf(a)["health"] = map[string]any{"status": "Degraded", "message": "Deployment has timed out progressing"}
		}, "Synced", "Degraded", "Succeeded", d2, false, "release 2 synced"},
		{"Argo names what is out of sync, not the last operation's success", func(a map[string]any) {
			syncOf(a)["status"] = "OutOfSync"
			appStatusOf(a)["resources"] = []any{
				map[string]any{"kind": "Deployment", "namespace": "apptique-dev", "name": "frontend", "status": "OutOfSync"},
				map[string]any{"kind": "Service", "namespace": "apptique-dev", "name": "frontend", "status": "Synced"},
				map[string]any{"kind": "Application", "namespace": "argocd", "name": "a", "status": "OutOfSync"},
				map[string]any{"kind": "Application", "namespace": "argocd", "name": "b", "status": "OutOfSync"},
				map[string]any{"kind": "Application", "namespace": "argocd", "name": "c", "status": "OutOfSync"},
			}
		}, "OutOfSync", "Healthy", "Succeeded", d2, false, "4 out of sync: Deployment apptique-dev/frontend, Application argocd/a, Application argocd/b and 1 more"},
		{"a failed sync", func(a map[string]any) {
			syncOf(a)["status"] = "OutOfSync"
			appStatusOf(a)["operationState"] = map[string]any{"phase": "Failed", "message": "one or more objects failed to apply"}
		}, "OutOfSync", "Healthy", "Failed", d2, false, "failed to apply"},
		{"a comparison error", func(a map[string]any) {
			syncOf(a)["status"] = "Unknown"
			appStatusOf(a)["conditions"] = []any{map[string]any{"type": "ComparisonError", "message": "failed to fetch oci manifest"}}
		}, "Unknown", "Healthy", "Succeeded", d2, false, "failed to fetch oci manifest"},
		{"an error condition while Argo still says Synced", func(a map[string]any) {
			appStatusOf(a)["conditions"] = []any{map[string]any{"type": "ComparisonError", "message": "rpc error: manifest unknown"}}
		}, "Unknown", "Healthy", "Succeeded", d2, false, "manifest unknown"},
		{"a warning condition is not a problem", func(a map[string]any) {
			appStatusOf(a)["conditions"] = []any{map[string]any{"type": "OrphanedResourceWarning", "message": "1 orphaned resource"}}
		}, "Synced", "Healthy", "Succeeded", d2, true, "release 2 synced"},
		{"still compared with the Git source it had before the repoint", func(a map[string]any) {
			syncOf(a)["comparedTo"] = map[string]any{"source": map[string]any{"repoURL": "https://github.com/acme/fleet", "targetRevision": "main", "path": "apps/apptique/overlays/dev"}}
		}, "Unknown", "Healthy", "Succeeded", "", false, "has not compared the Application with its ConfigHub source"},
		{"no sync operation yet", func(a map[string]any) {
			delete(appStatusOf(a), "operationState")
		}, "Synced", "Healthy", "", d2, false, "run no sync operation"},
		{"no health reported", func(a map[string]any) {
			delete(appStatusOf(a), "health")
		}, "Synced", "Unknown", "Succeeded", d2, false, "release 2 synced"},
		{"multi-source, one source reads the Space", func(a map[string]any) {
			other := map[string]any{"repoURL": "https://charts.example", "targetRevision": "1.0.0"}
			own := map[string]any{"repoURL": spaceURL, "targetRevision": "latest", "path": "."}
			a["spec"] = map[string]any{"sources": []any{other, own}}
			syncOf(a)["comparedTo"] = map[string]any{"sources": []any{other, own}}
		}, "Synced", "Healthy", "Succeeded", d2, true, "release 2 synced"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := readApp(t, app(t, tc.mutate)).Status
			if s.SyncStatus != tc.sync || s.HealthStatus != tc.health || s.OperationPhase != tc.phase || s.Revision != tc.rev {
				t.Errorf("want %s/%s/%s at %q, got %s/%s/%s at %q", tc.sync, tc.health, tc.phase, tc.rev, s.SyncStatus, s.HealthStatus, s.OperationPhase, s.Revision)
			}
			if s.Gate() != tc.gate {
				t.Errorf("gate: want %v, got %v for %s", tc.gate, s.Gate(), s)
			}
			if !strings.Contains(s.Message, tc.says) {
				t.Errorf("message %q should say %q", s.Message, tc.says)
			}
			if s.Source != StatusSource || s.App != "apptique-dev" || s.ObservedAt != "2026-09-30T12:00:00Z" {
				t.Errorf("source, app or time wrong: %+v", s)
			}
		})
	}
}

// Only an Application reading this Space's release is reported: one still on
// Git, or reading another Space, is not ConfigHub's to speak for.
func TestArgoStatusSkipsApplicationsNotReadingTheSpace(t *testing.T) {
	onGit := app(t, func(a map[string]any) {
		a["spec"] = map[string]any{"source": map[string]any{"repoURL": "https://github.com/acme/fleet", "targetRevision": "main", "path": "apps/apptique"}}
	})
	if r := readApp(t, onGit); !strings.Contains(r.Skip, "https://github.com/acme/fleet, not Space argo-apptique-dev") {
		t.Errorf("an Application on Git should be skipped: %+v", r)
	}
	other := app(t, func(a map[string]any) {
		a["spec"] = map[string]any{"source": map[string]any{"repoURL": "oci://gw.example/space/argo-apptique-dev-2", "targetRevision": "latest", "path": "."}}
	})
	if r := readApp(t, other); r.Skip == "" {
		t.Errorf("a Space whose name only starts the same is another Space: %+v", r)
	}
}

// A read that fails writes nothing: a guess could open a gate.
func TestArgoStatusReadFailureIsNotAReading(t *testing.T) {
	run := fake(map[string]string{"get application": app(t, nil)})
	if _, err := ReadStatus(run, &fakeHub{releasesErr: fmt.Errorf("no such space")}, statusCheck, time.Now()); err == nil {
		t.Error("a failed release list must be an error, not a reading")
	}
	if _, err := ReadStatus(fake(nil), &fakeHub{releases: releases}, statusCheck, time.Now()); err == nil {
		t.Error("a failed Application read must be an error, not a reading")
	}
}

func TestArgoStatusMessageIsClipped(t *testing.T) {
	r := readApp(t, app(t, func(a map[string]any) {
		appStatusOf(a)["conditions"] = []any{map[string]any{"type": "SyncError", "message": strings.Repeat("x", 2000)}}
	}))
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

// holding is a ConfigHub whose Space argo-apptique-dev holds that reading, or
// none.
func holding(s *LiveStatus) *fakeHub {
	ann := map[string]string{"other": "kept"}
	if s != nil {
		doc, _ := json.Marshal(s)
		ann = map[string]string{LiveStatusAnnotation: string(doc)}
	}
	return &fakeHub{annotations: map[string]map[string]string{"argo-apptique-dev": ann}}
}

func TestArgoReportStatusWritesOnlyWhatChanged(t *testing.T) {
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	reading := LiveStatus{Source: StatusSource, App: "apptique-dev", SyncStatus: "Synced", HealthStatus: "Healthy", OperationPhase: "Succeeded", Revision: d2, ObservedAt: now.Format(time.RFC3339)}
	held := func(source, sync string, age time.Duration) *LiveStatus {
		h := reading
		h.Source, h.SyncStatus, h.ObservedAt = source, sync, now.Add(-age).Format(time.RFC3339)
		return &h
	}
	for _, tc := range []struct {
		name string
		held *LiveStatus
		dry  bool
		did  string
	}{
		{"nothing held", nil, false, "written"},
		{"same, recent", held(StatusSource, "Synced", time.Minute), false, "unchanged"},
		{"same, old enough to refresh", held(StatusSource, "Synced", time.Hour), false, "written"},
		{"changed", held(StatusSource, "OutOfSync", time.Minute), false, "written"},
		{"argobot reporting", held("argobot", "OutOfSync", time.Minute), false, "left"},
		{"argobot stopped long ago", held("argobot", "OutOfSync", time.Hour), false, "written"},
		{"dry run", nil, true, "dry-run"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hub := holding(tc.held)
			outs, err := ReportStatus(hub, []Reading{{Check: statusCheck, Status: reading}}, 10*time.Minute, tc.dry, now)
			if err != nil {
				t.Fatal(err)
			}
			wrote := hub.patches
			if outs[0].Did != tc.did {
				t.Errorf("want %s, got %s", tc.did, outs[0].Did)
			}
			if (tc.did == "written") != (len(wrote) == 1) {
				t.Errorf("wrote %d times for %s", len(wrote), tc.did)
			}
			if len(wrote) == 1 && (!strings.Contains(wrote[0], `"Annotations":{"confighub.com/live-status":`) || strings.Contains(wrote[0], "Slug")) {
				t.Errorf("the patch must set only the live-status annotation: %s", wrote[0])
			}
			if tc.did == "left" && !strings.Contains(outs[0].Why, "argobot") {
				t.Errorf("say whose reading was left alone: %q", outs[0].Why)
			}
		})
	}
}

// After a handback to Git, the Space still holds the last reading, and the
// Healthy gate would pass on it. A reading this reporter wrote is replaced;
// another reporter's is left alone.
func TestArgoLeftSpaceReplacesOurStaleReading(t *testing.T) {
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	gone := Reading{Check: statusCheck, Skip: "reads https://github.com/acme/fleet, not Space argo-apptique-dev"}
	ours := &LiveStatus{Source: StatusSource, App: "apptique-dev", SyncStatus: "Synced", HealthStatus: "Healthy", OperationPhase: "Succeeded", Revision: d2, ObservedAt: now.Format(time.RFC3339)}
	theirs := &LiveStatus{Source: "argobot", App: "apptique-dev", SyncStatus: "Synced", HealthStatus: "Healthy", OperationPhase: "Succeeded", ObservedAt: now.Add(-time.Hour).Format(time.RFC3339)}
	for _, tc := range []struct {
		name string
		held *LiveStatus
		did  string
	}{{"ours", ours, "written"}, {"another reporter's", theirs, "skipped"}, {"nothing held", nil, "skipped"}} {
		t.Run(tc.name, func(t *testing.T) {
			hub := holding(tc.held)
			outs, err := ReportStatus(hub, []Reading{gone}, 10*time.Minute, false, now)
			if err != nil {
				t.Fatal(err)
			}
			wrote := strings.Join(hub.patches, "")
			if outs[0].Did != tc.did {
				t.Fatalf("want %s, got %s", tc.did, outs[0].Did)
			}
			if tc.did == "written" {
				if outs[0].Status.Gate() || !strings.Contains(wrote, "no longer reads this Space") {
					t.Errorf("the replacement must not pass the gate and must say why: %s", wrote)
				}
			}
		})
	}
}

// One Space that cannot be read must not stop the others being reported.
func TestArgoReportStatusCarriesOnPastAnUnreadableSpace(t *testing.T) {
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	st := LiveStatus{Source: StatusSource, SyncStatus: "Synced", HealthStatus: "Healthy", OperationPhase: "Succeeded", ObservedAt: now.Format(time.RFC3339)}
	hub := &fakeHub{unreadable: map[string]bool{"gone": true}}
	outs, err := ReportStatus(hub, []Reading{
		{Check: StatusCheck{Application: "a", Space: "gone"}, Status: st},
		{Check: StatusCheck{Application: "b", Space: "here"}, Status: st},
	}, time.Minute, false, now)
	if err == nil || !strings.Contains(err.Error(), "gone") {
		t.Errorf("the unreadable Space should be reported: %v", err)
	}
	if wrote := hub.patches; len(wrote) != 1 || !strings.HasPrefix(wrote[0], "here ") || len(outs) != 1 {
		t.Errorf("the next Space should still be written: wrote %v, outcomes %v", wrote, outs)
	}
}

// Given the repository, status reports every variant and every app of apps
// whose children moved into a control Space: the same set handover.sh moves.
func TestStatusChecksCoverVariantsAndControlSpaces(t *testing.T) {
	opts := staged
	opts.RepoRoot = repoRoot(t)
	opts.Prefix = "argo"
	p := planOf(t, example, opts)
	checks := StatusChecksFor(p, "argo")
	variants := ChecksFor(p)
	controls := p.controlSpaces("argo")
	if len(controls) == 0 || len(variants) == 0 {
		t.Fatalf("the expert example has both control Spaces and variants: %d, %d", len(controls), len(variants))
	}
	if len(checks) != len(controls)+len(variants) {
		t.Errorf("want %d checks, got %d", len(controls)+len(variants), len(checks))
	}
	seen := map[string]string{}
	for _, c := range checks {
		seen[c.Application] = c.Space
	}
	for _, cs := range controls {
		if seen[cs.Parent] != cs.Space {
			t.Errorf("app of apps %s should report to %s, got %q", cs.Parent, cs.Space, seen[cs.Parent])
		}
	}
}

// Argo CD caches the digest behind "latest". A reading that shows an older
// release synced asks for one hard refresh per release, not one per pass.
func TestHardRefreshOncePerRelease(t *testing.T) {
	behind := readApp(t, app(t, func(a map[string]any) { syncOf(a)["revision"] = d1 }))
	if behind.Unread != d2 {
		t.Fatalf("release 2 is unread: %+v", behind)
	}
	if current := readApp(t, app(t, nil)); current.Unread != "" {
		t.Errorf("nothing is unread at the newest release: %+v", current)
	}
	var asked []string
	f := &Refresher{Run: func(name string, args ...string) ([]byte, error) {
		asked = append(asked, name+" "+strings.Join(args, " "))
		return nil, nil
	}}
	for pass := 0; pass < 3; pass++ {
		if _, err := f.Refresh([]Reading{behind}); err != nil {
			t.Fatal(err)
		}
	}
	if len(asked) != 1 || !strings.Contains(asked[0], "annotate application apptique-dev argocd.argoproj.io/refresh=hard --overwrite") {
		t.Errorf("want one hard refresh, got %v", asked)
	}
	behind.Unread = dX
	if did, _ := f.Refresh([]Reading{behind}); len(did) != 1 {
		t.Errorf("a newer release is asked for again: %v", did)
	}
}
