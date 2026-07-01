// costest — cost-estimator's workload cost estimator.
//
//	costest import --fixtures            load pricing into the cost database
//	costest estimate <file|->            cost one manifest against the cost DB
//	costest inventory   --space <glob>   list the fleet's workloads + their requests
//	costest estimate-fleet --space <glob> cost every workload; --write-back records
//	                                     the estimate + budget verdict onto the Units
//
// All durable data lives in the cost database (a SQLite file) and ConfigHub;
// this binary holds nothing. See README.md.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"

	"costest/internal/chub"
	"costest/internal/cost"
	"costest/internal/costdb"
	"costest/internal/k8s"
)

const annoPrefix = "cost-estimator.confighub.com/"

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	switch os.Args[1] {
	case "import":
		cmdImport(os.Args[2:])
	case "estimate":
		cmdEstimate(os.Args[2:])
	case "inventory":
		cmdInventory(os.Args[2:])
	case "estimate-fleet":
		cmdEstimateFleet(os.Args[2:])
	case "version":
		v, n, err := costdb.Status("")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("costest (cost-estimator example) · cost DB %s (%d prices)\n", orDash(v), n)
	default:
		usage()
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `costest — cost-estimator workload cost estimator

  costest import --fixtures [--fixtures-dir DIR] | --source FILE  [--if-empty]
                                     load pricing + budgets into the cost DB
  costest estimate <file|->          cost one Kubernetes manifest
  costest inventory   --space <glob> list fleet workloads + resource requests
  costest estimate-fleet --space <glob> [--write-back] [--status-space S]
                                     cost every workload; record estimates as data
  costest version

Flags: --db <path> (default costdb/cost.db, or $COST_ESTIMATOR_DB),
       --provider <p>, --region <r>, --env <e>, --where <expr>, --json, --fail-on-over

Environment: CONFIGHUB_URL, CONFIGHUB_TOKEN (cub auth get-token)
`)
	os.Exit(2)
}

func newFlagSet(name string) *flag.FlagSet { return flag.NewFlagSet(name, flag.ExitOnError) }

func orDash(s string) string {
	if s == "" {
		return "(empty)"
	}
	return s
}

func mustBook(db string) *costdb.Book {
	book, err := costdb.Load(db)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if len(book.Version) == 0 {
		fmt.Fprintln(os.Stderr, "cost DB is empty — run ./costdb/build.sh (or costest import --fixtures)")
		os.Exit(1)
	}
	return book
}

func printJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

// ─── import ───────────────────────────────────────────────────────────────────

func cmdImport(args []string) {
	fs := newFlagSet("import")
	db := fs.String("db", "", "cost DB path (default costdb/cost.db or $COST_ESTIMATOR_DB)")
	fixtures := fs.Bool("fixtures", false, "import the curated pricing fixtures")
	fixturesDir := fs.String("fixtures-dir", "costdb/fixtures", "fixtures directory")
	source := fs.String("source", "", "import a pricing JSON file (same shape as fixtures/pricing.json)")
	ifEmpty := fs.Bool("if-empty", false, "skip if the cost DB already has prices")
	_ = fs.Parse(args)

	if *ifEmpty {
		if _, n, err := costdb.Status(*db); err == nil && n > 0 {
			fmt.Printf("cost DB already populated (%d prices); skipping import.\n", n)
			return
		}
	}

	path := *source
	if path == "" {
		if !*fixtures {
			fmt.Fprintln(os.Stderr, "import: pass --fixtures or --source FILE")
			os.Exit(2)
		}
		path = *fixturesDir + "/pricing.json"
	}
	fx, err := loadFixtures(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	n, err := costdb.Import(*db, path, fx)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	v, _, _ := costdb.Status(*db)
	fmt.Printf("Imported %d prices + %d budgets from %s. Cost DB version %s.\n", n, len(fx.Budgets), path, orDash(v))
}

func loadFixtures(path string) (*costdb.Fixtures, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read fixtures %q: %w", path, err)
	}
	var fx costdb.Fixtures
	if err := json.Unmarshal(b, &fx); err != nil {
		return nil, fmt.Errorf("parse fixtures %q: %w", path, err)
	}
	return &fx, nil
}

