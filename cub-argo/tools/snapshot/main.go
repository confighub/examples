// Command snapshot reads the Workshop Catalog's flattening-safety verdicts out
// of a helm-expt checkout and writes the compact index the plugin embeds, so
// the plugin can answer offline.
//
//	go run ./tools/snapshot -helm-expt ../../helm-expt
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

type verdictFile struct {
	Spec struct {
		Chart struct {
			Repository    string `yaml:"repository"`
			Name          string `yaml:"name"`
			Version       string `yaml:"version"`
			PackageSHA256 string `yaml:"packageSHA256"`
		} `yaml:"chart"`
		AuditedBase  string `yaml:"auditedBase"`
		Dispositions []struct {
			Class   string `yaml:"class"`
			Finding string `yaml:"finding"`
		} `yaml:"dispositions"`
		VariantScope []struct {
			Values string `yaml:"values"`
			Effect string `yaml:"effect"`
		} `yaml:"variantScope"`
		Verdict struct {
			Lane      string `yaml:"lane"`
			Rationale string `yaml:"rationale"`
		} `yaml:"verdict"`
	} `yaml:"spec"`
}

// Entry is one audited (chart, version, values base).
type Entry struct {
	Repository string `json:"repository"`
	Chart      string `json:"chart"`
	Version    string `json:"version"`
	Base       string `json:"base"`
	Lane       string `json:"lane"`
	Rationale  string `json:"rationale,omitempty"`
	SHA256     string `json:"packageSHA256,omitempty"`
	// Hazards are the classes the audit found present, which is what an
	// overlay has to leave alone for this verdict to carry.
	Hazards []string `json:"hazards,omitempty"`
	// MovedBy are the values changes the audit says take a variant out of
	// this verdict's scope.
	MovedBy []struct {
		Values string `json:"values"`
		Effect string `json:"effect"`
	} `json:"movedBy,omitempty"`
}

func main() {
	root := flag.String("helm-expt", "../helm-expt", "a helm-expt checkout to read verdicts from")
	out := flag.String("out", "internal/catalog/verdicts.json", "where to write the index")
	flag.Parse()

	var entries []Entry
	err := filepath.WalkDir(*root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		name := d.Name()
		if len(name) < 26 || name[:26] != "flattening-safety-verdict-" {
			if name != "flattening-safety-verdict.yaml" {
				return nil
			}
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		var v verdictFile
		if err := yaml.Unmarshal(data, &v); err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		s := v.Spec
		if s.Chart.Name == "" || s.Verdict.Lane == "" {
			return nil
		}
		e := Entry{
			Repository: s.Chart.Repository, Chart: s.Chart.Name, Version: s.Chart.Version,
			Base: s.AuditedBase, Lane: s.Verdict.Lane, Rationale: s.Verdict.Rationale,
			SHA256: s.Chart.PackageSHA256,
		}
		// The finding matters as much as the class: two bases of one chart can
		// both list "lookup" while only one of them actually fires it.
		for _, d := range s.Dispositions {
			if d.Finding != "absent" && d.Class != "" {
				e.Hazards = append(e.Hazards, d.Class+"="+d.Finding)
			}
		}
		sort.Strings(e.Hazards)
		for _, m := range s.VariantScope {
			e.MovedBy = append(e.MovedBy, struct {
				Values string `json:"values"`
				Effect string `json:"effect"`
			}{m.Values, m.Effect})
		}
		entries = append(entries, e)
		return nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	sort.Slice(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if a.Repository != b.Repository {
			return a.Repository < b.Repository
		}
		if a.Chart != b.Chart {
			return a.Chart < b.Chart
		}
		if a.Version != b.Version {
			return a.Version < b.Version
		}
		return a.Base < b.Base
	})
	buf, err := json.MarshalIndent(entries, "", " ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile(*out, append(buf, '\n'), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("wrote %d verdicts to %s\n", len(entries), *out)
}
