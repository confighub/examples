// Package cve ports the CVE import pipeline: it normalizes three upstream
// shapes (OSV exports, GitHub Advisory Database, CVE List V5) into one record,
// dedupes them by shared alias, and loads them into the SQLite cvedb.
package cve

import (
	"encoding/json"
	"math"
	"strings"
)

// SevRank orders severities; used for picking the worst and bucketing.
var SevRank = map[string]int{"UNKNOWN": 0, "LOW": 1, "MEDIUM": 2, "HIGH": 3, "CRITICAL": 4}

// Range is one affected version window. Empty Fixed = unbounded above.
type Range struct {
	Type, Introduced, Fixed, LastAffected string
}

// Affected is one affected package within an advisory.
type Affected struct {
	Ecosystem, Package, Purl string
	Ranges                   []Range
	Versions                 []string
}

// Record is a normalized advisory from any source.
type Record struct {
	ID        string
	Aliases   []string
	Summary   string
	Details   string
	Severity  string
	Score     *float64
	Vector    string
	Published string
	Modified  string
	Withdrawn string
	Source    string
	Affected  []Affected
}

// ── CVSS v3.0/v3.1 base score from a vector string ───────────────────────────

var (
	cvssAV  = map[string]float64{"N": 0.85, "A": 0.62, "L": 0.55, "P": 0.2}
	cvssAC  = map[string]float64{"L": 0.77, "H": 0.44}
	cvssUI  = map[string]float64{"N": 0.85, "R": 0.62}
	cvssPRU = map[string]float64{"N": 0.85, "L": 0.62, "H": 0.27} // scope unchanged
	cvssPRC = map[string]float64{"N": 0.85, "L": 0.68, "H": 0.5}  // scope changed
	cvssCIA = map[string]float64{"H": 0.56, "L": 0.22, "N": 0.0}
)

func roundup(x float64) float64 {
	i := int(math.Round(x * 100000))
	if i%10000 == 0 {
		return float64(i) / 100000.0
	}
	return (math.Floor(float64(i)/10000.0) + 1) / 10.0
}

// computeCVSS3 derives the base score from a CVSS:3.x vector; ok=false if it
// can't be parsed. Implements the published v3.1 formula (bucket boundaries are
// unaffected by the small v3.0 rounding difference).
func computeCVSS3(vector string) (float64, bool) {
	if vector == "" || !strings.Contains(vector, "CVSS:3") {
		return 0, false
	}
	parts := map[string]string{}
	for _, tok := range strings.Split(vector, "/") {
		if k, v, ok := strings.Cut(tok, ":"); ok {
			parts[k] = v
		}
	}
	scopeChanged := parts["S"] == "C"
	prTbl := cvssPRU
	if scopeChanged {
		prTbl = cvssPRC
	}
	av, ok1 := cvssAV[parts["AV"]]
	ac, ok2 := cvssAC[parts["AC"]]
	pr, ok3 := prTbl[parts["PR"]]
	ui, ok4 := cvssUI[parts["UI"]]
	c, ok5 := cvssCIA[parts["C"]]
	i, ok6 := cvssCIA[parts["I"]]
	a, ok7 := cvssCIA[parts["A"]]
	if !(ok1 && ok2 && ok3 && ok4 && ok5 && ok6 && ok7) {
		return 0, false
	}
	iscBase := 1 - (1-c)*(1-i)*(1-a)
	var impact float64
	if scopeChanged {
		impact = 7.52*(iscBase-0.029) - 3.25*math.Pow(iscBase-0.02, 15)
	} else {
		impact = 6.42 * iscBase
	}
	if impact <= 0 {
		return 0, true
	}
	expl := 8.22 * av * ac * pr * ui
	raw := impact + expl
	if scopeChanged {
		raw *= 1.08
	}
	return roundup(math.Min(raw, 10.0)), true
}

func severityFromScore(score float64) string {
	switch {
	case score >= 9.0:
		return "CRITICAL"
	case score >= 7.0:
		return "HIGH"
	case score >= 4.0:
		return "MEDIUM"
	case score > 0:
		return "LOW"
	default:
		return "UNKNOWN"
	}
}

// ── OSV shape (used by --osv-zip, --ghsa, --fixtures) ─────────────────────────

type osvSeverity struct {
	Type  string `json:"type"`
	Score string `json:"score"`
}

type osvAffected struct {
	Package struct {
		Ecosystem string `json:"ecosystem"`
		Name      string `json:"name"`
		Purl      string `json:"purl"`
	} `json:"package"`
	Ranges []struct {
		Type   string              `json:"type"`
		Events []map[string]string `json:"events"`
	} `json:"ranges"`
	Versions []string      `json:"versions"`
	Severity []osvSeverity `json:"severity"`
}

type osvDoc struct {
	ID               string        `json:"id"`
	Aliases          []string      `json:"aliases"`
	Summary          string        `json:"summary"`
	Details          string        `json:"details"`
	Published        string        `json:"published"`
	Modified         string        `json:"modified"`
	Withdrawn        string        `json:"withdrawn"`
	Severity         []osvSeverity `json:"severity"`
	DatabaseSpecific struct {
		Severity string `json:"severity"`
	} `json:"database_specific"`
	Affected []osvAffected `json:"affected"`
}

