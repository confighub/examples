// secscan — sec-scanner's custom container image vulnerability scanner.
//
//	secscan scan <image>              scan one image against the cvedb
//	secscan inventory  --space <glob> list the fleet's images (from ConfigHub)
//	secscan scan-fleet --space <glob> scan every fleet image; --write-back
//	                                  records findings onto the Units as data
//
// All durable data lives in the cvedb (a SQLite file) and ConfigHub; this
// binary holds nothing. See README.md.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"secscan/internal/chub"
	"secscan/internal/cve"
	"secscan/internal/cvedb"
	"secscan/internal/image"
	"secscan/internal/match"
)

var sevRank = map[string]int{"UNKNOWN": 0, "LOW": 1, "MEDIUM": 2, "HIGH": 3, "CRITICAL": 4}

// Report is the scan result for one image.
type Report struct {
	Image       string          `json:"image"`
	Digest      string          `json:"digest,omitempty"`
	Ecosystem   string          `json:"ecosystem"`
	Packages    int             `json:"packages"`
	MaxSeverity string          `json:"max_severity"`
	Counts      map[string]int  `json:"counts"`
	Findings    []match.Finding `json:"findings"`
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	switch os.Args[1] {
	case "scan":
		cmdScan(os.Args[2:])
	case "inventory":
		cmdInventory(os.Args[2:])
	case "scan-fleet":
		cmdScanFleet(os.Args[2:])
	case "stale":
		cmdStale(os.Args[2:])
	case "import":
		cmdImport(os.Args[2:])
	case "gen-fixtures":
		cmdGenFixtures(os.Args[2:])
	case "version":
		fmt.Println("secscan (sec-scanner example)")
	default:
		usage()
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `secscan — sec-scanner image vulnerability scanner

usage:
  secscan scan <image> [--json]
  secscan inventory  --space <glob> [--where EXPR] [--json]
  secscan scan-fleet --space <glob> [--where EXPR] [--write-back] [--status-space SLUG] [--fail-on SEV] [--json]
  secscan stale      --space <glob> [--where EXPR] [--json]
  secscan import     [--osv-zip ECO|URL|FILE]... [--ghsa DIR] [--cvelist DIR] [--fixtures] [--limit N]
  secscan gen-fixtures [--out FILE] [image]...
`)
	os.Exit(2)
}

// scanImage exports an image, parses its packages, and matches them against the
// cvedb for the image's ecosystem.
func scanImage(ref string) (*Report, error) {
	img, err := image.Extract(ref)
	if err != nil {
		return nil, err
	}
	pkgs := img.Packages()
	rep := &Report{
		Image: ref, Digest: img.Digest, Ecosystem: img.Ecosystem,
		Packages: len(pkgs), MaxSeverity: "NONE",
		Counts: map[string]int{}, Findings: []match.Finding{},
	}
	if img.Ecosystem == "" {
		return rep, fmt.Errorf("could not determine base image OS (no /etc/os-release); apk/dpkg only")
	}
	db, err := match.Load(img.Ecosystem)
	if err != nil {
		return rep, err
	}
	findings := db.Scan(pkgs)
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Score != findings[j].Score {
			return findings[i].Score > findings[j].Score
		}
		return findings[i].Package < findings[j].Package
	})
	rep.Findings = findings
	for _, f := range findings {
		rep.Counts[f.Severity]++
		if sevRank[f.Severity] > sevRank[rep.MaxSeverity] {
			rep.MaxSeverity = f.Severity
		}
	}
	if len(findings) == 0 {
		rep.MaxSeverity = "NONE"
	}
	return rep, nil
}

func cmdScan(args []string) {
	fs := flag.NewFlagSet("scan", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "emit JSON")
	// Tolerate the image being given before or after flags.
	fs.Parse(flagsFirst(args))
	if fs.NArg() != 1 {
		usage()
	}
	rep, err := scanImage(fs.Arg(0))
	if err != nil && len(rep.Findings) == 0 {
		fmt.Fprintf(os.Stderr, "scan %s: %v\n", fs.Arg(0), err)
		os.Exit(1)
	}
	if *asJSON {
		printJSON(rep)
		return
	}
	printReport(rep)
}

