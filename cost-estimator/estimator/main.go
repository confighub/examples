// costest — cost-estimator's workload cost estimator.
//
//	costest estimate <file|->          cost one manifest against the price book
//	costest inventory   --space <glob> list the fleet's workloads + their requests
//	costest estimate-fleet --space <glob> cost every workload; --write-back records
//	                                  the estimate + budget verdict onto the Units
//
// All durable data lives in the price book (a JSON file) and ConfigHub; this
// binary holds nothing. See README.md.
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
	"costest/internal/k8s"
	"costest/internal/pricing"
)

const annoPrefix = "cost-estimator.confighub.com/"

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	switch os.Args[1] {
	case "estimate":
		cmdEstimate(os.Args[2:])
	case "inventory":
		cmdInventory(os.Args[2:])
	case "estimate-fleet":
		cmdEstimateFleet(os.Args[2:])
	case "version":
		fmt.Println("costest (cost-estimator example)")
	default:
		usage()
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `costest — cost-estimator workload cost estimator

  costest estimate <file|->          cost one Kubernetes manifest
  costest inventory   --space <glob> list fleet workloads + resource requests
  costest estimate-fleet --space <glob> [--write-back] [--status-space S]
                                     cost every workload; record estimates as data
  costest version

Flags: --pricebook <path> (default pricing/pricebook.json), --budgets <path>,
       --region <r>, --env <e>, --where <expr>, --json, --fail-on-over

Environment: CONFIGHUB_URL, CONFIGHUB_TOKEN (cub auth get-token)
`)
	os.Exit(2)
}

func newFlagSet(name string) *flag.FlagSet { return flag.NewFlagSet(name, flag.ExitOnError) }

