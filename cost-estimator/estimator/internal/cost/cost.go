// Package cost turns a parsed workload + a price book into a monthly cost
// estimate and a discrete budget verdict (the signal the within-budget guardrail
// gates on).
package cost

import (
	"math"

	"costest/internal/k8s"
	"costest/internal/pricing"
)

// Budget status values. OVER is what the within-budget guardrail blocks on;
// UNKNOWN means the workload couldn't be costed (missing resource requests).
const (
	StatusOK      = "OK"
	StatusWarn    = "WARN"
	StatusOver    = "OVER"
	StatusUnknown = "UNKNOWN"
)

// warnRatio is the fraction of budget at which a workload flips OK → WARN.
const warnRatio = 0.8

// Report is the cost estimate for one workload.
type Report struct {
	Unit           string  `json:"unit,omitempty" yaml:"unit,omitempty"`
	Space          string  `json:"space,omitempty" yaml:"space,omitempty"`
	Kind           string  `json:"kind" yaml:"kind"`
	Name           string  `json:"name" yaml:"name"`
	Replicas       int     `json:"replicas" yaml:"replicas"`
	CPUCores       float64 `json:"cpu_cores" yaml:"cpu_cores"`
	MemoryGB       float64 `json:"memory_gb" yaml:"memory_gb"`
	StorageGB      float64 `json:"storage_gb" yaml:"storage_gb"`
	Region         string  `json:"region" yaml:"region"`
	Environment    string  `json:"environment,omitempty" yaml:"environment,omitempty"`
	MonthlyUSD     float64 `json:"monthly_usd" yaml:"monthly_usd"`
	BudgetUSD      float64 `json:"budget_usd" yaml:"budget_usd"`
	BudgetStatus   string  `json:"budget_status" yaml:"budget_status"`
	MissingRequest bool    `json:"missing_requests" yaml:"missing_requests"`
	PricingVersion string  `json:"pricing_version" yaml:"pricing_version"`
}

// Estimate costs a workload. region/env come from the unit's labels (region
// drives the price multiplier; env selects the budget).
func Estimate(w *k8s.Workload, region, env string, pb *pricing.PriceBook, budgets map[string]float64) Report {
	var cpu, mem float64
	for _, c := range w.Containers {
		cpu += c.CPU
		mem += c.Memory
	}
	reps := float64(w.Replicas)
	cpuTotal := cpu * reps
	memTotal := mem * reps

	mult := pb.Multiplier(region)
	compute := (cpuTotal*pb.Rates.CPUCoreHour + memTotal*pb.Rates.MemoryGBHour) * pb.HoursPerMonth
	storage := w.StorageGB * pb.Rates.StorageGBMonth
	monthly := (compute + storage) * mult

	budget := pricing.Budget(budgets, env)
	r := Report{
		Kind:           w.Kind,
		Name:           w.Name,
		Replicas:       w.Replicas,
		CPUCores:       round(cpuTotal, 3),
		MemoryGB:       round(memTotal, 3),
		StorageGB:      round(w.StorageGB, 3),
		Region:         regionOr(region, pb.DefaultRegion),
		Environment:    env,
		MonthlyUSD:     round(monthly, 2),
		BudgetUSD:      budget,
		MissingRequest: w.MissingRequests(),
		PricingVersion: pb.Version,
	}
	r.BudgetStatus = status(r.MissingRequest, monthly, budget)
	return r
}

func status(missing bool, monthly, budget float64) string {
	if missing {
		return StatusUnknown // can't cost an unbounded workload
	}
	if budget <= 0 {
		return StatusUnknown // no budget configured for this environment
	}
	switch ratio := monthly / budget; {
	case ratio > 1.0:
		return StatusOver
	case ratio >= warnRatio:
		return StatusWarn
	default:
		return StatusOK
	}
}

func regionOr(region, def string) string {
	if region == "" {
		return def
	}
	return region
}

func round(v float64, places int) float64 {
	p := math.Pow(10, float64(places))
	return math.Round(v*p) / p
}
