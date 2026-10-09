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
		name                    string
		mutate                  func(a map[string]any)
		sync, health, operation string
		release                 int // where it is recorded; 0 is nowhere
		gate                    bool
		says                    string
	}{
		{"newest release synced and healthy", nil,
			"Synced", "Healthy", "Succeeded", 2, true, "release 2 synced"},
		{"an older release synced: Argo has not read the gateway again yet", func(a map[string]any) {
			syncOf(a)["revision"] = d1
		}, "Synced", "Healthy", "Succeeded", 1, false, "release 1 synced; release 2 is published and Argo CD has not read it"},
		{"an unpublished digest synced", func(a map[string]any) {
			syncOf(a)["revision"] = dX
		}, "Synced", "Healthy", "Succeeded", 0, false, "no published release"},
		{"Argo says OutOfSync", func(a map[string]any) {
			syncOf(a)["status"] = "OutOfSync"
			appStatusOf(a)["health"] = map[string]any{"status": "Progressing", "message": "Waiting for rollout"}
		}, "OutOfSync", "Progressing", "Succeeded", 2, false, "Waiting for rollout"},
		{"degraded", func(a map[string]any) {
			appStatusOf(a)["health"] = map[string]any{"status": "Degraded", "message": "Deployment has timed out progressing"}
		}, "Synced", "Degraded", "Succeeded", 2, false, "release 2 synced"},
		{"Argo names what is out of sync, not the last operation's success", func(a map[string]any) {
			syncOf(a)["status"] = "OutOfSync"
			appStatusOf(a)["resources"] = []any{
				map[string]any{"kind": "Deployment", "namespace": "apptique-dev", "name": "frontend", "status": "OutOfSync"},
				map[string]any{"kind": "Service", "namespace": "apptique-dev", "name": "frontend", "status": "Synced"},
				map[string]any{"kind": "Application", "namespace": "argocd", "name": "a", "status": "OutOfSync"},
				map[string]any{"kind": "Application", "namespace": "argocd", "name": "b", "status": "OutOfSync"},
				map[string]any{"kind": "Application", "namespace": "argocd", "name": "c", "status": "OutOfSync"},
			}
		}, "OutOfSync", "Healthy", "Succeeded", 2, false, "4 out of sync: Deployment apptique-dev/frontend, Application argocd/a, Application argocd/b and 1 more"},
		{"a failed sync", func(a map[string]any) {
			syncOf(a)["status"] = "OutOfSync"
			appStatusOf(a)["operationState"] = map[string]any{"phase": "Failed", "message": "one or more objects failed to apply"}
		}, "OutOfSync", "Healthy", "Failed", 2, false, "failed to apply"},
		{"Argo's Error phase is a failed operation", func(a map[string]any) {
			appStatusOf(a)["operationState"] = map[string]any{"phase": "Error", "message": "context deadline exceeded"}
		}, "Unknown", "Healthy", "Failed", 2, false, "context deadline exceeded"},
		{"a sync still running", func(a map[string]any) {
			appStatusOf(a)["operationState"] = map[string]any{"phase": "Running", "message": "waiting for healthy state"}
		}, "Synced", "Healthy", "Running", 2, false, "release 2 synced"},
		{"a comparison error", func(a map[string]any) {
			syncOf(a)["status"] = "Unknown"
			appStatusOf(a)["conditions"] = []any{map[string]any{"type": "ComparisonError", "message": "failed to fetch oci manifest"}}
		}, "Unknown", "Healthy", "Succeeded", 2, false, "failed to fetch oci manifest"},
		{"an error condition while Argo still says Synced", func(a map[string]any) {
			appStatusOf(a)["conditions"] = []any{map[string]any{"type": "ComparisonError", "message": "rpc error: manifest unknown"}}
		}, "Unknown", "Healthy", "Succeeded", 2, false, "manifest unknown"},
		{"a warning condition is not a problem", func(a map[string]any) {
			appStatusOf(a)["conditions"] = []any{map[string]any{"type": "OrphanedResourceWarning", "message": "1 orphaned resource"}}
		}, "Synced", "Healthy", "Succeeded", 2, true, "release 2 synced"},
		{"still compared with the Git source it had before the repoint", func(a map[string]any) {
			syncOf(a)["comparedTo"] = map[string]any{"source": map[string]any{"repoURL": "https://github.com/acme/fleet", "targetRevision": "main", "path": "apps/apptique/overlays/dev"}}
		}, "Unknown", "Healthy", "Succeeded", 0, false, "has not compared the Application with its ConfigHub source"},
		{"no sync operation yet: nothing is running or failed, so the gate passes", func(a map[string]any) {
			delete(appStatusOf(a), "operationState")
		}, "Synced", "Healthy", "", 2, true, "release 2 synced"},
		{"no health reported", func(a map[string]any) {
			delete(appStatusOf(a), "health")
		}, "Synced", "Unknown", "Succeeded", 2, false, "release 2 synced"},
		{"multi-source, one source reads the Space", func(a map[string]any) {
			other := map[string]any{"repoURL": "https://charts.example", "targetRevision": "1.0.0"}
			own := map[string]any{"repoURL": spaceURL, "targetRevision": "latest", "path": "."}
			a["spec"] = map[string]any{"sources": []any{other, own}}
			syncOf(a)["comparedTo"] = map[string]any{"sources": []any{other, own}}
		}, "Synced", "Healthy", "Succeeded", 2, true, "release 2 synced"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := readApp(t, app(t, tc.mutate))
			s := r.Status
			if s.Sync != tc.sync || s.Health != tc.health || s.Operation != tc.operation || r.Release != tc.release {
				t.Errorf("want %s/%s/%s on release %d, got %s/%s/%s on release %d", tc.sync, tc.health, tc.operation, tc.release, s.Sync, s.Health, s.Operation, r.Release)
			}
			// The gate reads the newest release only.
			if gate := s.Gate() && r.Release == r.Newest; gate != tc.gate {
				t.Errorf("gate: want %v, got %v for %s on release %d of %d", tc.gate, gate, s, r.Release, r.Newest)
			}
			if says := s.Message + " " + r.Unrecorded; !strings.Contains(says, tc.says) {
				t.Errorf("%q should say %q", says, tc.says)
			}
			if (r.Release == 0) != (r.Unrecorded != "") {
				t.Errorf("a reading with no release to record it on says why, and only then: %+v", r)
			}
			if s.Reporter != StatusReporter || s.DataSource != "apptique-dev" || s.ObservedAt != "2026-09-30T12:00:00Z" || r.Newest != 2 {
				t.Errorf("reporter, source, time or newest release wrong: %+v", r)
			}
		})
	}
}