func mustPricebook(path string) *pricing.PriceBook {
	pb, err := pricing.Load(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return pb
}

// budgetsFrom returns the override budgets file if given, else the price book's.
func budgetsFrom(pb *pricing.PriceBook, path string) map[string]float64 {
	if path == "" {
		return pb.BudgetsMonthlyUSD
	}
	b, err := pricing.LoadBudgets(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return b
}

func printJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func cmdEstimate(args []string) {
	fs := newFlagSet("estimate")
	pricebook := fs.String("pricebook", "pricing/pricebook.json", "price book path")
	budgets := fs.String("budgets", "", "budgets override file")
	region := fs.String("region", "", "region (defaults to the price book's default)")
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
	pb := mustPricebook(*pricebook)
	r := cost.Estimate(w, *region, *env, pb, budgetsFrom(pb, *budgets))
	printJSON(r)
}

func cmdInventory(args []string) {
	fs := newFlagSet("inventory")
	space := fs.String("space", "", "space glob, e.g. 'cost-demo-*'")
	where := fs.String("where", "", "ConfigHub where-filter expression")
	pricebook := fs.String("pricebook", "pricing/pricebook.json", "price book path")
	budgets := fs.String("budgets", "", "budgets override file")
	asJSON := fs.Bool("json", false, "emit JSON")
	_ = fs.Parse(args)
	if *space == "" {
		usage()
	}
	pb := mustPricebook(*pricebook)
	bm := budgetsFrom(pb, *budgets)

	units, err := chub.Inventory(*space, *where)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var reports []cost.Report
	for _, u := range units {
		w, err := k8s.Parse([]byte(u.YAML))
		if err != nil {
			continue // not a workload (e.g. a record Unit)
		}
		r := cost.Estimate(w, u.Labels["Region"], u.Labels["Environment"], pb, bm)
		r.Unit, r.Space = u.Unit, u.Space
		reports = append(reports, r)
	}
	if *asJSON {
		printJSON(reports)
		return
	}
	fmt.Printf("%-20s %-22s %-7s %7s %7s %7s %12s  %s\n",
		"SPACE", "UNIT", "KIND", "CPU", "MEM(GB)", "STG(GB)", "MONTHLY($)", "BUDGET")
	for _, r := range reports {
		fmt.Printf("%-20s %-22s %-7s %7.2f %7.2f %7.0f %12.2f  %s\n",
			r.Space, r.Unit, r.Kind, r.CPUCores, r.MemoryGB, r.StorageGB, r.MonthlyUSD, r.BudgetStatus)
	}
}

func cmdEstimateFleet(args []string) {
	fs := newFlagSet("estimate-fleet")
	space := fs.String("space", "", "space glob, e.g. 'cost-demo-*'")
	where := fs.String("where", "", "ConfigHub where-filter expression")
	pricebook := fs.String("pricebook", "pricing/pricebook.json", "price book path")
	budgets := fs.String("budgets", "", "budgets override file")
	writeBack := fs.Bool("write-back", false, "record estimates onto the Units as data")
	statusSpace := fs.String("status-space", "", "publish a pricebook-status Unit into this Space (with --write-back)")
	failOnOver := fs.Bool("fail-on-over", false, "exit non-zero if any workload is OVER budget")
	asJSON := fs.Bool("json", false, "emit JSON")
	_ = fs.Parse(args)
	if *space == "" {
		usage()
	}
	pb := mustPricebook(*pricebook)
	bm := budgetsFrom(pb, *budgets)

	units, err := chub.Inventory(*space, *where)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	estimatedAt := time.Now().UTC().Format(time.RFC3339)

	// Accumulate one cost-estimate-record per Space (published after the loop).
	type spaceRecord struct {
		id      string
		reports []cost.Report
	}
	records := map[string]*spaceRecord{}
	var recordOrder []string

	var all []cost.Report
	anyOver := false
	for _, u := range units {
		w, err := k8s.Parse([]byte(u.YAML))
		if err != nil {
			continue // not a workload
		}
		r := cost.Estimate(w, u.Labels["Region"], u.Labels["Environment"], pb, bm)
		r.Unit, r.Space = u.Unit, u.Space
		all = append(all, r)
		if r.BudgetStatus == cost.StatusOver {
			anyOver = true
		}

		if *writeBack {
			desc := fmt.Sprintf("Cost estimate: $%.2f/mo (%s)", r.MonthlyUSD, r.BudgetStatus)
			annos := map[string]string{
				annoPrefix + "monthly-usd":     strconv.FormatFloat(r.MonthlyUSD, 'f', 2, 64),
				annoPrefix + "budget-status":   r.BudgetStatus,
				annoPrefix + "cpu-cores":       strconv.FormatFloat(r.CPUCores, 'f', 3, 64),
				annoPrefix + "memory-gb":       strconv.FormatFloat(r.MemoryGB, 'f', 3, 64),
				annoPrefix + "storage-gb":      strconv.FormatFloat(r.StorageGB, 'f', 3, 64),
				annoPrefix + "estimated-at":    estimatedAt,
				annoPrefix + "pricing-version": pb.Version,
			}
			if err := chub.SetAnnotations(u, annos, desc); err != nil {
				fmt.Fprintf(os.Stderr, "  warn: write-back %s/%s: %v\n", u.Space, u.Unit, err)
			}
			rec := records[u.SpaceID]
			if rec == nil {
				rec = &spaceRecord{id: u.SpaceID}
				records[u.SpaceID] = rec
				recordOrder = append(recordOrder, u.SpaceID)
			}
			rec.reports = append(rec.reports, r)
		}
	}

	if *writeBack {
		for _, sid := range recordOrder {
			rec := records[sid]
			sort.Slice(rec.reports, func(i, j int) bool { return rec.reports[i].Unit < rec.reports[j].Unit })
			data := marshalRecord(rec.reports, estimatedAt, pb.Version)
			labels := map[string]string{"app": "cost-estimator", "role": "cost-record"}
			if err := chub.UpsertUnit(sid, "cost-estimate-record", "AppConfig/YAML",
				"Cost estimate record", labels, data, "Publish cost estimate record"); err != nil {
				fmt.Fprintf(os.Stderr, "  warn: publish record for space %s: %v\n", sid, err)
			}
		}
		if *statusSpace != "" {
			if err := publishStatus(*statusSpace, pb, estimatedAt); err != nil {
				fmt.Fprintf(os.Stderr, "  warn: publish pricebook-status: %v\n", err)
			}
		}
	}

	if *asJSON {
		printJSON(all)
	} else {
		fmt.Printf("Estimated %d workload(s) at pricing %s.\n", len(all), pb.Version)
		for _, r := range all {
			fmt.Printf("  %-20s %-22s $%9.2f/mo  %s\n", r.Space, r.Unit, r.MonthlyUSD, r.BudgetStatus)
		}
	}
	if *failOnOver && anyOver {
		os.Exit(3)
	}
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

func publishStatus(statusSpace string, pb *pricing.PriceBook, at string) error {
	sid, err := chub.ResolveSpaceID(statusSpace)
	if err != nil {
		return err
	}
	doc := map[string]any{
		"pricingVersion": pb.Version,
		"currency":       pb.Currency,
		"generatedAt":    at,
		"rates":          pb.Rates,
		"budgets":        pb.BudgetsMonthlyUSD,
	}
	data, _ := yaml.Marshal(doc)
	return chub.UpsertUnit(sid, "pricebook-status", "AppConfig/YAML",
		"Price book status", map[string]string{"app": "cost-estimator", "role": "pricebook-status"},
		data, "Publish price book status")
}
