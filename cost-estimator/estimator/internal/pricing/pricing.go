// Package pricing loads the static price book (pricing/pricebook.json) and the
// per-environment budgets the estimator costs against.
package pricing

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// PriceBook is the versioned cost model: hourly rates, a per-region multiplier,
// and per-environment monthly budgets.
type PriceBook struct {
	Version       string `json:"version"`
	Currency      string `json:"currency"`
	HoursPerMonth float64 `json:"hours_per_month"`
	Rates         struct {
		CPUCoreHour    float64 `json:"cpu_core_hour"`
		MemoryGBHour   float64 `json:"memory_gb_hour"`
		StorageGBMonth float64 `json:"storage_gb_month"`
	} `json:"rates"`
	DefaultRegion     string             `json:"default_region"`
	RegionMultipliers map[string]float64 `json:"region_multipliers"`
	BudgetsMonthlyUSD map[string]float64 `json:"budgets_monthly_usd"`
}

// Load reads a price book from disk.
func Load(path string) (*PriceBook, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read price book %q: %w", path, err)
	}
	var pb PriceBook
	if err := json.Unmarshal(b, &pb); err != nil {
		return nil, fmt.Errorf("parse price book %q: %w", path, err)
	}
	if pb.HoursPerMonth == 0 {
		pb.HoursPerMonth = 730
	}
	return &pb, nil
}

// Multiplier returns the cost multiplier for a region, falling back to the
// default region's multiplier (or 1.0).
func (p *PriceBook) Multiplier(region string) float64 {
	if region == "" {
		region = p.DefaultRegion
	}
	if m, ok := p.RegionMultipliers[region]; ok {
		return m
	}
	if m, ok := p.RegionMultipliers[p.DefaultRegion]; ok {
		return m
	}
	return 1.0
}

// Budget returns the monthly budget for an environment (case-insensitive),
// falling back to the "default" budget, or 0 if none is configured.
func Budget(budgets map[string]float64, env string) float64 {
	if budgets == nil {
		return 0
	}
	if v, ok := budgets[strings.ToLower(env)]; ok {
		return v
	}
	if v, ok := budgets["default"]; ok {
		return v
	}
	return 0
}

// LoadBudgets reads a budgets override file: {"budgets_monthly_usd": {...}} or a
// bare {"dev": 500, ...} object.
func LoadBudgets(path string) (map[string]float64, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read budgets %q: %w", path, err)
	}
	var wrapped struct {
		BudgetsMonthlyUSD map[string]float64 `json:"budgets_monthly_usd"`
	}
	if err := json.Unmarshal(b, &wrapped); err == nil && len(wrapped.BudgetsMonthlyUSD) > 0 {
		return wrapped.BudgetsMonthlyUSD, nil
	}
	var bare map[string]float64
	if err := json.Unmarshal(b, &bare); err != nil {
		return nil, fmt.Errorf("parse budgets %q: %w", path, err)
	}
	return bare, nil
}