// Argo CD's own words are kept beside ConfigHub's, including when the
// normalized one is held back.
func TestArgoStatusKeepsArgosOwnWords(t *testing.T) {
	r := readApp(t, app(t, func(a map[string]any) {
		appStatusOf(a)["operationState"] = map[string]any{"phase": "Error", "message": "boom"}
	}))
	if s := r.Status; s.ReporterSync != "Synced" || s.ReporterHealth != "Healthy" || s.ReporterOperation != "Error" || s.Sync != "Unknown" || s.Operation != "Failed" {
		t.Errorf("want Argo's Synced/Healthy/Error beside Unknown and Failed: %+v", s)
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
	if len(r.Status.Message) > statusMessageLimit {
		t.Errorf("a message is clipped to %d bytes; this is %d", statusMessageLimit, len(r.Status.Message))
	}
}

// at is a reading of release 2, the newest, and what that Release holds.
func at(status LiveStatus, held *LiveStatus) Reading {
	return Reading{Check: statusCheck, Status: status, Revision: d2, Release: 2, Newest: 2, Held: held}
}

func TestArgoReportStatusWritesOnlyWhatChanged(t *testing.T) {
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	reading := LiveStatus{Reporter: StatusReporter, DataSource: "apptique-dev", Sync: "Synced", Health: "Healthy", Operation: "Succeeded", ObservedAt: now.Format(time.RFC3339)}
	held := func(reporter, sync string, age time.Duration) *LiveStatus {
		h := reading
		h.Reporter, h.Sync, h.ObservedAt = reporter, sync, now.Add(-age).Format(time.RFC3339)
		return &h
	}
	for _, tc := range []struct {
		name string
		held *LiveStatus
		dry  bool
		did  string
	}{
		{"nothing held", nil, false, "written"},
		{"same, recent", held(StatusReporter, "Synced", time.Minute), false, "unchanged"},
		{"same, old enough to refresh", held(StatusReporter, "Synced", time.Hour), false, "written"},
		{"changed", held(StatusReporter, "OutOfSync", time.Minute), false, "written"},
		{"argobot reporting", held("argobot", "OutOfSync", time.Minute), false, "left"},
		{"argobot stopped long ago", held("argobot", "OutOfSync", time.Hour), false, "written"},
		{"dry run", nil, true, "dry-run"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hub := &fakeHub{}
			outs, err := ReportStatus(hub, []Reading{at(reading, tc.held)}, 10*time.Minute, tc.dry, now)
			if err != nil {
				t.Fatal(err)
			}
			if outs[0].Did != tc.did {
				t.Errorf("want %s, got %s", tc.did, outs[0].Did)
			}
			wrote := hub.recorded
			if (tc.did == "written") != (len(wrote) == 1) {
				t.Errorf("wrote %d times for %s", len(wrote), tc.did)
			}
			if len(wrote) == 1 && !strings.HasPrefix(wrote[0], `argo-apptique-dev release 2 {"reporter":"cub-argo"`) {
				t.Errorf("the status goes on release 2 of the Space, as ours: %s", wrote[0])
			}
			if tc.did == "left" && !strings.Contains(outs[0].Why, "argobot") {
				t.Errorf("say whose reading was left alone: %q", outs[0].Why)
			}
		})
	}
}