func cmdInventory(args []string) {
	fs := flag.NewFlagSet("inventory", flag.ExitOnError)
	space := fs.String("space", "", "space glob, e.g. 'sec-demo-*'")
	where := fs.String("where", "", "ConfigHub where-filter expression")
	asJSON := fs.Bool("json", false, "emit JSON")
	fs.Parse(args)
	if *space == "" {
		usage()
	}
	wls, err := chub.Inventory(*space, *where)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *asJSON {
		printJSON(wls)
		return
	}
	fmt.Printf("%-22s %-22s %s\n", "SPACE", "UNIT", "IMAGE")
	for _, w := range wls {
		for _, img := range w.Images {
			fmt.Printf("%-22s %-22s %s\n", w.Space, w.Unit, img)
		}
	}
}

func cmdScanFleet(args []string) {
	fs := flag.NewFlagSet("scan-fleet", flag.ExitOnError)
	space := fs.String("space", "", "space glob, e.g. 'sec-demo-*'")
	where := fs.String("where", "", "ConfigHub where-filter expression")
	writeBack := fs.Bool("write-back", false, "record findings onto the Units as data")
	statusSpace := fs.String("status-space", "", "publish a cvedb-status Unit into this Space (with --write-back)")
	failOn := fs.String("fail-on", "", "exit non-zero if any image has >= this severity (e.g. CRITICAL)")
	asJSON := fs.Bool("json", false, "emit JSON")
	fs.Parse(args)
	if *space == "" {
		usage()
	}
	wls, err := chub.Inventory(*space, *where)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// The CVE DB version (latest import time) and this run's scan time stamp
	// every write-back, so units record which DB snapshot judged them.
	dbVersion, dbAdvisories, _ := cvedb.Status("")
	scannedAt := time.Now().UTC().Format(time.RFC3339)

	type fleetResult struct {
		Space  string  `json:"space"`
		Unit   string  `json:"unit"`
		Report *Report `json:"report"`
	}
	var results []fleetResult
	worst := 0
	cache := map[string]*Report{}

	// Findings are written back as one AppConfig/YAML "sec-scan-record" Unit per
	// Space (a multi-document YAML), so we accumulate per-Space reports here and
	// publish them after the scan loop.
	type spaceRecord struct {
		id      string
		reports []workloadReport
	}
	records := map[string]*spaceRecord{}
	var recordOrder []string

	for _, w := range wls {
		// A Unit's severity is the max across all images it references.
		unitMax := "NONE"
		var merged []match.Finding
		var primary *Report
		for _, ref := range w.Images {
			rep := cache[ref]
			if rep == nil {
				rep, err = scanImage(ref)
				if err != nil && (rep == nil || len(rep.Findings) == 0) {
					fmt.Fprintf(os.Stderr, "  warn: scan %s: %v\n", ref, err)
				}
				if rep != nil {
					cache[ref] = rep
				}
			}
			if rep == nil {
				continue
			}
			if primary == nil {
				primary = rep
			}
			merged = append(merged, rep.Findings...)
			if sevRank[rep.MaxSeverity] > sevRank[unitMax] {
				unitMax = rep.MaxSeverity
			}
		}
		if primary == nil {
			continue
		}
		counts := map[string]int{}
		for _, f := range merged {
			counts[f.Severity]++
		}
		ur := &Report{
			Image: strings.Join(w.Images, ","), Ecosystem: primary.Ecosystem,
			MaxSeverity: unitMax, Counts: counts, Findings: merged,
		}
		results = append(results, fleetResult{Space: w.Space, Unit: w.Unit, Report: ur})
		if sevRank[unitMax] > worst {
			worst = sevRank[unitMax]
		}

		if *writeBack {
			desc := fmt.Sprintf("Scan: %s, %d CVE(s) across %d image(s)", unitMax, len(merged), len(w.Images))
			// Small gate signal on the workload: the no-critical-cves Trigger
			// reads max-severity; the dashboard reads cve-count.
			annos := map[string]string{
				"sec-scanner.confighub.com/max-severity":  unitMax,
				"sec-scanner.confighub.com/cve-count":     fmt.Sprintf("%d", len(merged)),
				"sec-scanner.confighub.com/scanned-at":    scannedAt,
				"sec-scanner.confighub.com/cvedb-version": dbVersion,
			}
			if err := chub.SetAnnotations(w, annos, desc); err != nil {
				fmt.Fprintf(os.Stderr, "  warn: write-back %s/%s: %v\n", w.Space, w.Unit, err)
			}
			// Accumulate the full findings for this workload into its Space's
			// scan-record (published as one multi-doc YAML Unit after the loop).
			rec := records[w.SpaceID]
			if rec == nil {
				rec = &spaceRecord{id: w.SpaceID}
				records[w.SpaceID] = rec
				recordOrder = append(recordOrder, w.SpaceID)
			}
			rec.reports = append(rec.reports,
				buildReport(w.Unit, w.Images, primary.Ecosystem, unitMax, scannedAt, dbVersion, merged))
		}
	}

	// Publish each Space's findings as a single AppConfig/YAML "sec-scan-record"
	// Unit — a multi-document YAML (one stable, key-ordered doc per workload).
	// The data is small, so one record per Space beats one Unit per workload.
	if *writeBack {
		for _, sid := range recordOrder {
			rec := records[sid]
			sort.Slice(rec.reports, func(i, j int) bool { return rec.reports[i].Unit < rec.reports[j].Unit })
			recordYAML := marshalRecord(rec.reports)
			labels := map[string]string{"app": "sec-scanner", "role": "scan-record"}
			desc := fmt.Sprintf("Scan record: %d workload(s)", len(rec.reports))
			if err := chub.UpsertUnit(sid, "sec-scan-record", "AppConfig/YAML",
				"Scan record", labels, recordYAML, desc); err != nil {
				fmt.Fprintf(os.Stderr, "  warn: scan-record %s: %v\n", sid, err)
			}
		}
	}

	// Publish the current CVE DB version to ConfigHub so the console can flag
	// units scanned against an older snapshot.
	if *writeBack && *statusSpace != "" {
		if sid, err := chub.ResolveSpaceID(*statusSpace); err != nil {
			fmt.Fprintf(os.Stderr, "  warn: cvedb-status space %q: %v\n", *statusSpace, err)
		} else {
			statusYAML, _ := yaml.Marshal(cvedbStatus{
				CVEDBVersion: dbVersion,
				Advisories:   dbAdvisories,
				LastScanAt:   scannedAt,
			})
			labels := map[string]string{"app": "sec-scanner", "role": "cvedb-status"}
			if err := chub.UpsertUnit(sid, "cvedb-status", "AppConfig/YAML",
				"CVE database status", labels, statusYAML, "Update cvedb status"); err != nil {
				fmt.Fprintf(os.Stderr, "  warn: cvedb-status: %v\n", err)
			}
		}
	}

	if *asJSON {
		printJSON(results)
	} else {
		fmt.Printf("%-18s %-18s %-9s %s\n", "SPACE", "UNIT", "SEVERITY", "CVEs (C/H/M/L)")
		for _, r := range results {
			c := r.Report.Counts
			fmt.Printf("%-18s %-18s %-9s %d/%d/%d/%d\n", r.Space, r.Unit, r.Report.MaxSeverity,
				c["CRITICAL"], c["HIGH"], c["MEDIUM"], c["LOW"])
		}
		if *writeBack {
			fmt.Printf("\nWrote scan results back (DB version %s).\n", orNone(dbVersion))
		}
	}

	if *failOn != "" && worst >= sevRank[strings.ToUpper(*failOn)] && sevRank[strings.ToUpper(*failOn)] > 0 {
		os.Exit(3)
	}
}