func osvToNorm(data []byte, source string) (Record, bool) {
	var d osvDoc
	if err := json.Unmarshal(data, &d); err != nil || d.ID == "" {
		return Record{}, false
	}
	rec := Record{
		ID: d.ID, Aliases: d.Aliases, Summary: d.Summary, Details: d.Details,
		Severity: "UNKNOWN", Published: d.Published, Modified: d.Modified,
		Withdrawn: d.Withdrawn, Source: source,
	}
	consider := func(vec string) {
		if s, ok := computeCVSS3(vec); ok && (rec.Score == nil || s > *rec.Score) {
			sc := s
			rec.Score = &sc
			rec.Vector = vec
		}
	}
	for _, s := range d.Severity {
		if strings.HasPrefix(s.Type, "CVSS") && s.Score != "" {
			consider(s.Score)
		}
	}
	if ds := strings.ToUpper(d.DatabaseSpecific.Severity); ds != "" {
		if _, ok := SevRank[ds]; ok {
			rec.Severity = ds
		}
	}
	for _, a := range d.Affected {
		if a.Package.Ecosystem == "" || a.Package.Name == "" {
			continue
		}
		af := Affected{Ecosystem: a.Package.Ecosystem, Package: a.Package.Name, Purl: a.Package.Purl}
		for _, r := range a.Ranges {
			rng := Range{Type: r.Type}
			if rng.Type == "" {
				rng.Type = "ECOSYSTEM"
			}
			for _, ev := range r.Events {
				if v, ok := ev["introduced"]; ok {
					rng.Introduced = v
				} else if v, ok := ev["fixed"]; ok {
					rng.Fixed = v
				} else if v, ok := ev["last_affected"]; ok {
					rng.LastAffected = v
				}
			}
			af.Ranges = append(af.Ranges, rng)
		}
		af.Versions = append(af.Versions, a.Versions...)
		rec.Affected = append(rec.Affected, af)
		for _, s := range a.Severity {
			if strings.HasPrefix(s.Type, "CVSS") && s.Score != "" {
				consider(s.Score)
			}
		}
	}
	finalizeSeverity(&rec)
	return rec, true
}

// ── CVE List V5 shape ─────────────────────────────────────────────────────────

type cve5Doc struct {
	CVEMetadata struct {
		CVEID         string `json:"cveId"`
		DatePublished string `json:"datePublished"`
		DateUpdated   string `json:"dateUpdated"`
	} `json:"cveMetadata"`
	Containers struct {
		CNA struct {
			Descriptions []struct {
				Lang  string `json:"lang"`
				Value string `json:"value"`
			} `json:"descriptions"`
			Metrics []map[string]struct {
				BaseScore    float64 `json:"baseScore"`
				VectorString string  `json:"vectorString"`
			} `json:"metrics"`
			Affected []struct {
				PackageName   string `json:"packageName"`
				Product       string `json:"product"`
				Vendor        string `json:"vendor"`
				CollectionURL string `json:"collectionURL"`
				Versions      []struct {
					Version         string `json:"version"`
					LessThan        string `json:"lessThan"`
					LessThanOrEqual string `json:"lessThanOrEqual"`
					Status          string `json:"status"`
				} `json:"versions"`
			} `json:"affected"`
		} `json:"cna"`
	} `json:"containers"`
}

func cve5ToNorm(data []byte) (Record, bool) {
	var d cve5Doc
	if err := json.Unmarshal(data, &d); err != nil || d.CVEMetadata.CVEID == "" {
		return Record{}, false
	}
	cna := d.Containers.CNA
	rec := Record{
		ID: d.CVEMetadata.CVEID, Severity: "UNKNOWN", Source: "cvelist",
		Published: d.CVEMetadata.DatePublished, Modified: d.CVEMetadata.DateUpdated,
	}
	for _, desc := range cna.Descriptions {
		if strings.HasPrefix(desc.Lang, "en") {
			rec.Summary = desc.Value
			break
		}
	}
	for _, m := range cna.Metrics {
		for _, key := range []string{"cvssV3_1", "cvssV3_0", "cvssV4_0"} {
			if c, ok := m[key]; ok {
				if c.BaseScore > 0 {
					sc := c.BaseScore
					rec.Score = &sc
				}
				if c.VectorString != "" {
					rec.Vector = c.VectorString
				}
			}
		}
	}
	for _, a := range cna.Affected {
		name := a.PackageName
		if name == "" {
			name = a.Product
		}
		if name == "" || name == "n/a" {
			continue
		}
		eco := a.CollectionURL
		if eco == "" {
			eco = a.Vendor
		}
		if eco == "" {
			eco = "CVE"
		}
		af := Affected{Ecosystem: eco, Package: name}
		for _, v := range a.Versions {
			lt := v.LessThan
			if lt == "" {
				lt = v.LessThanOrEqual
			}
			if lt != "" {
				intro := v.Version
				if intro == "" || intro == "0" {
					intro = "0"
				}
				af.Ranges = append(af.Ranges, Range{Type: "ECOSYSTEM", Introduced: intro, Fixed: lt})
			} else if v.Version != "" && v.Status == "affected" {
				af.Versions = append(af.Versions, v.Version)
			}
		}
		rec.Affected = append(rec.Affected, af)
	}
	finalizeSeverity(&rec)
	return rec, true
}

// finalizeSeverity fills severity from the computed score when no word-form
// severity was provided.
func finalizeSeverity(rec *Record) {
	if rec.Severity == "UNKNOWN" && rec.Score != nil {
		rec.Severity = severityFromScore(*rec.Score)
	}
}