// A reading with no Release to record it on is written nowhere, and said.
func TestArgoReportStatusRecordsNothingWithoutARelease(t *testing.T) {
	hub := &fakeHub{}
	r := Reading{Check: statusCheck, Status: LiveStatus{Reporter: StatusReporter, Sync: "Unknown", Health: "Healthy"}, Newest: 2,
		Unrecorded: "Argo CD has not compared the Application with its ConfigHub source yet"}
	outs, err := ReportStatus(hub, []Reading{r}, time.Minute, false, time.Now())
	if err != nil || len(hub.recorded) != 0 || outs[0].Did != "unrecorded" {
		t.Fatalf("want nothing recorded: %v %v %+v", err, hub.recorded, outs)
	}
	var b strings.Builder
	PrintOutcomes(&b, outs)
	if !strings.Contains(b.String(), "not recorded") || !strings.Contains(b.String(), "has not compared") || !strings.Contains(b.String(), "gate would not pass") {
		t.Errorf("say that nothing was recorded, why, and what it means for the gate: %s", b.String())
	}
}

// A reading recorded on an older release is true of that release, and the gate
// reads the newest: say so rather than "would pass".
func TestArgoOutcomeOnAnOlderReleaseDoesNotPassTheGate(t *testing.T) {
	r := readApp(t, app(t, func(a map[string]any) { syncOf(a)["revision"] = d1 }))
	outs, err := ReportStatus(&fakeHub{}, []Reading{r}, time.Minute, false, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	PrintOutcomes(&b, outs)
	if !strings.Contains(b.String(), "release 1: Synced/Healthy") || !strings.Contains(b.String(), "would not pass: it reads release 2") {
		t.Errorf("want release 1 reported and the gate explained: %s", b.String())
	}
}

// After a handback to Git, the newest Release still holds the last reading,
// and the Healthy gate would pass on it. A reading this reporter wrote is
// replaced; another reporter's is left alone.
func TestArgoLeftSpaceReplacesOurStaleReading(t *testing.T) {
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	ours := &LiveStatus{Reporter: StatusReporter, DataSource: "apptique-dev", Sync: "Synced", Health: "Healthy", Operation: "Succeeded", ObservedAt: now.Format(time.RFC3339)}
	theirs := &LiveStatus{Reporter: "argobot", DataSource: "apptique-dev", Sync: "Synced", Health: "Healthy", Operation: "Succeeded", ObservedAt: now.Add(-time.Hour).Format(time.RFC3339)}
	for _, tc := range []struct {
		name string
		held *LiveStatus
		did  string
	}{{"ours", ours, "written"}, {"another reporter's", theirs, "skipped"}, {"nothing held", nil, "skipped"}} {
		t.Run(tc.name, func(t *testing.T) {
			hub := &fakeHub{}
			gone := Reading{Check: statusCheck, Skip: "reads https://github.com/acme/fleet, not Space argo-apptique-dev", Release: 2, Newest: 2, Held: tc.held}
			outs, err := ReportStatus(hub, []Reading{gone}, 10*time.Minute, false, now)
			if err != nil {
				t.Fatal(err)
			}
			if outs[0].Did != tc.did {
				t.Fatalf("want %s, got %s", tc.did, outs[0].Did)
			}
			if tc.did == "written" {
				wrote := strings.Join(hub.recorded, "")
				if outs[0].Status.Gate() || !strings.Contains(wrote, "release 2 ") || !strings.Contains(wrote, "no longer reads this Space") {
					t.Errorf("the replacement goes on the newest release, must not pass the gate, and must say why: %s", wrote)
				}
			}
		})
	}
}

// What the newest Release holds is read for an Application that has left its
// Space, so a reading of ours left behind there can be found.
func TestArgoStatusOfALeftSpaceNamesTheNewestRelease(t *testing.T) {
	ours := &LiveStatus{Reporter: StatusReporter, Sync: "Synced", Health: "Healthy"}
	withStatus := []HubRelease{releases[0], {Num: 2, ManifestDigest: d2, Published: true, Live: ours}, releases[2]}
	onGit := app(t, func(a map[string]any) {
		a["spec"] = map[string]any{"source": map[string]any{"repoURL": "https://github.com/acme/fleet", "targetRevision": "main", "path": "apps/apptique"}}
	})
	r, err := ReadStatus(fake(map[string]string{"get application": onGit}), &fakeHub{releases: withStatus}, statusCheck, time.Now())
	if err != nil || r.Skip == "" || r.Release != 2 || r.Held != ours {
		t.Errorf("want the skip, and release 2 with what it holds: %+v %v", r, err)
	}
}

// One Release that cannot be written must not stop the others being reported.
func TestArgoReportStatusCarriesOnPastAnUnwritableSpace(t *testing.T) {
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	st := LiveStatus{Reporter: StatusReporter, Sync: "Synced", Health: "Healthy", Operation: "Succeeded", ObservedAt: now.Format(time.RFC3339)}
	hub := &fakeHub{unwritable: map[string]bool{"gone": true}}
	outs, err := ReportStatus(hub, []Reading{
		{Check: StatusCheck{Application: "a", Space: "gone"}, Status: st, Release: 1, Newest: 1},
		{Check: StatusCheck{Application: "b", Space: "here"}, Status: st, Release: 1, Newest: 1},
	}, time.Minute, false, now)
	if err == nil || !strings.Contains(err.Error(), "gone") {
		t.Errorf("the unwritable Space should be reported: %v", err)
	}
	if wrote := hub.recorded; len(wrote) != 1 || !strings.HasPrefix(wrote[0], "here ") || len(outs) != 1 {
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