// ─── estimate (one manifest) ──────────────────────────────────────────────────

func cmdEstimate(args []string) {
	fs := newFlagSet("estimate")
	db := fs.String("db", "", "cost DB path")
	provider := fs.String("provider", "", "cloud provider (defaults to the cost DB default)")
	region := fs.String("region", "", "region (defaults to the cost DB default)")
	env := fs.String("env", "", "environment for the budget verdict (dev/staging/prod)")
	_ = fs.Parse(args)
	if fs.NArg() < 1 {
		usage()
	}

	var data []byte
	var err error
	if path := fs.Arg(0); path == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	w, err := k8s.Parse(data)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	printJSON(cost.Estimate(w, *provider, *region, *env, mustBook(*db)))
}

// ─── inventory ────────────────────────────────────────────────────────────────

func cmdInventory(args []string) {
	fs := newFlagSet("inventory")
	space := fs.String("space", "", "space glob, e.g. 'cost-demo-*'")
	where := fs.String("where", "", "ConfigHub where-filter expression")
	db := fs.String("db", "", "cost DB path")
	provider := fs.String("provider", "", "override provider (else from the unit's Provider label)")
	asJSON := fs.Bool("json", false, "emit JSON")
	_ = fs.Parse(args)
	if *space == "" {
		usage()
	}
	book := mustBook(*db)
	reports := estimateFleet(*space, *where, *provider, book)
	if *asJSON {
		printJSON(reports)
		return
	}
	fmt.Printf("%-20s %-22s %-7s %5s %7s %7s %7s %12s  %s\n",
		"SPACE", "UNIT", "KIND", "CPU", "MEM(GB)", "STG(GB)", "PROV", "MONTHLY($)", "BUDGET")
	for _, r := range reports {
		fmt.Printf("%-20s %-22s %-7s %5.2f %7.2f %7.0f %7s %12.2f  %s\n",
			r.Space, r.Unit, r.Kind, r.CPUCores, r.MemoryGB, r.StorageGB, r.Provider, r.MonthlyUSD, r.BudgetStatus)
	}
}

// ─── estimate-fleet ───────────────────────────────────────────────────────────

