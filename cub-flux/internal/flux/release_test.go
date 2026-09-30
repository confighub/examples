package flux

import (
	"strings"
	"testing"
)

const releaseJSON = `{"Release":{"ReleaseNum":7,"ManifestDigest":"sha256:m7","TagID":"tag-7"}}`

// The check reads the unit as the release bundled it, which is what the
// handover delivers, even when the head has moved on (confighub/helm-expt#2021).
func TestCheckReadsTheReleaseNotTheHead(t *testing.T) {
	var asked []string
	run := func(name string, args ...string) ([]byte, error) {
		key := strings.Join(args, " ")
		asked = append(asked, key)
		switch {
		case strings.HasPrefix(key, "release get"):
			return []byte(releaseJSON), nil
		case strings.HasPrefix(key, "revision list"):
			if !strings.Contains(key, "Tags ? 'tag-7'") {
				t.Errorf("the revision should be found by the release's tag: %s", key)
			}
			return []byte(`[{"Revision":{"RevisionNum":1}}]`), nil
		case strings.HasPrefix(key, "revision data"):
			return []byte("kind: AtRevision1\n"), nil
		case strings.HasPrefix(key, "unit get"):
			return []byte(`{"Unit":{"HeadRevisionNum":2}}`), nil
		}
		return nil, nil
	}
	r, data, err := ReleasedData(run, "s", "apps", "sha256:m7")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "kind: AtRevision1\n" || r.UnitRevision != 1 || r.Num != 7 || r.ManifestDigest != "sha256:m7" {
		t.Errorf("want revision 1 of release 7, got %+v %q", r, data)
	}
	if !strings.Contains(asked[len(asked)-2], "revision data --space s apps 1") {
		t.Errorf("revision 1's data should be read: %v", asked)
	}
	if !strings.Contains(asked[0], "--oci-reference sha256:m7") {
		t.Errorf("the named release should be read, not latest: %s", asked[0])
	}
	for _, a := range asked {
		if strings.HasPrefix(a, "unit data") {
			t.Errorf("the head's data must not be what is compared: %s", a)
		}
	}
	if !strings.Contains(r.HeadAhead(), "head is revision 2, and release 7 holds revision 1") {
		t.Errorf("a head ahead of the release should be said: %q", r.HeadAhead())
	}
}

func TestReleaseWithoutTheUnitFails(t *testing.T) {
	run := fake(map[string]string{"release get": releaseJSON, "revision list": `[]`})
	if _, _, err := ReleasedData(run, "s", "apps", ""); err == nil || !strings.Contains(err.Error(), "does not hold apps") {
		t.Errorf("a release without the unit would deliver none of it: %v", err)
	}
}

func TestNoPublishedReleaseFails(t *testing.T) {
	run := fake(map[string]string{})
	if _, _, err := ReleasedData(run, "s", "apps", ""); err == nil || !strings.Contains(err.Error(), "published release") {
		t.Errorf("no release means nothing to check: %v", err)
	}
}