// reportFinding / workloadReport / cvedbStatus are the YAML shapes the scanner
// writes back as AppConfig/YAML data. They are plain structs (not maps) so the
// emitted YAML has a fixed, stable key order — re-scanning produces a byte-stable
// document unless the findings themselves change. Keys are snake_case to match
// the scanner's JSON finding shape, so where_data paths (e.g.
// findings.0.fixed_version) and the UI line up.
type reportFinding struct {
	Advisory     string  `yaml:"advisory"`
	Severity     string  `yaml:"severity"`
	CVSSScore    float64 `yaml:"cvss_score,omitempty"`
	Package      string  `yaml:"package"`
	Version      string  `yaml:"version"`
	FixedVersion string  `yaml:"fixed_version,omitempty"`
}

// workloadReport is one document in a Space's sec-scan-record (multi-doc YAML).
// Unit comes first so the document is self-identifying.
type workloadReport struct {
	Unit         string          `yaml:"unit"`
	Images       []string        `yaml:"images"`
	Ecosystem    string          `yaml:"ecosystem"`
	MaxSeverity  string          `yaml:"max_severity"`
	CVECount     int             `yaml:"cve_count"`
	ScannedAt    string          `yaml:"scanned_at"`
	CVEDBVersion string          `yaml:"cvedb_version"`
	Findings     []reportFinding `yaml:"findings"`
}

