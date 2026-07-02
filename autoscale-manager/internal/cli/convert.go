// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/confighub/sdk/cliutil"

	"github.com/confighub/examples/autoscale-manager/internal/cub"
	"github.com/confighub/examples/autoscale-manager/internal/localexec"
)

type convertPlan struct {
	Action   string `json:"action"`
	DryRun   bool   `json:"dryRun"`
	Space    string `json:"space"`
	Unit     string `json:"unit"`
	Changed  bool   `json:"changed"`
	Revision int64  `json:"revision,omitempty"`
	Manifest string `json:"manifest,omitempty"`
}

func newConvertCmd() *cobra.Command {
	var output string
	var commit cliutil.CommitFlags
	cmd := &cobra.Command{
		Use:   "convert-keda <space>/<unit>",
		Short: "Convert a HorizontalPodAutoscaler Unit to a KEDA ScaledObject (dry-run unless --commit)",
		Long: `convert-keda rewrites a Unit's HorizontalPodAutoscaler into an equivalent KEDA
ScaledObject, preserving min/max replicas and cpu/memory metrics.

The conversion runs the convert-hpa-to-keda ConfigHub function in an embedded
executor in-process: the command fetches the Unit's data, runs the function on
it locally (no server-side function, no worker), and writes the result back as a
new Unit revision. cpu/memory Resource metrics become KEDA cpu/memory triggers;
Pods/Object/External metrics are not converted (KEDA needs a matching scaler).

Dry-run by default (prints the resulting ScaledObject); --commit --change-desc
writes the revision. The Unit is edited, not applied — rolling it out is a
separate 'cub unit apply'. Deploying KEDA ScaledObjects also requires the KEDA
operator installed in the target cluster.

To schema-validate the generated ScaledObject automatically, install the
guardrails pack ('cub-autoscale guardrails install --commit'): its schema-valid
trigger runs vet-schemas on every mutation, and keda.sh is in the schema catalog.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			space, unit, ok := strings.Cut(args[0], "/")
			if !ok || space == "" || unit == "" {
				return fmt.Errorf("target must be <space>/<unit>, got %q", args[0])
			}
			changeDesc, dryRun, err := commit.Validate(fmt.Sprintf("convert HPA to KEDA ScaledObject in %s", args[0]))
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			client, err := cub.Preflight(ctx)
			if err != nil {
				return err
			}
			ref, err := cub.ResolveUnit(ctx, client, space, unit)
			if err != nil {
				return err
			}
			encoded, err := cub.GetUnitData(ctx, client, ref)
			if err != nil {
				return err
			}
			yamlBytes, err := base64.StdEncoding.DecodeString(encoded)
			if err != nil {
				return fmt.Errorf("decode unit data: %w", err)
			}

			mutated, changed, err := localexec.ConvertHPAToKEDA(ctx, yamlBytes)
			if err != nil {
				return err
			}

			plan := convertPlan{Action: "convert-hpa-to-keda", DryRun: dryRun, Space: space, Unit: unit, Changed: changed}
			out := cmd.OutOrStdout()
			if !changed {
				plan.Manifest = string(yamlBytes)
				if output == outputJSON {
					return printJSON(out, plan)
				}
				fprintln(out, fmt.Sprintf("No HorizontalPodAutoscaler to convert in %s/%s — nothing to do.", space, unit))
				return nil
			}
			plan.Manifest = string(mutated)
			if dryRun {
				if output == outputJSON {
					return printJSON(out, plan)
				}
				fprintln(out, fmt.Sprintf("Dry run — would convert the HPA in %s/%s to a KEDA ScaledObject:", space, unit))
				fprintln(out, "")
				fprintln(out, string(mutated))
				fprintln(out, "Re-run with --commit --change-desc \"…\" to write the revision. It is not applied until you apply it.")
				return nil
			}

			if err := cub.PatchUnitData(ctx, client, ref, base64.StdEncoding.EncodeToString(mutated), changeDesc); err != nil {
				return err
			}
			if output == outputJSON {
				return printJSON(out, plan)
			}
			fprintln(out, fmt.Sprintf("Converted %s/%s to a KEDA ScaledObject (new revision). Apply it to deploy: cub unit apply --space %s %s", space, unit, space, unit))
			return nil
		},
	}
	addOutputFlag(cmd, &output)
	commit.Bind(cmd)
	return cmd
}
