// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/confighub/sdk/cliutil"

	"github.com/confighub/examples/autoscale-manager/internal/cub"
)

// newSetHPACmd edits an existing HorizontalPodAutoscaler Unit's replica bounds
// and/or cpu/memory utilization targets.
func newSetHPACmd() *cobra.Command {
	var output string
	var min, max, cpu, memory int
	var commit cliutil.CommitFlags
	cmd := &cobra.Command{
		Use:   "set-hpa <space>/<unit>",
		Short: "Edit a HorizontalPodAutoscaler's min/max replicas and cpu/memory targets (dry-run unless --commit)",
		Long: `set-hpa edits an existing HorizontalPodAutoscaler Unit (autoscaling/v2):

  --min N        set spec.minReplicas
  --max N        set spec.maxReplicas
  --cpu PCT      set a cpu Resource metric at PCT average Utilization
  --memory PCT   set a memory Resource metric at PCT average Utilization

At least one flag is required. --cpu / --memory replace spec.metrics with the
given Resource utilization target(s); --min / --max only touch the replica bounds.
To add an HPA to a workload that has none, or to convert an HPA to KEDA, see
'convert-keda'. Dry-run unless --commit --change-desc; never bypasses ApplyGates.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			flags := cmd.Flags()
			var parts []string
			if flags.Changed("min") {
				parts = append(parts, fmt.Sprintf(".spec.minReplicas = %d", min))
			}
			if flags.Changed("max") {
				parts = append(parts, fmt.Sprintf(".spec.maxReplicas = %d", max))
			}
			metrics := resourceMetrics(flags.Changed("cpu"), cpu, flags.Changed("memory"), memory)
			if metrics != "" {
				parts = append(parts, ".spec.metrics = "+metrics)
			}
			if len(parts) == 0 {
				return fmt.Errorf("nothing to change: pass at least one of --min, --max, --cpu, --memory")
			}
			if flags.Changed("min") && flags.Changed("max") && min > max {
				return fmt.Errorf("--min (%d) must not exceed --max (%d)", min, max)
			}
			expr := strings.Join(parts, " | ")
			changeDesc, dryRun, err := commit.Validate(fmt.Sprintf("set HPA bounds/targets on %s", args[0]))
			if err != nil {
				return err
			}
			return runYQ(cmd, "set-hpa", args[0], expr, dryRun, changeDesc, output)
		},
	}
	addOutputFlag(cmd, &output)
	commit.Bind(cmd)
	cmd.Flags().IntVar(&min, "min", 0, "spec.minReplicas")
	cmd.Flags().IntVar(&max, "max", 0, "spec.maxReplicas")
	cmd.Flags().IntVar(&cpu, "cpu", 0, "cpu target as average Utilization percent")
	cmd.Flags().IntVar(&memory, "memory", 0, "memory target as average Utilization percent")
	return cmd
}

// resourceMetrics builds a yq array literal of Resource/Utilization metric entries
// for the requested cpu/memory targets, or "" if neither was requested.
func resourceMetrics(hasCPU bool, cpu int, hasMemory bool, memory int) string {
	var entries []string
	if hasCPU {
		entries = append(entries, resourceMetricEntry("cpu", cpu))
	}
	if hasMemory {
		entries = append(entries, resourceMetricEntry("memory", memory))
	}
	if len(entries) == 0 {
		return ""
	}
	return "[" + strings.Join(entries, ", ") + "]"
}

func resourceMetricEntry(name string, pct int) string {
	return fmt.Sprintf(`{"type": "Resource", "resource": {"name": %q, "target": {"type": "Utilization", "averageUtilization": %d}}}`, name, pct)
}

// runYQ resolves the target and applies a set-yq expression, dry-run or commit.
func runYQ(cmd *cobra.Command, command, target, expr string, dryRun bool, changeDesc, output string) error {
	client, err := cub.Preflight(cmd.Context())
	if err != nil {
		return err
	}
	ref, err := parseUnitRef(cmd.Context(), client, target)
	if err != nil {
		return err
	}
	res, err := cub.MutateUnitYQ(cmd.Context(), client, expr, ref.selector(), changeOf(changeDesc, dryRun))
	if err != nil {
		return err
	}
	return reportMutation(cmd, command, ref.spaceSlug, dryRun, output, res)
}
