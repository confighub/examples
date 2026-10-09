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

// Releases 1 and 2 are published; 3 is not.
var releases = []HubRelease{
	{Num: 1, ManifestDigest: d1, Published: true},
	{Num: 2, ManifestDigest: d2, Published: true},
	{Num: 3, ManifestDigest: dX},
}

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
	})
	r, err := ReadStatus(run, &fakeHub{releases: releases}, Check{Kustomization: "apps", Space: "flux-apps-dev"}, time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

const spaceURL = "oci://gw.example/space/flux-apps-dev"

func TestStatusMapping(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		mutate                  func(k map[string]any)
		sync, health, operation string
		release                 int // where it is recorded; 0 is nowhere
		gate                    bool
		says                    string
	}{
		{"newest release applied, workloads checked", nil,
			"Synced", "Healthy", "Succeeded", 2, true, "release 2 applied"},
		{"an older release applied", func(k map[string]any) {
			status(k)["lastAppliedRevision"] = "latest@" + d1
			status(k)["lastAttemptedRevision"] = "latest@" + d1
		}, "Synced", "Healthy", "Succeeded", 1, false, "release 1 applied; release 2 is published and not applied yet"},
		{"an unpublished digest applied", func(k map[string]any) {
			status(k)["lastAppliedRevision"] = "latest@" + dX
			status(k)["lastAttemptedRevision"] = "latest@" + dX
		}, "Synced", "Healthy", "Succeeded", 0, false, "no published release"},
		{"workloads nobody checked", func(k map[string]any) {
			spec(k)["wait"] = false
		}, "Synced", "Unknown", "Succeeded", 2, false, "Ready means applied, not healthy"},
		{"healthChecks covering every workload count as checked", func(k map[string]any) {
			spec(k)["wait"] = false
			spec(k)["healthChecks"] = []any{map[string]any{"kind": "Deployment", "name": "frontend", "namespace": "apptique-dev"}}
		}, "Synced", "Healthy", "Succeeded", 2, true, "release 2 applied"},
		{"healthChecks in the targetNamespace by default", func(k map[string]any) {
			spec(k)["wait"] = false
			spec(k)["targetNamespace"] = "apptique-dev"
			spec(k)["healthChecks"] = []any{map[string]any{"kind": "Deployment", "name": "frontend"}}
		}, "Synced", "Healthy", "Succeeded", 2, true, "release 2 applied"},
		{"healthChecks covering one workload of two", func(k map[string]any) {
			spec(k)["wait"] = false
			spec(k)["healthChecks"] = []any{map[string]any{"kind": "Deployment", "name": "frontend", "namespace": "apptique-dev"}}
			status(k)["inventory"] = map[string]any{"entries": []any{
				map[string]any{"id": "apptique-dev_frontend_apps_Deployment", "v": "v1"},
				map[string]any{"id": "apptique-dev_cart_apps_Deployment", "v": "v1"},
			}}
		}, "Synced", "Unknown", "Succeeded", 2, false, "Deployment apptique-dev/cart"},
		{"no workloads, nothing to check", func(k map[string]any) {
			spec(k)["wait"] = false
			status(k)["inventory"] = map[string]any{"entries": []any{map[string]any{"id": "_apptique-dev__Namespace", "v": "v1"}}}
		}, "Synced", "Healthy", "Succeeded", 2, true, "release 2 applied"},
		{"applying the newest release: the reading is about that one", func(k map[string]any) {
			status(k)["lastAppliedRevision"] = "latest@" + d1
		}, "OutOfSync", "Progressing", "Running", 2, false, "Flux is applying this release; it last applied " + d1},
		{"applying a digest that is no published release", func(k map[string]any) {
			status(k)["lastAttemptedRevision"] = "latest@" + dX
		}, "OutOfSync", "Progressing", "Running", 0, false, "no published release"},
		{"Ready False: the release Flux tried is the one that failed", func(k map[string]any) {
			status(k)["conditions"] = []any{map[string]any{"type": "Ready", "status": "False", "observedGeneration": 3, "message": "health check failed"}}
		}, "OutOfSync", "Degraded", "Failed", 2, false, "health check failed"},
		{"Ready False with no message still says so", func(k map[string]any) {
			status(k)["conditions"] = []any{map[string]any{"type": "Ready", "status": "False", "observedGeneration": 3}}
		}, "OutOfSync", "Degraded", "Failed", 2, false, "not ready"},
		{"stalled", func(k map[string]any) {
			status(k)["conditions"] = append(status(k)["conditions"].([]any), map[string]any{"type": "Stalled", "status": "True", "message": "path not found"})
		}, "OutOfSync", "Degraded", "Failed", 2, false, "stalled: path not found"},
		{"reconciling a release it has not applied yet", func(k map[string]any) {
			status(k)["lastAppliedRevision"] = "latest@" + d1
			status(k)["conditions"] = append(status(k)["conditions"].([]any), map[string]any{"type": "Reconciling", "status": "True", "message": "applying"})
		}, "OutOfSync", "Progressing", "Running", 2, false, "applying"},
		{"a generation Flux has not seen", func(k map[string]any) {
			k["metadata"].(map[string]any)["generation"] = 4
		}, "Unknown", "Progressing", "Running", 2, false, "current generation"},
		{"Ready from an old generation", func(k map[string]any) {
			status(k)["conditions"] = []any{map[string]any{"type": "Ready", "status": "True", "observedGeneration": 2}}
		}, "Unknown", "Progressing", "Running", 2, false, "current generation"},
		{"suspended", func(k map[string]any) {
			spec(k)["suspend"] = true
		}, "Unknown", "Suspended", "", 2, false, "suspended"},
		{"nothing applied or attempted yet", func(k map[string]any) {
			delete(status(k), "lastAppliedRevision")
			delete(status(k), "lastAttemptedRevision")
			status(k)["conditions"] = []any{map[string]any{"type": "Ready", "status": "False", "observedGeneration": 3, "message": "artifact not found"}}
		}, "OutOfSync", "Degraded", "Failed", 0, false, "artifact not found"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := readWith(t, ks(t, tc.mutate), spaceURL)
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
			if s.Reporter != StatusReporter || s.DataSource != "apps" || s.ObservedAt != "2026-09-30T12:00:00Z" || r.Newest != 2 {
				t.Errorf("reporter, source, time or newest release wrong: %+v", r)
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
	if _, err := ReadStatus(run, &fakeHub{releasesErr: fmt.Errorf("no such space")}, Check{Kustomization: "apps", Space: "flux-apps-dev"}, time.Now()); err == nil {
		t.Error("a failed release list must be an error, not a reading")
	}
}

func TestStatusMessageIsClipped(t *testing.T) {
	long := strings.Repeat("x", 2000)
	r := readWith(t, ks(t, func(k map[string]any) {
		status(k)["conditions"] = []any{map[string]any{"type": "Ready", "status": "False", "observedGeneration": 3, "message": long}}
	}), spaceURL)
	if len(r.Status.Message) > statusMessageLimit {
		t.Errorf("a message is clipped to %d bytes; this is %d", statusMessageLimit, len(r.Status.Message))
	}
}

var fluxCheck = Check{Kustomization: "apps", Space: "flux-apps-dev"}

// at is a reading of release 2, the newest, and what that Release holds.
func at(status LiveStatus, held *LiveStatus) Reading {
	return Reading{Check: fluxCheck, Status: status, Revision: d2, Release: 2, Newest: 2, Held: held}
}

func TestReportStatusWritesOnlyWhatChanged(t *testing.T) {
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	reading := LiveStatus{Reporter: StatusReporter, DataSource: "apps", Sync: "Synced", Health: "Healthy", Operation: "Succeeded", ObservedAt: now.Format(time.RFC3339)}
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
			if len(wrote) == 1 && !strings.HasPrefix(wrote[0], `flux-apps-dev release 2 {"reporter":"cub-flux"`) {
				t.Errorf("the status goes on release 2 of the Space, as ours: %s", wrote[0])
			}
			if tc.did == "left" && !strings.Contains(outs[0].Why, "argobot") {
				t.Errorf("say whose reading was left alone: %q", outs[0].Why)
			}
		})
	}
}

