// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package cli is the k8s-to-score command surface.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"

	"github.com/confighub/examples/k8s-to-score/internal/convert"
	"github.com/confighub/examples/k8s-to-score/internal/cub"
	"github.com/confighub/examples/k8s-to-score/internal/version"
)

var opts struct {
	space       string
	where       string
	outDir      string
	fromDir     string
	explain     bool
	explainJSON bool
	reportJSON  bool
	stdout      bool
}

// Execute runs the command, returning a process exit code.
func Execute() int {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	return 0
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "k8s-to-score",
		Short: "Convert Kubernetes resources in a ConfigHub Space into Score workload specs",
		Long: `k8s-to-score reads the Kubernetes config data held in the Units of a ConfigHub
Space and writes one score.dev/v1b1 Workload file per Deployment or StatefulSet.

Deployments and StatefulSets become Score workloads. The Services, Ingresses,
ConfigMaps, Secrets and PersistentVolumeClaims around them are folded in as
service ports, route resources, files and volume resources.

The output is what score-k8s consumes:

    k8s-to-score --space my-app-prod --out-dir score/
    score-k8s init --no-sample && score-k8s generate score/*.yaml

This command only reads from ConfigHub. It never mutates a Unit, a Space, or
live infrastructure.`,
		Version:      version.Version,
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE:         run,
	}

	f := cmd.Flags()
	f.StringVar(&opts.space, "space", "", "ConfigHub Space slug to read Units from (required unless --from-dir)")
	f.StringVar(&opts.where, "where", "", "optional ConfigHub filter over Units, e.g. \"Labels.Tier = 'web'\"")
	f.StringVarP(&opts.outDir, "out-dir", "o", "score", "directory to write one <workload>.yaml per workload into")
	f.StringVar(&opts.fromDir, "from-dir", "", "convert local Kubernetes YAML files instead of a Space (for testing without a session)")
	f.BoolVar(&opts.stdout, "stdout", false, "write workloads to stdout as a multi-document stream instead of files")
	f.BoolVar(&opts.explain, "explain", false, "describe what the conversion would do, without reading ConfigHub")
	f.BoolVar(&opts.explainJSON, "explain-json", false, "machine-readable form of --explain")
	f.BoolVar(&opts.reportJSON, "report-json", false, "after converting, print the warning and skip report as JSON on stdout")

	return cmd
}

func run(cmd *cobra.Command, _ []string) error {
	switch {
	case opts.explainJSON:
		return printExplainJSON(cmd)
	case opts.explain:
		return printExplain(cmd)
	}

	units, err := loadUnits(cmd.Context())
	if err != nil {
		return err
	}
	if len(units) == 0 {
		return fmt.Errorf("no Units with config data found — check --space and --where")
	}

	result, err := convert.Convert(units)
	if err != nil {
		return err
	}
	if len(result.Workloads) == 0 {
		return fmt.Errorf("found %d Unit(s) but no Deployment or StatefulSet to convert into a Score workload", len(units))
	}

	if err := emit(cmd, result); err != nil {
		return err
	}
	return report(cmd, result)
}

// loadUnits reads config data either from a ConfigHub Space or, for testing,
// from a directory of Kubernetes YAML files.
func loadUnits(ctx context.Context) ([]convert.UnitData, error) {
	if opts.fromDir != "" {
		return loadFromDir(opts.fromDir)
	}
	if opts.space == "" {
		return nil, fmt.Errorf("--space is required (or use --from-dir to convert local files)")
	}
	client, err := cub.Preflight(ctx)
	if err != nil {
		return nil, err
	}
	units, err := cub.ListUnits(ctx, client, opts.space, opts.where)
	if err != nil {
		return nil, err
	}
	out := make([]convert.UnitData, 0, len(units))
	for _, u := range units {
		out = append(out, convert.UnitData{Slug: u.Slug, Data: u.Data})
	}
	return out, nil
}

func loadFromDir(dir string) ([]convert.UnitData, error) {
	var out []convert.UnitData
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		switch filepath.Ext(e.Name()) {
		case ".yaml", ".yml":
		default:
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		out = append(out, convert.UnitData{Slug: e.Name(), Data: data})
	}
	return out, nil
}

// emit writes each workload as its own file, which is the unit score-k8s
// consumes: a Score file holds exactly one workload.
func emit(cmd *cobra.Command, result *convert.Result) error {
	if opts.stdout {
		for i, w := range result.Workloads {
			data, err := yaml.Marshal(w.Workload)
			if err != nil {
				return err
			}
			if i > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "---")
			}
			fmt.Fprint(cmd.OutOrStdout(), string(data))
		}
		return nil
	}

	if err := os.MkdirAll(opts.outDir, 0o755); err != nil {
		return err
	}
	for _, w := range result.Workloads {
		data, err := yaml.Marshal(w.Workload)
		if err != nil {
			return err
		}
		path := filepath.Join(opts.outDir, w.Name+".yaml")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return err
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "wrote %s (%s %s from unit %s)\n", path, w.Kind, w.Name, w.UnitSlug)
	}
	return nil
}

// report prints what the conversion could not express. Warnings go to stderr so
// that --stdout stays a clean YAML stream.
func report(cmd *cobra.Command, result *convert.Result) error {
	if opts.reportJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{
			"workloads": workloadNames(result),
			"warnings":  result.Warnings,
			"skipped":   result.Skipped,
		})
	}

	err := cmd.ErrOrStderr()
	for _, s := range result.Skipped {
		fmt.Fprintf(err, "skipped: %s/%s (unit %s) has no Score representation\n", s.Kind, s.Name, s.Unit)
	}
	for _, w := range result.Warnings {
		fmt.Fprintf(err, "warning: %s: %s\n", w.Workload, w.Message)
	}
	if n := len(result.Warnings); n > 0 {
		fmt.Fprintf(err, "\n%d warning(s): the Score files are complete in shape but some values need review.\n", n)
	}
	return nil
}

func workloadNames(result *convert.Result) []string {
	names := make([]string, 0, len(result.Workloads))
	for _, w := range result.Workloads {
		names = append(names, w.Name)
	}
	return names
}
