// Package cost turns a parsed workload + the cost database into a monthly cost
// estimate and a discrete budget verdict (the signal the within-budget guardrail
// gates on).
package cost

import (
	"math"
	"strings"

	"costest/internal/costdb"
	"costest/internal/k8s"
)

// Budget status values. OVER is what the within-budget guardrail blocks on;
// UNKNOWN means the workload couldn't be costed (missing resource requests, or
// no budget configured for its environment).
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
	SpaceID        string  `json:"-" yaml:"-"` // identity for write-back; not serialized
	UnitID         string  `json:"-" yaml:"-"`
	Kind           string  `json:"kind" yaml:"kind"`
	Name           string  `json:"name" yaml:"name"`
	Replicas       int     `json:"replicas" yaml:"replicas"`
	CPUCores       float64 `json:"cpu_cores" yaml:"cpu_cores"`
	MemoryGB       float64 `json:"memory_gb" yaml:"memory_gb"`
	StorageGB      float64 `json:"storage_gb" yaml:"storage_gb"`
	GPUUnits       float64 `json:"gpu_units" yaml:"gpu_units"`
	Provider       string  `json:"provider" yaml:"provider"`
	Region         string  `json:"region" yaml:"region"`
	Environment    string  `json:"environment,omitempty" yaml:"environment,omitempty"`
	MonthlyUSD     float64 `json:"monthly_usd" yaml:"monthly_usd"`
	BudgetUSD      float64 `json:"budget_usd" yaml:"budget_usd"`
	BudgetStatus   string  `json:"budget_status" yaml:"budget_status"`
	MissingRequest bool    `json:"missing_requests" yaml:"missing_requests"`
	PricingVersion string  `json:"pricing_version" yaml:"pricing_version"`
}

// Estimate costs a workload. provider/region come from the unit's labels
// (default to the cost DB's defaults); env selects the budget.
func Estimate(w *k8s.Workload, provider, region, env string, book *costdb.Book) Report {
	if provider == "" {
		provider = book.DefaultProvider
	}
	if region == "" {
		region = book.DefaultRegion
	}

	var cpu, mem, gpu float64
	for _, c := range w.Containers {
		cpu += c.CPU
		mem += c.Memory
		gpu += c.GPU
	}
	reps := float64(w.Replicas)
	cpuTotal, memTotal, gpuTotal := cpu*reps, mem*reps, gpu*reps

	cpuRate, _ := book.Rate(provider, region, "cpu")
	memRate, _ := book.Rate(provider, region, "memory")
	gpuRate, _ := book.Rate(provider, region, "gpu")
	stoRate, _ := book.Rate(provider, region, "storage")

	compute := (cpuTotal*cpuRate + memTotal*memRate + gpuTotal*gpuRate) * book.HoursPerMonth
	monthly := compute + w.StorageGB*stoRate

	budget, hasBudget := book.Budget(strings.ToLower(env))
	r := Report{
		Kind:           w.Kind,
		Name:           w.Name,
		Replicas:       w.Replicas,
		CPUCores:       round(cpuTotal, 3),
		MemoryGB:       round(memTotal, 3),
		StorageGB:      round(w.StorageGB, 3),
		GPUUnits:       round(gpuTotal, 3),
		Provider:       provider,
		Region:         region,
		Environment:    env,
		MonthlyUSD:     round(monthly, 2),
		BudgetUSD:      budget,
		MissingRequest: w.MissingRequests(),
		PricingVersion: book.Version,
	}
	r.BudgetStatus = status(r.MissingRequest, hasBudget, monthly, budget)
	return r
}

func status(missing, hasBudget bool, monthly, budget float64) string {
	if missing {
		return StatusUnknown // can't cost an unbounded workload
	}
	if !hasBudget || budget <= 0 {
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

func round(v float64, places int) float64 {
	p := math.Pow(10, float64(places))
	return math.Round(v*p) / p
}