// A reading with no Release to record it on is written nowhere, and said.
func TestReportStatusRecordsNothingWithoutARelease(t *testing.T) {
	hub := &fakeHub{}
	r := Reading{Check: fluxCheck, Status: LiveStatus{Reporter: StatusReporter, Sync: "Unknown", Health: "Healthy"}, Newest: 2,
		Unrecorded: "Flux reports no revision"}
	outs, err := ReportStatus(hub, []Reading{r}, time.Minute, false, time.Now())
	if err != nil || len(hub.recorded) != 0 || outs[0].Did != "unrecorded" {
		t.Fatalf("want nothing recorded: %v %v %+v", err, hub.recorded, outs)
	}
	var b strings.Builder
	PrintOutcomes(&b, outs)
	if !strings.Contains(b.String(), "not recorded") || !strings.Contains(b.String(), "no revision") || !strings.Contains(b.String(), "gate reads release 2, the newest, which nothing has reported on") {
		t.Errorf("say that nothing was recorded, why, and what it means for the gate: %s", b.String())
	}
}

// After a handback to Git, the newest Release still holds the last reading,
// and the Healthy gate would pass on it. A reading this reporter wrote is
// replaced; another reporter's is left alone.
func TestLeftSpaceReplacesOurStaleReading(t *testing.T) {
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	ours := &LiveStatus{Reporter: StatusReporter, DataSource: "apps", Sync: "Synced", Health: "Healthy", Operation: "Succeeded", ObservedAt: now.Format(time.RFC3339)}
	theirs := &LiveStatus{Reporter: "argobot", DataSource: "apps", Sync: "Synced", Health: "Healthy", Operation: "Succeeded", ObservedAt: now.Add(-time.Hour).Format(time.RFC3339)}
	for _, tc := range []struct {
		name string
		held *LiveStatus
		did  string
	}{{"ours", ours, "written"}, {"another reporter's", theirs, "skipped"}, {"nothing held", nil, "skipped"}} {
		t.Run(tc.name, func(t *testing.T) {
			hub := &fakeHub{}
			gone := Reading{Check: fluxCheck, Skip: "reads https://github.com/acme/fleet, not Space argo-apps", Release: 2, Newest: 2, Held: tc.held}
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

// What the newest Release holds is read for a layer that has left its Space,
// so a reading of ours left behind there can be found.
func TestStatusOfALeftSpaceNamesTheNewestRelease(t *testing.T) {
	ours := &LiveStatus{Reporter: StatusReporter, Sync: "Synced", Health: "Healthy"}
	withStatus := []HubRelease{releases[0], {Num: 2, ManifestDigest: d2, Published: true, Live: ours}, releases[2]}
	onGit := ks(t, func(k map[string]any) {
		spec(k)["sourceRef"] = map[string]any{"kind": "GitRepository", "name": "fleet-repo"}
	})
	r, err := ReadStatus(fake(map[string]string{"get kustomization": onGit}), &fakeHub{releases: withStatus}, fluxCheck, time.Now())
	if err != nil || r.Skip == "" || r.Release != 2 || r.Held != ours {
		t.Errorf("want the skip, and release 2 with what it holds: %+v %v", r, err)
	}
}

// One Release that cannot be written must not stop the others being reported.
func TestReportStatusCarriesOnPastAnUnwritableSpace(t *testing.T) {
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	st := LiveStatus{Reporter: StatusReporter, Sync: "Synced", Health: "Healthy", Operation: "Succeeded", ObservedAt: now.Format(time.RFC3339)}
	hub := &fakeHub{unwritable: map[string]bool{"gone": true}}
	outs, err := ReportStatus(hub, []Reading{
		{Check: Check{Kustomization: "a", Space: "gone"}, Status: st, Release: 1, Newest: 1},
		{Check: Check{Kustomization: "b", Space: "here"}, Status: st, Release: 1, Newest: 1},
	}, time.Minute, false, now)
	if err == nil || !strings.Contains(err.Error(), "gone") {
		t.Errorf("the unwritable Space should be reported: %v", err)
	}
	if wrote := hub.recorded; len(wrote) != 1 || !strings.HasPrefix(wrote[0], "here ") || len(outs) != 1 {
		t.Errorf("the next Space should still be written: wrote %v, outcomes %v", wrote, outs)
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

// A reading is true of one Release. A passing reading this reporter left on
// the newest Release is withdrawn when Flux now reports another release,
// or none: the gate reads the newest, and would go on passing on it.
func TestWithdrawsAPassingReadingThatIsNoLongerTrue(t *testing.T) {
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	passing := func(reporter string) *LiveStatus {
		return &LiveStatus{Reporter: reporter, DataSource: "apps", Sync: "Synced", Health: "Healthy", Operation: "Succeeded", ObservedAt: now.Format(time.RFC3339)}
	}
	on := func(newest *LiveStatus) *fakeHub {
		return &fakeHub{releases: []HubRelease{releases[0], {Num: 2, ManifestDigest: d2, Published: true, Live: newest}, releases[2]}}
	}
	rolledBack := ks(t, func(k map[string]any) {
		status(k)["lastAppliedRevision"] = "latest@" + d1
		status(k)["lastAttemptedRevision"] = "latest@" + d1
	})
	notCompared := ks(t, func(k map[string]any) {
		delete(status(k), "lastAppliedRevision")
		delete(status(k), "lastAttemptedRevision")
		status(k)["conditions"] = []any{map[string]any{"type": "Ready", "status": "False", "observedGeneration": 3, "message": "artifact not found"}}
	})
	for _, tc := range []struct {
		name     string
		app      string
		newest   *LiveStatus
		withdrew string
		says     string
	}{
		{"rolled back to release 1", rolledBack, passing(StatusReporter), "written", `release 2 {"reporter":"cub-flux","dataSource":"apps","sync":"OutOfSync","health":"Unknown","message":"not what is running: Flux reports release 1"`},
		{"no reading at all", notCompared, passing(StatusReporter), "written", `release 2 {"reporter":"cub-flux","dataSource":"apps","sync":"Unknown","health":"Unknown","message":"not known to be running: artifact not found`},
		{"argobot's reading is not ours to withdraw", rolledBack, passing("argobot"), "", ""},
		{"another Application's reading is not this one's", rolledBack, func() *LiveStatus { p := passing(StatusReporter); p.DataSource = "other"; return p }(), "", ""},
		{"nothing on the newest release", rolledBack, nil, "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hub := on(tc.newest)
			r, err := ReadStatus(fake(map[string]string{"get kustomization": tc.app, "get ocirepository": spaceURL}), hub, fluxCheck, now)
			if err != nil {
				t.Fatal(err)
			}
			outs, err := ReportStatus(hub, []Reading{r}, 10*time.Minute, false, now)
			if err != nil {
				t.Fatal(err)
			}
			if outs[0].Withdrew != tc.withdrew {
				t.Fatalf("withdrew: want %q, got %q", tc.withdrew, outs[0].Withdrew)
			}
			wrote := strings.Join(hub.recorded, "\n")
			if tc.says != "" && !strings.Contains(wrote, "flux-apps-dev "+tc.says) {
				t.Errorf("want %s in:\n%s", tc.says, wrote)
			}
			if tc.withdrew == "" && strings.Contains("\n"+wrote, "\nflux-apps-dev release 2 ") {
				t.Errorf("release 2 must be left as it is:\n%s", wrote)
			}
			var b strings.Builder
			PrintOutcomes(&b, outs)
			if tc.newest != nil && tc.withdrew == "" && !strings.Contains(b.String(), "holds Synced/Healthy/Succeeded from "+tc.newest.Reporter) {
				t.Errorf("say what the newest release holds, since the gate reads it: %s", b.String())
			}
			if tc.withdrew != "" && !strings.Contains(b.String(), "withdrawn") {
				t.Errorf("say the reading was withdrawn: %s", b.String())
			}
		})
	}
}

// argobot writes only when something changes, so an old reading of its that
// says the same is not a stopped reporter, and is left alone.
func TestLeavesAnOldReadingOfAnotherReporterThatSaysTheSame(t *testing.T) {
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	reading := LiveStatus{Reporter: StatusReporter, DataSource: "apps", Sync: "Synced", Health: "Healthy", Operation: "Succeeded", ObservedAt: now.Format(time.RFC3339)}
	theirs := reading
	theirs.Reporter, theirs.Message, theirs.ObservedAt = "argobot", "its own words", now.Add(-24*time.Hour).Format(time.RFC3339)
	hub := &fakeHub{}
	outs, err := ReportStatus(hub, []Reading{at(reading, &theirs)}, 10*time.Minute, false, now)
	if err != nil || outs[0].Did != "left" || len(hub.recorded) != 0 || !strings.Contains(outs[0].Why, "argobot reported the same") {
		t.Errorf("want it left alone, and said why: %+v %v %v", outs, hub.recorded, err)
	}
}

// A message is cut on a character: half of one would read back as another,
// and the reading would be rewritten on every pass.
func TestClipCutsOnACharacter(t *testing.T) {
	got := clip(strings.Repeat("é", 150))
	if len(got) > statusMessageLimit || strings.ContainsRune(got, '\uFFFD') || !strings.HasSuffix(got, "é...") {
		t.Errorf("want whole characters within %d bytes: %d bytes, %q", statusMessageLimit, len(got), got[len(got)-8:])
	}
}

// Flux marks a layer Reconciling at the start of every pass, including the one
// it makes each interval over a release it applied long ago. That says nothing
// new, so nothing is written and what is recorded stands.
func TestStatusOfARoutineReconcileStands(t *testing.T) {
	for name, mutate := range map[string]func(k map[string]any){
		"Reconciling": func(k map[string]any) {
			status(k)["conditions"] = append(status(k)["conditions"].([]any), map[string]any{"type": "Reconciling", "status": "True", "message": "Reconciliation in progress"})
		},
		"readiness unknown": func(k map[string]any) {
			status(k)["conditions"] = []any{map[string]any{"type": "Ready", "status": "Unknown", "observedGeneration": 3, "message": "Reconciliation in progress"}}
		},
	} {
		t.Run(name, func(t *testing.T) {
			r := readWith(t, ks(t, mutate), spaceURL)
			if r.Pending == "" || r.Release != 0 {
				t.Fatalf("want nothing new to say: %+v", r)
			}
			hub := &fakeHub{}
			outs, err := ReportStatus(hub, []Reading{r}, time.Minute, false, time.Now())
			if err != nil || len(hub.recorded) != 0 || outs[0].Did != "pending" {
				t.Errorf("want nothing written: %+v %v %v", outs, hub.recorded, err)
			}
			var b strings.Builder
			PrintOutcomes(&b, outs)
			if !strings.Contains(b.String(), "what is recorded stands") {
				t.Errorf("say so: %s", b.String())
			}
		})
	}
}
