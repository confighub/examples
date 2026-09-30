// Package catalog answers what the Workshop Catalog has already decided about
// a chart: whether its rendered objects may stand in for it, and what would
// take that decision out of scope.
//
// The answers are a snapshot of helm-expt's flattening-safety verdicts, so the
// plugin answers offline. Refresh it with:
//
//	go run ./tools/snapshot -helm-expt ../../helm-expt
package catalog

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
)

//go:embed verdicts.json
var snapshot []byte

// Entry is one audited (chart, version, values base).
type Entry struct {
	Repository string `json:"repository"`
	Chart      string `json:"chart"`
	Version    string `json:"version"`
	Base       string `json:"base"`
	// Lane is the flattening verdict: safe-to-flatten, flatten-with-routes or
	// unsafe-to-flatten.
	Lane      string   `json:"lane"`
	Rationale string   `json:"rationale,omitempty"`
	SHA256    string   `json:"packageSHA256,omitempty"`
	Hazards   []string `json:"hazards,omitempty"`
	// MovedBy are the values changes the audit says take a variant out of this
	// verdict's scope. An overlay that makes one of these cannot inherit it.
	MovedBy []struct {
		Values string `json:"values"`
		Effect string `json:"effect"`
	} `json:"movedBy,omitempty"`
}

// Safe reports whether the rendered objects may stand in for the chart with no
// further work.
func (e Entry) Safe() bool { return e.Lane == "safe-to-flatten" }

var entries []Entry

func load() []Entry {
	if entries == nil {
		if err := json.Unmarshal(snapshot, &entries); err != nil {
			panic("the embedded verdict snapshot does not parse: " + err.Error())
		}
	}
	return entries
}

// Count is how many audited bases the snapshot holds.
func Count() int { return len(load()) }

// Find returns every audited base of one chart version. A chart the Catalog
// has not audited returns none, which is not the same as a clean verdict and
// must not be reported as one.
func Find(chart, version string) []Entry {
	var out []Entry
	for _, e := range load() {
		if strings.EqualFold(e.Chart, chart) && e.Version == version {
			out = append(out, e)
		}
	}
	return out
}

// Versions returns the versions of a chart the Catalog has audited, for
// telling someone their version is not one of them.
func Versions(chart string) []string {
	seen := map[string]bool{}
	var out []string
	for _, e := range load() {
		if strings.EqualFold(e.Chart, chart) && !seen[e.Version] {
			seen[e.Version] = true
			out = append(out, e.Version)
		}
	}
	return out
}

// Describe says what the Catalog knows about a chart at a version, in one line
// per audited base, or what it does not know.
func Describe(chart, version string) []string {
	found := Find(chart, version)
	if len(found) == 0 {
		if v := Versions(chart); len(v) > 0 {
			return []string{fmt.Sprintf(
				"chart %s %s is not in the Workshop Catalog; it has %s. A verdict for another version does not carry",
				chart, version, strings.Join(v, ", "))}
		}
		return []string{fmt.Sprintf(
			"chart %s is not in the Workshop Catalog, so whether its objects may stand in for it has not been decided", chart)}
	}
	var out []string
	for _, e := range found {
		line := fmt.Sprintf("chart %s %s, base %q: %s", e.Chart, e.Version, e.Base, e.Lane)
		if e.Rationale != "" && !e.Safe() {
			line += " — " + e.Rationale
		}
		out = append(out, line)
		for _, m := range e.MovedBy {
			out = append(out, fmt.Sprintf("  out of that verdict's scope: %s (%s)", m.Values, m.Effect))
		}
	}
	return out
}
