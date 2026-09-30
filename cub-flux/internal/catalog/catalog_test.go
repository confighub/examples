package catalog

import (
	"strings"
	"testing"
)

func TestSnapshotLoads(t *testing.T) {
	if n := Count(); n < 200 {
		t.Fatalf("the embedded snapshot holds %d verdicts; it should hold the Catalog's", n)
	}
}

// One chart version can have several audited bases with different verdicts.
// Reporting only one of them would be reporting the wrong one half the time.
func TestOneChartCanHaveTwoVerdicts(t *testing.T) {
	found := Find("redis", "0.34.11")
	if len(found) < 2 {
		t.Fatalf("redis 0.34.11 has more than one audited base; got %d", len(found))
	}
	lanes := map[string]string{}
	for _, e := range found {
		lanes[e.Base] = e.Lane
	}
	if lanes["default"] != "unsafe-to-flatten" {
		t.Errorf("redis default should be unsafe-to-flatten, got %q", lanes["default"])
	}
	if lanes["reuse-existing-secret"] != "safe-to-flatten" {
		t.Errorf("redis reuse-existing-secret should be safe-to-flatten, got %q", lanes["reuse-existing-secret"])
	}
}

// The verdict says which values changes take a variant out of its scope. That
// is what an overlay has to be checked against.
func TestVerdictNamesWhatMovesIt(t *testing.T) {
	lines := Describe("redis", "0.34.11")
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "unsafe-to-flatten") {
		t.Errorf("want the unsafe verdict reported:\n%s", joined)
	}
	if !strings.Contains(joined, "out of that verdict's scope") || !strings.Contains(joined, "auth.existingSecret") {
		t.Errorf("want the values that move the verdict named:\n%s", joined)
	}
}

// A chart the Catalog has not audited is unchecked, which is not a clean
// verdict and must not read as one.
func TestUnknownChartIsSaidToBeUnknown(t *testing.T) {
	lines := Describe("no-such-chart", "1.0.0")
	if len(lines) != 1 || !strings.Contains(lines[0], "not in the Workshop Catalog") {
		t.Errorf("want it said plainly that nothing is known, got %v", lines)
	}
	if strings.Contains(strings.Join(lines, " "), "safe") {
		t.Errorf("an unaudited chart must not be described as safe: %v", lines)
	}
}

// A verdict for one version does not carry to another.
func TestKnownChartAtUnknownVersion(t *testing.T) {
	v := Versions("redis")
	if len(v) == 0 {
		t.Skip("no redis in the snapshot")
	}
	lines := Describe("redis", "0.0.1-not-audited")
	joined := strings.Join(lines, " ")
	if !strings.Contains(joined, "does not carry") {
		t.Errorf("want it said that another version's verdict does not carry: %v", lines)
	}
	if !strings.Contains(joined, v[0]) {
		t.Errorf("want the audited versions listed, got %v", lines)
	}
}