// cvedbStatus is the AppConfig/YAML cvedb-status Unit (current CVE DB snapshot).
type cvedbStatus struct {
	CVEDBVersion string `yaml:"cvedb_version"`
	Advisories   int    `yaml:"advisories"`
	LastScanAt   string `yaml:"last_scan_at"`
}

// buildReport assembles the full (uncapped) scan report for one workload, with a
// total-order finding sort (severity, then score, then advisory, package,
// version) so the output is deterministic for a given set of findings.
func buildReport(unit string, images []string, ecosystem, maxSev, scannedAt, cvedbVersion string, findings []match.Finding) workloadReport {
	rf := make([]reportFinding, 0, len(findings))
	for _, f := range findings {
		rf = append(rf, reportFinding{
			Advisory: f.Advisory, Severity: f.Severity, CVSSScore: f.Score,
			Package: f.Package, Version: f.Version, FixedVersion: f.FixedVersion,
		})
	}
	sort.Slice(rf, func(i, j int) bool {
		a, b := rf[i], rf[j]
		if a.Severity != b.Severity {
			return sevRank[a.Severity] > sevRank[b.Severity]
		}
		if a.CVSSScore != b.CVSSScore {
			return a.CVSSScore > b.CVSSScore
		}
		if a.Advisory != b.Advisory {
			return a.Advisory < b.Advisory
		}
		if a.Package != b.Package {
			return a.Package < b.Package
		}
		return a.Version < b.Version
	})
	return workloadReport{
		Unit: unit, Images: images, Ecosystem: ecosystem, MaxSeverity: maxSev,
		CVECount: len(rf), ScannedAt: scannedAt, CVEDBVersion: cvedbVersion, Findings: rf,
	}
}

// marshalRecord renders one workload report per YAML document, separated by the
// standard `---` marker, in the order given (callers sort by unit slug).
func marshalRecord(reports []workloadReport) []byte {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	for _, r := range reports {
		if err := enc.Encode(r); err != nil { // each Encode emits a `---`-led document
			fmt.Fprintf(os.Stderr, "  warn: marshal scan-record doc %q: %v\n", r.Unit, err)
		}
	}
	enc.Close()
	return buf.Bytes()
}

func orNone(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}

