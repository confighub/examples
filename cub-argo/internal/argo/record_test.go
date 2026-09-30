package argo

import (
	"strings"
	"testing"
)

func recorder(t *testing.T, got *string) Runner {
	t.Helper()
	return func(name string, args ...string) ([]byte, error) {
		*got = strings.Join(args, " ")
		return []byte("Recorded pass LiveCheck attestation 0f0e0d0c-0b0a-4908-8706-050403020100 in x"), nil
	}
}

func checked(fields FieldCheck, inv InventoryComparison) Result {
	return Result{
		Check:     Check{Application: "apps", Space: "flux-apps-dev", Unit: "apps"},
		Release:   Release{Num: 2, ManifestDigest: "sha256:m2", UnitRevision: 5},
		Inventory: inv,
		Fields:    &fields,
	}
}

// A clean check records a Pass on the revision the release bundled, with the
// same type cub kubara uses.
func TestRecordCheckPass(t *testing.T) {
	var got string
	id, err := RecordCheck(recorder(t, &got), checked(FieldCheck{Total: 4, Compared: 4}, InventoryComparison{Same: 4}))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"attestation create --space flux-apps-dev", "--where Slug = 'apps'", "--revision 5", "--type LiveCheck",
		"--claim argocd.argoproj.io/application=apps", "--claim confighub.com/release=sha256:m2", "every field the release sets matches on all 4"} {
		if !strings.Contains(got, want) {
			t.Errorf("want %q in: %s", want, got)
		}
	}
	if strings.Contains(got, "--reject") || id != "0f0e0d0c-0b0a-4908-8706-050403020100" {
		t.Errorf("a clean check is a Pass, and its ID comes back: %s, %s", got, id)
	}
}

// A difference is recorded as a rejection that names it.
func TestRecordCheckRejects(t *testing.T) {
	var got string
	f := FieldCheck{Total: 1, Compared: 1, Diffs: []FieldDiff{{Object: "Deployment apptique-dev/frontend", Path: ".spec.replicas", Live: "4", Release: "1"}}}
	if _, err := RecordCheck(recorder(t, &got), checked(f, InventoryComparison{Same: 1})); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "--reject") || !strings.Contains(got, ".spec.replicas: cluster has 4, the release holds 1") {
		t.Errorf("want a rejection naming the field: %s", got)
	}
}

// Without a field comparison, or without a release, there is nothing to claim.
func TestRecordCheckNeedsFieldsAndARelease(t *testing.T) {
	var got string
	r := checked(FieldCheck{}, InventoryComparison{})
	r.Fields = nil
	if _, err := RecordCheck(recorder(t, &got), r); err == nil || got != "" {
		t.Errorf("no --fields, no record: %v %q", err, got)
	}
	r = checked(FieldCheck{Total: 1, Compared: 1}, InventoryComparison{Same: 1})
	r.Release = Release{}
	if _, err := RecordCheck(recorder(t, &got), r); err == nil || got != "" {
		t.Errorf("no release, no record: %v %q", err, got)
	}
}
