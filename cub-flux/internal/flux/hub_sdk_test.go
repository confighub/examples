package flux

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	spaceID = "11111111-1111-4111-8111-111111111111"
	unitID  = "22222222-2222-4222-8222-222222222222"
	tagID   = "33333333-3333-4333-8333-333333333333"
	revID   = "44444444-4444-4444-8444-444444444444"
)

// stubHub is ConfigHub's API as the SDK client meets it: one Space, one Unit,
// two published releases and one not, and a tag on revisions 2 and 5. It
// records each request.
func stubHub(t *testing.T) (*SDKHub, *[]string) {
	t.Helper()
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		q := r.URL.Query()
		asked = append(asked, r.Method+" "+r.URL.Path+" where="+q.Get("where")+" body="+string(body))
		w.Header().Set("Content-Type", "application/json")
		p := r.URL.Path
		switch {
		case p == "/api/space":
			w.Write([]byte(`[{"Space":{"SpaceID":"` + spaceID + `","Slug":"s","Annotations":{"k":"v"}}}]`))
		case p == "/api/unit" || p == "/api/space/"+spaceID+"/unit":
			w.Write([]byte(`[{"Unit":{"UnitID":"` + unitID + `","SpaceID":"` + spaceID + `","Slug":"apps","HeadRevisionNum":6}}]`))
		case strings.HasSuffix(p, "/target"):
			if strings.Contains(q.Get("where"), "'dev'") {
				w.Write([]byte(`[{"Target":{"TargetID":"55555555-5555-4555-8555-555555555555","SpaceID":"` + spaceID + `","Slug":"dev"}}]`))
			} else {
				w.Write([]byte(`[]`))
			}
		case strings.HasSuffix(p, "/release"):
			all := []string{
				`{"Release":{"ReleaseNum":1,"ManifestDigest":"sha256:m1","Published":true,"TagID":"` + tagID + `"}}`,
				`{"Release":{"ReleaseNum":2,"ManifestDigest":"sha256:m2","Published":true,"TagID":"` + tagID + `"}}`,
				`{"Release":{"ReleaseNum":3,"ManifestDigest":"sha256:m3","Published":false}}`,
			}
			switch q.Get("where") {
			case "Published = true":
				all = all[:2]
			case "ManifestDigest = 'sha256:m1'":
				all = all[:1]
			case "":
			default:
				all = nil
			}
			w.Write([]byte("[" + strings.Join(all, ",") + "]"))
		case strings.HasSuffix(p, "/revision") || strings.HasSuffix(p, "/extended_revision"):
			// Out of order, as the server may return them.
			w.Write([]byte(`[{"Revision":{"RevisionID":"` + revID + `","RevisionNum":2}},{"Revision":{"RevisionID":"` + revID + `","RevisionNum":5}}]`))
		case strings.HasSuffix(p, "/data"):
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Write([]byte("kind: AtRevision5\n"))
		case strings.HasSuffix(p, "/attestation"):
			w.Write([]byte(`{"Attestation":{"AttestationID":"0f0e0d0c-0b0a-4908-8706-050403020100"}}`))
		default:
			w.Write([]byte(`{}`))
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("CUB_SERVER", srv.URL)
	t.Setenv("CUB_TOKEN", "token")
	return NewHub("test"), &asked
}

func last(asked *[]string) string { return (*asked)[len(*asked)-1] }

// "latest" is the newest published release; a digest names one exactly; and
// anything else is refused before it reaches a filter.
func TestSDKHubFindsReleases(t *testing.T) {
	h, asked := stubHub(t)
	r, err := h.Release("s", "latest")
	if err != nil || r.Num != 2 || r.ManifestDigest != "sha256:m2" || r.TagID != tagID || !strings.Contains(last(asked), "where=Published = true") {
		t.Errorf("latest is the newest published release: %+v %v (%s)", r, err, last(asked))
	}
	r, err = h.Release("s", "sha256:m1")
	if err != nil || r.Num != 1 || !strings.Contains(last(asked), "where=ManifestDigest = 'sha256:m1'") {
		t.Errorf("a digest names one release: %+v %v (%s)", r, err, last(asked))
	}
	n := len(*asked)
	for _, ref := range []string{"v1", "sha256:x' OR 'a' = 'a"} {
		if _, err := h.Release("s", ref); err == nil {
			t.Errorf("%q is neither latest nor a digest and must be refused", ref)
		}
	}
	for _, a := range (*asked)[n:] {
		if strings.Contains(a, "/release") {
			t.Errorf("a refused reference must not be asked for: %s", a)
		}
	}
	all, err := h.Releases("s")
	if err != nil || len(all) != 3 || all[2].Published {
		t.Errorf("every release is listed, published or not: %+v %v", all, err)
	}
}

// The revision a release bundled is found by the release's tag, and is the
// newest one carrying it whatever order the server answers in.
func TestSDKHubReadsTheTaggedRevision(t *testing.T) {
	h, asked := stubHub(t)
	rev, found, err := h.TaggedRevision("s", "apps", tagID)
	if err != nil || !found || rev != 5 || !strings.Contains(last(asked), "where=Tags ? '"+tagID+"'") {
		t.Errorf("want revision 5 by tag: %d %v %v (%s)", rev, found, err, last(asked))
	}
	data, err := h.RevisionData("s", "apps", 5)
	if err != nil || string(data) != "kind: AtRevision5\n" || !strings.Contains(strings.Join(*asked, "\n"), "where=RevisionNum = 5") {
		t.Errorf("revision 5's data: %q %v", data, err)
	}
	if head, err := h.UnitHead("s", "apps"); err != nil || head != 6 {
		t.Errorf("the head is revision 6: %d %v", head, err)
	}
}

// A Pass leaves the result to the server, as cub does; a rejection is a Fail.
// The Unit is named by its slug and the revision by its number.
func TestSDKHubRecordsAnAttestation(t *testing.T) {
	h, asked := stubHub(t)
	a := Attestation{Space: "s", Unit: "apps", Revision: 5, Type: "LiveCheck", Claims: map[string]string{"k": "v"}, Note: "n"}
	id, err := h.Attest(a)
	if err != nil || id != "0f0e0d0c-0b0a-4908-8706-050403020100" {
		t.Fatalf("the attestation's ID comes back: %s %v", id, err)
	}
	sent := last(asked)
	for _, want := range []string{`"WhereUnit":"Slug = 'apps'"`, `"Revision":"5"`, `"Type":"LiveCheck"`, `"Claims":{"k":"v"}`, `"Note":"n"`} {
		if !strings.Contains(sent, want) {
			t.Errorf("want %s in %s", want, sent)
		}
	}
	if strings.Contains(sent, `"Result"`) {
		t.Errorf("a Pass names no result: %s", sent)
	}
	a.Reject = true
	if _, err := h.Attest(a); err != nil || !strings.Contains(last(asked), `"Result":"Fail"`) {
		t.Errorf("a rejection is a Fail: %v %s", err, last(asked))
	}
	a.Unit = "apps' OR 'a' = 'a"
	if _, err := h.Attest(a); err == nil {
		t.Error("a Unit name with a quote must be refused, not put in a filter")
	}
}

// Live status is a merge patch on the Space, and the annotations come back.
func TestSDKHubReadsAndPatchesASpace(t *testing.T) {
	h, asked := stubHub(t)
	ann, err := h.SpaceAnnotations("s")
	if err != nil || ann["k"] != "v" {
		t.Errorf("the Space's annotations: %v %v", ann, err)
	}
	if err := h.PatchSpace("s", []byte(`{"Annotations":{"a":"b"}}`)); err != nil {
		t.Fatal(err)
	}
	if sent := last(asked); !strings.HasPrefix(sent, "PATCH /api/space/"+spaceID+" ") || !strings.Contains(sent, `body={"Annotations":{"a":"b"}}`) {
		t.Errorf("want a PATCH of the Space with the body as given: %s", sent)
	}
}

// A watch asks which Units a layers Space holds and whether a cluster has its
// Target yet; a Target that is not there is an answer, not an error.
func TestSDKHubListsUnitsAndFindsTargets(t *testing.T) {
	h, asked := stubHub(t)
	units, err := h.UnitSlugs("s")
	if err != nil || len(units) != 1 || units[0] != "apps" || !strings.Contains(last(asked), spaceID) {
		t.Errorf("the Space's Units, by its ID: %v %v (%s)", units, err, last(asked))
	}
	if ok, err := h.HasTarget("s", "dev"); err != nil || !ok {
		t.Errorf("dev has a Target: %v %v", ok, err)
	}
	if ok, err := h.HasTarget("s", "prod"); err != nil || ok {
		t.Errorf("a missing Target is false, not an error: %v %v", ok, err)
	}
}
