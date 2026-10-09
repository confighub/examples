package argo

import (
	"encoding/json"
	"fmt"
)

// fakeHub stands in for ConfigHub. Its zero value holds nothing: no release.
type fakeHub struct {
	release     *HubRelease // what Release answers; nil is "no release"
	releases    []HubRelease
	releasesErr error
	revision    int    // the revision the release's tag names; 0 is "not in the release"
	data        string // that revision's data
	head        int
	unwritable  map[string]bool // Spaces whose Releases cannot be written

	asked    []string
	recorded []string // "<space> release <n> <status as JSON>"
	attested []Attestation
}

func (f *fakeHub) Release(space, ref string) (HubRelease, error) {
	f.asked = append(f.asked, "release "+space+" "+ref)
	if f.release == nil {
		return HubRelease{}, fmt.Errorf("no release %s found in space %s", ref, space)
	}
	return *f.release, nil
}

func (f *fakeHub) Releases(space string) ([]HubRelease, error) {
	return f.releases, f.releasesErr
}

func (f *fakeHub) TaggedRevision(space, unit, tagID string) (int, bool, error) {
	f.asked = append(f.asked, "tagged "+space+" "+unit+" "+tagID)
	return f.revision, f.revision != 0, nil
}

func (f *fakeHub) RevisionData(space, unit string, revision int) ([]byte, error) {
	f.asked = append(f.asked, fmt.Sprintf("data %s %s %d", space, unit, revision))
	return []byte(f.data), nil
}

func (f *fakeHub) UnitHead(space, unit string) (int, error) {
	f.asked = append(f.asked, "head "+space+" "+unit)
	return f.head, nil
}

func (f *fakeHub) SetLiveStatus(space string, release int, st LiveStatus) error {
	if f.unwritable[space] {
		return fmt.Errorf("space %s not found", space)
	}
	doc, _ := json.Marshal(st)
	f.recorded = append(f.recorded, fmt.Sprintf("%s release %d %s", space, release, doc))
	return nil
}

func (f *fakeHub) Attest(a Attestation) (string, error) {
	f.attested = append(f.attested, a)
	return "0f0e0d0c-0b0a-4908-8706-050403020100", nil
}