func cmdEstimateFleet(args []string) {
	fs := newFlagSet("estimate-fleet")
	space := fs.String("space", "", "space glob, e.g. 'cost-demo-*'")
	where := fs.String("where", "", "ConfigHub where-filter expression")
	db := fs.String("db", "", "cost DB path")
	provider := fs.String("provider", "", "override provider (else from the unit's Provider label)")
	writeBack := fs.Bool("write-back", false, "record estimates onto the Units as data")
	statusSpace := fs.String("status-space", "", "publish a costdb-status Unit into this Space (with --write-back)")
	failOnOver := fs.Bool("fail-on-over", false, "exit non-zero if any workload is OVER budget")
	asJSON := fs.Bool("json", false, "emit JSON")
	_ = fs.Parse(args)
	if *space == "" {
		usage()
	}
	book := mustBook(*db)
	reports := estimateFleet(*space, *where, *provider, book)

	estimatedAt := time.Now().UTC().Format(time.RFC3339)
	type spaceRecord struct {
		id      string
		reports []cost.Report
	}
	records := map[string]*spaceRecord{}
	var recordOrder []string
	anyOver := false

	for _, r := range reports {
		if r.BudgetStatus == cost.StatusOver {
			anyOver = true
		}
		if !*writeBack {
			continue
		}
		desc := fmt.Sprintf("Cost estimate: $%.2f/mo (%s)", r.MonthlyUSD, r.BudgetStatus)
		annos := map[string]string{
			annoPrefix + "monthly-usd":     strconv.FormatFloat(r.MonthlyUSD, 'f', 2, 64),
			annoPrefix + "budget-status":   r.BudgetStatus,
			annoPrefix + "cpu-cores":       strconv.FormatFloat(r.CPUCores, 'f', 3, 64),
			annoPrefix + "memory-gb":       strconv.FormatFloat(r.MemoryGB, 'f', 3, 64),
			annoPrefix + "storage-gb":      strconv.FormatFloat(r.StorageGB, 'f', 3, 64),
			annoPrefix + "provider":        r.Provider,
			annoPrefix + "region":          r.Region,
			annoPrefix + "estimated-at":    estimatedAt,
			annoPrefix + "pricing-version": r.PricingVersion,
		}
		u := chub.Unit{Space: r.Space, SpaceID: r.SpaceID, Unit: r.Unit, UnitID: r.UnitID}
		if err := chub.SetAnnotations(u, annos, desc); err != nil {
			fmt.Fprintf(os.Stderr, "  warn: write-back %s/%s: %v\n", r.Space, r.Unit, err)
		}
		rec := records[r.SpaceID]
		if rec == nil {
			rec = &spaceRecord{id: r.SpaceID}
			records[r.SpaceID] = rec
			recordOrder = append(recordOrder, r.SpaceID)
		}
		rec.reports = append(rec.reports, r)
	}

	if *writeBack {
		for _, sid := range recordOrder {
			rec := records[sid]
			sort.Slice(rec.reports, func(i, j int) bool { return rec.reports[i].Unit < rec.reports[j].Unit })
			data := marshalRecord(rec.reports, estimatedAt, book.Version)
			labels := map[string]string{"app": "cost-estimator", "role": "cost-record"}
			if err := chub.UpsertUnit(sid, "cost-estimate-record", "AppConfig/YAML",
				"Cost estimate record", labels, data, "Publish cost estimate record"); err != nil {
				fmt.Fprintf(os.Stderr, "  warn: publish record for space %s: %v\n", sid, err)
			}
		}
		if *statusSpace != "" {
			if err := publishStatus(*statusSpace, book, *db, estimatedAt); err != nil {
				fmt.Fprintf(os.Stderr, "  warn: publish costdb-status: %v\n", err)
			}
		}
	}

	if *asJSON {
		printJSON(reports)
	} else {
		fmt.Printf("Estimated %d workload(s) at cost DB %s.\n", len(reports), orDash(book.Version))
		for _, r := range reports {
			fmt.Printf("  %-20s %-22s $%9.2f/mo  %s\n", r.Space, r.Unit, r.MonthlyUSD, r.BudgetStatus)
		}
	}
	if *failOnOver && anyOver {
		os.Exit(3)
	}
}

// estimateFleet lists the workloads in scope and costs each one, carrying the
// Unit's space/unit IDs for write-back.
func estimateFleet(space, where, providerOverride string, book *costdb.Book) []cost.Report {
	units, err := chub.Inventory(space, where)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var out []cost.Report
	for _, u := range units {
		w, err := k8s.Parse([]byte(u.YAML))
		if err != nil {
			continue // not a workload (e.g. a record / status Unit)
		}
		provider := providerOverride
		if provider == "" {
			provider = u.Labels["Provider"]
		}
		r := cost.Estimate(w, provider, u.Labels["Region"], u.Labels["Environment"], book)
		r.Unit, r.Space = u.Unit, u.Space
		r.SpaceID, r.UnitID = u.SpaceID, u.UnitID
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].MonthlyUSD > out[j].MonthlyUSD })
	return out
}

// marshalRecord renders a Space's estimates as a stable AppConfig/YAML document.
func marshalRecord(reports []cost.Report, estimatedAt, version string) []byte {
	doc := map[string]any{
		"estimatedAt":    estimatedAt,
		"pricingVersion": version,
		"workloads":      reports,
	}
	b, _ := yaml.Marshal(doc)
	return b
}

func publishStatus(statusSpace string, book *costdb.Book, db, at string) error {
	sid, err := chub.ResolveSpaceID(statusSpace)
	if err != nil {
		return err
	}
	_, prices, _ := costdb.Status(db)
	doc := map[string]any{
		"costdbVersion":   book.Version,
		"currency":        book.Currency,
		"defaultProvider": book.DefaultProvider,
		"defaultRegion":   book.DefaultRegion,
		"prices":          prices,
		"generatedAt":     at,
	}
	data, _ := yaml.Marshal(doc)
	return chub.UpsertUnit(sid, "costdb-status", "AppConfig/YAML",
		"Cost DB status", map[string]string{"app": "cost-estimator", "role": "costdb-status"},
		data, "Publish cost DB status")
}
