package cve

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Load replaces the touched advisories and inserts their normalized rows in one
// transaction. Deleting each advisory first lets ON DELETE CASCADE clear stale
// children; the inserts then repopulate them.
func Load(db *sql.DB, byID map[string]*Merged, source, detail string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for id := range byID {
		if _, err := tx.Exec("DELETE FROM advisory WHERE id = ?", id); err != nil {
			return err
		}
	}

	insAdv, err := tx.Prepare("INSERT INTO advisory " +
		"(id,source,sources,summary,details,severity,cvss_score,cvss_vector,published,modified,withdrawn,raw) " +
		"VALUES (?,?,?,?,?,?,?,?,?,?,?,?)")
	if err != nil {
		return err
	}
	insAlias, _ := tx.Prepare("INSERT OR IGNORE INTO advisory_alias (advisory_id,alias) VALUES (?,?)")
	insAff, _ := tx.Prepare("INSERT INTO affected (id,advisory_id,ecosystem,package,purl) VALUES (?,?,?,?,?)")
	insRng, _ := tx.Prepare("INSERT INTO affected_range (affected_id,range_type,introduced,fixed,last_affected) VALUES (?,?,?,?,?)")
	insVer, _ := tx.Prepare("INSERT INTO affected_version (affected_id,version) VALUES (?,?)")

	for cid, m := range byID {
		raw, _ := json.Marshal(map[string]string{"id": cid})
		if _, err := insAdv.Exec(cid, m.Source, joinSet(m.Sources),
			nullStr(m.Summary), nullStr(m.Details), m.Severity, m.Score,
			nullStr(m.Vector), nullStr(m.Published), nullStr(m.Modified),
			nullStr(m.Withdrawn), string(raw)); err != nil {
			return fmt.Errorf("insert advisory %s: %w", cid, err)
		}
		for alias := range m.Aliases {
			if alias != "" && alias != cid {
				insAlias.Exec(cid, alias)
			}
		}
		for i, key := range sortedKeys(m.Affected) {
			a := m.Affected[key]
			aid := fmt.Sprintf("%s#%d", cid, i)
			insAff.Exec(aid, cid, a.Ecosystem, a.Package, nullStr(a.Purl))
			for _, r := range a.Ranges {
				rt := r.Type
				if rt == "" {
					rt = "ECOSYSTEM"
				}
				insRng.Exec(aid, rt, nullStr(r.Introduced), nullStr(r.Fixed), nullStr(r.LastAffected))
			}
			for _, v := range dedupeStrings(a.Versions) {
				insVer.Exec(aid, v)
			}
		}
	}

	if _, err := tx.Exec("INSERT INTO import_log (source, detail, advisories) VALUES (?,?,?)",
		source, detail, len(byID)); err != nil {
		return err
	}
	return tx.Commit()
}

func joinSet(s map[string]bool) string {
	keys := make([]string, 0, len(s))
	for k := range s {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}

func sortedKeys(m map[string]*Affected) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func dedupeStrings(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range in {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}

// nullStr maps "" to a SQL NULL so empty fields don't masquerade as data.
func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