// cmdStale lists in-scope units whose recorded cvedb-version is older than the
// current CVE DB (or that were never scanned) — i.e. what to re-scan.
func cmdStale(args []string) {
	fs := flag.NewFlagSet("stale", flag.ExitOnError)
	space := fs.String("space", "", "space glob, e.g. 'sec-demo-*'")
	where := fs.String("where", "", "ConfigHub where-filter expression")
	asJSON := fs.Bool("json", false, "emit JSON")
	fs.Parse(args)
	if *space == "" {
		usage()
	}
	current, _, err := cvedb.Status("")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	wls, err := chub.Inventory(*space, *where)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	type staleRow struct {
		Space     string `json:"space"`
		Unit      string `json:"unit"`
		ScannedAt string `json:"scanned_at"`
		ScannedDB string `json:"scanned_cvedb_version"`
		Reason    string `json:"reason"`
	}
	var rows []staleRow
	for _, w := range wls {
		reason := ""
		switch {
		case w.CVEDBVersion == "":
			reason = "never scanned"
		case current != "" && w.CVEDBVersion < current:
			reason = "scanned against older DB"
		}
		if reason != "" {
			rows = append(rows, staleRow{w.Space, w.Unit, w.ScannedAt, w.CVEDBVersion, reason})
		}
	}

	if *asJSON {
		printJSON(map[string]any{"cvedb_version": current, "stale": rows})
		return
	}
	fmt.Printf("CVE DB version (latest import): %s\n\n", orNone(current))
	if len(rows) == 0 {
		fmt.Println("All in-scope units are scanned against the current CVE DB.")
		return
	}
	fmt.Printf("%-18s %-20s %-22s %s\n", "SPACE", "UNIT", "SCANNED-AGAINST", "REASON")
	for _, r := range rows {
		fmt.Printf("%-18s %-20s %-22s %s\n", r.Space, r.Unit, orNone(r.ScannedDB), r.Reason)
	}
	fmt.Printf("\n%d unit(s) need re-scanning:\n  secscan scan-fleet --space %q --write-back\n", len(rows), *space)
}

func printReport(rep *Report) {
	fmt.Printf("image:      %s\n", rep.Image)
	if rep.Digest != "" {
		fmt.Printf("digest:     %s\n", rep.Digest)
	}
	fmt.Printf("ecosystem:  %s\n", rep.Ecosystem)
	fmt.Printf("packages:   %d\n", rep.Packages)
	fmt.Printf("severity:   %s  (C:%d H:%d M:%d L:%d)\n", rep.MaxSeverity,
		rep.Counts["CRITICAL"], rep.Counts["HIGH"], rep.Counts["MEDIUM"], rep.Counts["LOW"])
	if len(rep.Findings) == 0 {
		fmt.Println("\nNo known CVEs matched.")
		return
	}
	fmt.Printf("\n%-9s %-7s %-18s %-16s %-16s %s\n", "SEVERITY", "SCORE", "PACKAGE", "VERSION", "FIXED", "CVE")
	for _, f := range rep.Findings {
		fmt.Printf("%-9s %-7.1f %-18s %-16s %-16s %s\n",
			f.Severity, f.Score, trunc(f.Package, 18), trunc(f.Version, 16), trunc(f.FixedVersion, 16), f.Advisory)
	}
}

func trunc(s string, n int) string {
	if len(s) > n {
		return s[:n-1] + "…"
	}
	return s
}

// flagsFirst reorders argv so dash-prefixed tokens precede positionals, letting
// the stdlib flag package parse flags that appear after the image argument.
// Safe here because every flag in `scan` is boolean (takes no separate value).
func flagsFirst(args []string) []string {
	var flags, pos []string
	for _, a := range args {
		if strings.HasPrefix(a, "-") && a != "-" {
			flags = append(flags, a)
		} else {
			pos = append(pos, a)
		}
	}
	return append(flags, pos...)
}

func printJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(v)
}

// stringList is a repeatable string flag (e.g. --osv-zip A --osv-zip B).
type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error {
	*s = append(*s, v)
	return nil
}

