package flux

import (
	"encoding/json"
	"fmt"
)

// fakeHub stands in for ConfigHub. Its zero value holds nothing: no release,
// no Units, no Targets.
type fakeHub struct {
	release     *HubRelease     // what Release answers for any Space; nil is "no release"
	released    map[string]bool // or: the Spaces that have a release
	releases    []HubRelease
	releasesErr error
	revision    int            // the revision the release's tag names; 0 is "not in the release"
	revisions   map[string]int // or: per Unit
	data        string         // that revision's data
	head        int
	units       []string
	targets     map[string]bool
	unwritable  map[string]bool // Spaces whose Releases cannot be written

	asked    []string
	recorded []string // "<space> release <n> <status as JSON>"
	patches  []string
	attested []Attestation
}

func (f *fakeHub) Release(space, ref string) (HubRelease, error) {
	f.asked = append(f.asked, "release "+space+" "+ref)
	if f.released[space] {
		return HubRelease{Num: 1, ManifestDigest: "sha256:x", TagID: "t", Published: true}, nil
	}
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
	if f.revisions != nil {
		return f.revisions[unit], f.revisions[unit] != 0, nil
	}
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

func (f *fakeHub) PatchSpace(space string, patch []byte) error {
	f.patches = append(f.patches, space+" "+string(patch))
	return nil
}

func (f *fakeHub) Attest(a Attestation) (string, error) {
	f.attested = append(f.attested, a)
	return "0f0e0d0c-0b0a-4908-8706-050403020100", nil
}

func (f *fakeHub) UnitSlugs(space string) ([]string, error) { return f.units, nil }

func (f *fakeHub) HasTarget(space, target string) (bool, error) { return f.targets[target], nil }
