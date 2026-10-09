package argo

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
	rel2ID  = "66666666-6666-4666-8666-666666666666"
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
		asked = append(asked, r.Method+" "+r.URL.Path+" where="+q.Get("where")+" body="+string(body)+" select="+q.Get("select"))
		w.Header().Set("Content-Type", "application/json")
		p := r.URL.Path
		switch {
		case p == "/api/space":
			w.Write([]byte(`[{"Space":{"SpaceID":"` + spaceID + `","Slug":"s","Annotations":{"k":"v"}}}]`))
		case p == "/api/unit" || p == "/api/space/"+spaceID+"/unit":
			w.Write([]byte(`[{"Unit":{"UnitID":"` + unitID + `","SpaceID":"` + spaceID + `","Slug":"apps","HeadRevisionNum":6}}]`))
		case r.Method == http.MethodPatch && strings.HasSuffix(p, "/release/"+rel2ID):
			w.Write([]byte(`{"ReleaseID":"` + rel2ID + `"}`))
		case strings.HasSuffix(p, "/release"):
			all := []string{
				`{"Release":{"ReleaseNum":1,"ManifestDigest":"sha256:m1","Published":true,"TagID":"` + tagID + `"}}`,
				`{"Release":{"ReleaseID":"` + rel2ID + `","ReleaseNum":2,"ManifestDigest":"sha256:m2","Published":true,"TagID":"` + tagID + `",
				  "LiveStatus":{"Reporter":"argobot","DataSource":"app","Sync":"Synced","Health":"Healthy","Operation":"Succeeded","ReporterSync":"Synced","Message":"m","ObservedAt":"2026-10-09T08:00:00Z"}}}`,
				`{"Release":{"ReleaseNum":3,"ManifestDigest":"sha256:m3","Published":false}}`,
			}
			switch q.Get("where") {
			case "Published = true":
				all = all[:2]
			case "ManifestDigest = 'sha256:m1'":
				all = all[:1]
			case "ReleaseNum = 2":
				all = all[1:2]
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

// What a tool reported on a release comes back with the release, and a new
// status is a merge patch on that Release naming every field, the empty ones
// as null, so nothing an earlier reporter wrote stays beside it.
func TestSDKHubReadsAndRecordsLiveStatus(t *testing.T) {
	h, asked := stubHub(t)
	all, err := h.Releases("s")
	if err != nil || all[0].Live != nil || all[1].Live == nil {
		t.Fatalf("only release 2 has been reported on: %+v %v", all, err)
	}
	// The fields are named, so a release's bundle is not fetched with it.
	if !strings.HasSuffix(last(asked), "select=ReleaseID,ReleaseNum,SpaceID,OrganizationID,Published,ManifestDigest,TagID,LiveStatus") {
		t.Errorf("want the fields named: %s", last(asked))
	}
	if got, want := *all[1].Live, (LiveStatus{Reporter: "argobot", DataSource: "app", Sync: "Synced", Health: "Healthy", Operation: "Succeeded",
		ReporterSync: "Synced", Message: "m", ObservedAt: "2026-10-09T08:00:00Z"}); got != want {
		t.Errorf("want %+v, got %+v", want, got)
	}
	st := LiveStatus{Reporter: "cub-argo", DataSource: "apptique-dev", Sync: "OutOfSync", Health: "Progressing", ObservedAt: "2026-10-09T09:00:00Z"}
	if err := h.SetLiveStatus("s", 2, st); err != nil {
		t.Fatal(err)
	}
	sent := last(asked)
	if !strings.HasPrefix(sent, "PATCH /api/space/"+spaceID+"/release/"+rel2ID+" ") {
		t.Fatalf("want a PATCH of release 2: %s", sent)
	}
	for _, want := range []string{`"LiveStatus":{`, `"Reporter":"cub-argo"`, `"Sync":"OutOfSync"`, `"Health":"Progressing"`, `"ObservedAt":"2026-10-09T09:00:00Z"`,
		`"Operation":null`, `"ReporterSync":null`, `"Message":null`} {
		if !strings.Contains(sent, want) {
			t.Errorf("want %s in %s", want, sent)
		}
	}
	if err := h.SetLiveStatus("s", 9, st); err == nil {
		t.Error("a release that does not exist must be an error, not a silent nothing")
	}
	st.ObservedAt = ""
	if err := h.SetLiveStatus("s", 2, st); err == nil {
		t.Error("a status with no time is refused here rather than by the server")
	}
}