// cmdImport loads CVE data from any of the three sources into the cvedb,
// normalizing and deduping in Go and writing through the pure-Go SQLite driver.
func cmdImport(args []string) {
	fs := flag.NewFlagSet("import", flag.ExitOnError)
	var osvZips stringList
	fs.Var(&osvZips, "osv-zip", "OSV ecosystem export: eco name (Alpine:v3.9), URL, or .zip; repeatable")
	ghsa := fs.String("ghsa", "", "path to a github/advisory-database clone")
	cvelist := fs.String("cvelist", "", "path to a CVEProject/cvelistV5 clone")
	fixtures := fs.Bool("fixtures", false, "load curated fixtures (see --fixtures-dir)")
	fixturesDir := fs.String("fixtures-dir", "cvedb/fixtures", "fixtures directory")
	limit := fs.Int("limit", 0, "cap records per source (0 = all)")
	ifEmpty := fs.Bool("if-empty", false, "skip (no download) if the cvedb already has advisories")
	fs.Parse(args)

	if len(osvZips) == 0 && *ghsa == "" && *cvelist == "" && !*fixtures {
		fmt.Fprintln(os.Stderr, "import: pick at least one source: --osv-zip / --ghsa / --cvelist / --fixtures")
		os.Exit(2)
	}

	db, err := cvedb.Open("")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer db.Close()

	if *ifEmpty {
		var n int
		if err := db.QueryRow("SELECT count(*) FROM advisory").Scan(&n); err == nil && n > 0 {
			fmt.Printf("cvedb already has %d advisories at %s, skipping import\n", n, cvedb.DefaultPath())
			return
		}
	}

	type job struct {
		source, label string
		recs          []cve.Record
	}
	var jobs []job
	add := func(source, label string, recs []cve.Record, err error) {
		if err != nil {
			fmt.Fprintf(os.Stderr, "  %s: %v\n", source, err)
			os.Exit(1)
		}
		jobs = append(jobs, job{source, label, recs})
	}
	if *fixtures {
		l, r, e := cve.ReadFixtures(*fixturesDir, *limit)
		add("fixtures", l, r, e)
	}
	for _, spec := range osvZips {
		l, r, e := cve.ReadOSVZip(spec, *limit)
		add("osv", l, r, e)
	}
	if *ghsa != "" {
		l, r, e := cve.ReadGHSA(*ghsa, *limit)
		add("ghsa", l, r, e)
	}
	if *cvelist != "" {
		l, r, e := cve.ReadCVEList(*cvelist, *limit)
		add("cvelist", l, r, e)
	}

	grand := 0
	for _, j := range jobs {
		byID := cve.Dedupe(j.recs)
		if err := cve.Load(db, byID, j.source, j.label); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("loaded %6d advisories from %s (%s)\n", len(byID), j.source, j.label)
		grand += len(byID)
	}
	fmt.Printf("done: %d advisories across %d source(s) -> %s\n", grand, len(jobs), cvedb.DefaultPath())
}

// cmdGenFixtures regenerates cvedb/fixtures/alpine-demo.json: the advisories the
// demo's vulnerable images match, in normalized OSV form, for an offline import.
func cmdGenFixtures(args []string) {
	fs := flag.NewFlagSet("gen-fixtures", flag.ExitOnError)
	out := fs.String("out", "cvedb/fixtures/alpine-demo.json", "output file")
	fs.Parse(args)
	images := fs.Args()
	if len(images) == 0 {
		images = []string{"nginx:1.16-alpine", "python:3.7-alpine3.10"}
	}

	ids := map[string]bool{}
	for _, ref := range images {
		rep, err := scanImage(ref)
		if err != nil && (rep == nil || len(rep.Findings) == 0) {
			fmt.Fprintf(os.Stderr, "  scan %s: %v\n", ref, err)
			continue
		}
		for _, f := range rep.Findings {
			ids[f.Advisory] = true
		}
	}
	idList := make([]string, 0, len(ids))
	for id := range ids {
		idList = append(idList, id)
	}
	sort.Strings(idList)
	fmt.Fprintf(os.Stderr, "%d matched advisories\n", len(idList))

	docs, err := cve.DumpFixtures(cvedb.DefaultPath(), idList)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	b, _ := json.MarshalIndent(docs, "", " ")
	if err := os.WriteFile(*out, append(b, '\n'), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "wrote %s with %d advisories\n", *out, len(docs))
}
