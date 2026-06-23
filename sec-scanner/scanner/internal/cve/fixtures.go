package cve

import (
	"sort"
	"strings"

	"secscan/internal/cvedb"
)

// Fixture document shape (a subset of OSV) emitted into cvedb/fixtures.
type fxSeverity struct {
	Type  string `json:"type"`
	Score string `json:"score"`
}
type fxPackage struct {
	Ecosystem string `json:"ecosystem"`
	Name      string `json:"name"`
}
type fxRange struct {
	Type   string              `json:"type"`
	Events []map[string]string `json:"events"`
}
type fxAffected struct {
	Package  fxPackage `json:"package"`
	Ranges   []fxRange `json:"ranges"`
	Versions []string  `json:"versions"`
}

// FixtureDoc marshals to one entry of an OSV fixtures file.
type FixtureDoc struct {
	ID               string            `json:"id"`
	Summary          *string           `json:"summary"`
	Severity         []fxSeverity      `json:"severity,omitempty"`
	DatabaseSpecific map[string]string `json:"database_specific"`
	Affected         []fxAffected      `json:"affected"`
}

// DumpFixtures reads the given advisories from the cvedb and returns them in
// fixtures (OSV) form, keeping only Alpine affected entries (the demo images).
func DumpFixtures(dbPath string, ids []string) ([]FixtureDoc, error) {
	db, err := cvedb.Open(dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	if len(ids) == 0 {
		return nil, nil
	}
	in := "(" + strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",") + ")"
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}

	docs := map[string]*FixtureDoc{}
	rows, err := db.Query("SELECT id, COALESCE(summary,''), severity, COALESCE(cvss_vector,'') "+
		"FROM advisory WHERE id IN "+in, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id, summary, sev, vector string
		if err := rows.Scan(&id, &summary, &sev, &vector); err != nil {
			rows.Close()
			return nil, err
		}
		d := &FixtureDoc{ID: id, DatabaseSpecific: map[string]string{"severity": sev}}
		if summary != "" {
			s := summary
			d.Summary = &s
		}
		if vector != "" {
			d.Severity = []fxSeverity{{Type: "CVSS_V3", Score: vector}}
		}
		docs[id] = d
	}
	rows.Close()

	// affected_id -> ranges, restricted to the advisories in question
	ranges := map[string][]fxRange{}
	rr, err := db.Query("SELECT affected_id, range_type, COALESCE(introduced,''), COALESCE(fixed,'') "+
		"FROM affected_range WHERE affected_id IN "+
		"(SELECT id FROM affected WHERE advisory_id IN "+in+")", args...)
	if err != nil {
		return nil, err
	}
	for rr.Next() {
		var aid, rtype, intro, fixed string
		if err := rr.Scan(&aid, &rtype, &intro, &fixed); err != nil {
			rr.Close()
			return nil, err
		}
		events := []map[string]string{{"introduced": orZero(intro)}}
		if fixed != "" {
			events = append(events, map[string]string{"fixed": fixed})
		}
		if rtype == "" {
			rtype = "ECOSYSTEM"
		}
		ranges[aid] = append(ranges[aid], fxRange{Type: rtype, Events: events})
	}
	rr.Close()

	ar, err := db.Query("SELECT id, advisory_id, ecosystem, package FROM affected "+
		"WHERE advisory_id IN "+in+" AND ecosystem LIKE 'Alpine:%'", args...)
	if err != nil {
		return nil, err
	}
	for ar.Next() {
		var aid, advid, eco, pkg string
		if err := ar.Scan(&aid, &advid, &eco, &pkg); err != nil {
			ar.Close()
			return nil, err
		}
		d := docs[advid]
		if d == nil {
			continue
		}
		d.Affected = append(d.Affected, fxAffected{
			Package:  fxPackage{Ecosystem: eco, Name: pkg},
			Ranges:   ranges[aid],
			Versions: []string{},
		})
	}
	ar.Close()

	out := make([]FixtureDoc, 0, len(docs))
	for _, d := range docs {
		if len(d.Affected) > 0 {
			out = append(out, *d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func orZero(s string) string {
	if s == "" {
		return "0"
	}
	return s
}
