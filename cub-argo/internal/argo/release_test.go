package argo

import (
	"strings"
	"testing"
)

var release7 = HubRelease{Num: 7, ManifestDigest: "sha256:m7", TagID: "tag-7", Published: true}

// The check reads the unit as the release bundled it, which is what the
// handover delivers, even when the head has moved on (confighub/helm-expt#2021).
func TestCheckReadsTheReleaseNotTheHead(t *testing.T) {
	hub := &fakeHub{release: &release7, revision: 1, data: "kind: AtRevision1\n", head: 2}
	r, data, err := ReleasedData(hub, "s", "apps", "sha256:m7")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "kind: AtRevision1\n" || r.UnitRevision != 1 || r.Num != 7 || r.ManifestDigest != "sha256:m7" {
		t.Errorf("want revision 1 of release 7, got %+v %q", r, data)
	}
	asked := strings.Join(hub.asked, "; ")
	if !strings.Contains(asked, "tagged s apps tag-7") {
		t.Errorf("the revision should be found by the release's tag: %s", asked)
	}
	if !strings.Contains(asked, "data s apps 1") {
		t.Errorf("revision 1's data should be read, not the head's: %s", asked)
	}
	if hub.asked[0] != "release s sha256:m7" {
		t.Errorf("the named release should be read, not latest: %s", hub.asked[0])
	}
	if !strings.Contains(r.HeadAhead(), "head is revision 2, and release 7 holds revision 1") {
		t.Errorf("a head ahead of the release should be said: %q", r.HeadAhead())
	}
}

func TestReleaseWithoutTheUnitFails(t *testing.T) {
	if _, _, err := ReleasedData(&fakeHub{release: &release7}, "s", "apps", ""); err == nil || !strings.Contains(err.Error(), "does not hold apps") {
		t.Errorf("a release without the unit would deliver none of it: %v", err)
	}
}

func TestNoPublishedReleaseFails(t *testing.T) {
	if _, _, err := ReleasedData(&fakeHub{}, "s", "apps", ""); err == nil || !strings.Contains(err.Error(), "published release") {
		t.Errorf("no release means nothing to check: %v", err)
	}
}
