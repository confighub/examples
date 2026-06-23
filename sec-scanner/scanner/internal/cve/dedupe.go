package cve

import "strings"

// Merged is the deduplicated, cross-source-merged form of one advisory, keyed
// by canonical id.
type Merged struct {
	ID        string
	Source    string
	Sources   map[string]bool
	Summary   string
	Details   string
	Severity  string
	Score     *float64
	Vector    string
	Published string
	Modified  string
	Withdrawn string
	Aliases   map[string]bool
	Affected  map[string]*Affected // key: ecosystem + "\x00" + package
}

func canonicalID(r Record) string {
	if strings.HasPrefix(r.ID, "CVE-") {
		return r.ID
	}
	for _, a := range r.Aliases {
		if strings.HasPrefix(a, "CVE-") {
			return a
		}
	}
	return r.ID
}

func mergeInto(m *Merged, r Record) {
	m.Sources[r.Source] = true
	for _, a := range r.Aliases {
		m.Aliases[a] = true
	}
	m.Aliases[r.ID] = true
	if m.Summary == "" {
		m.Summary = r.Summary
	}
	if m.Details == "" {
		m.Details = r.Details
	}
	if m.Vector == "" {
		m.Vector = r.Vector
	}
	if m.Published == "" {
		m.Published = r.Published
	}
	if m.Modified == "" {
		m.Modified = r.Modified
	}
	if m.Withdrawn == "" {
		m.Withdrawn = r.Withdrawn
	}
	if m.Score == nil && r.Score != nil {
		m.Score = r.Score
	}
	if SevRank[r.Severity] > SevRank[m.Severity] {
		m.Severity = r.Severity
	}
	for _, a := range r.Affected {
		key := a.Ecosystem + "\x00" + a.Package
		if existing, ok := m.Affected[key]; ok {
			existing.Ranges = append(existing.Ranges, a.Ranges...)
			existing.Versions = append(existing.Versions, a.Versions...)
		} else {
			cp := a
			m.Affected[key] = &cp
		}
	}
}

// Dedupe merges records that share an alias, keyed by canonical id. Records are
// linked transitively: a later record naming an existing alias folds in.
func Dedupe(records []Record) map[string]*Merged {
	byID := map[string]*Merged{}
	aliasToCanon := map[string]string{}
	for _, rec := range records {
		cid := canonicalID(rec)
		if c, ok := aliasToCanon[cid]; ok {
			cid = c
		}
		m, ok := byID[cid]
		if !ok {
			m = &Merged{
				ID: cid, Source: rec.Source, Severity: "UNKNOWN",
				Sources: map[string]bool{}, Aliases: map[string]bool{},
				Affected: map[string]*Affected{},
			}
			byID[cid] = m
		}
		mergeInto(m, rec)
		for a := range m.Aliases {
			aliasToCanon[a] = cid
		}
		aliasToCanon[cid] = cid
	}
	return byID
}
