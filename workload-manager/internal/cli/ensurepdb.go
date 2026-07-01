// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"encoding/base64"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/confighub/sdk/cliutil"
	goclientnew "github.com/confighub/sdk/core/openapi/goclient-new"

	"github.com/confighub/examples/workload-manager/internal/cub"
	"github.com/confighub/examples/workload-manager/internal/snapshot"
	"github.com/confighub/examples/workload-manager/internal/workload"
)

type ensurePDBPlan struct {
	Action    string `json:"action"`
	DryRun    bool   `json:"dryRun"`
	Space     string `json:"space"`
	Namespace string `json:"namespace,omitempty"`
	Unit      string `json:"unit"`
	UnitID    string `json:"unitId,omitempty"`
	Revision  int64  `json:"revision,omitempty"`
	Manifest  string `json:"manifest"`
}

func newEnsurePDBCmd() *cobra.Command {
	var output string
	var minAvailable, maxUnavailable string
	var commit cliutil.CommitFlags
	cmd := &cobra.Command{
		Use:   "ensure-pdb <space>/<unit>",
		Short: "Author a PodDisruptionBudget whose selector is derived from the workload",
		Long: `ensure-pdb reads a workload Unit's pod-template labels and namespace and authors
a new PodDisruptionBudget Unit whose selector matches exactly. The PDB is created
in the same Space as the workload (as its own Unit, per one-resource-per-Unit),
but not applied — deploying it is a separate 'cub unit apply'.

Set the policy with --min-available or --max-unavailable (default:
maxUnavailable 1, i.e. at most one pod down during a voluntary disruption).
Refuses to guess when the workload has no pod-template labels (an empty selector
would cover the whole namespace). Dry-run unless --commit --change-desc.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if minAvailable != "" && maxUnavailable != "" {
				return fmt.Errorf("set only one of --min-available / --max-unavailable")
			}
			policyKey, policyVal := "maxUnavailable", "1"
			switch {
			case minAvailable != "":
				policyKey, policyVal = "minAvailable", minAvailable
			case maxUnavailable != "":
				policyKey, policyVal = "maxUnavailable", maxUnavailable
			}
			changeDesc, dryRun, err := commit.Validate(fmt.Sprintf("ensure PodDisruptionBudget for %s (%s: %s)", args[0], policyKey, policyVal))
			if err != nil {
				return err
			}
			client, err := cub.Preflight(cmd.Context())
			if err != nil {
				return err
			}
			ref, err := parseUnitRef(cmd.Context(), client, args[0])
			if err != nil {
				return err
			}

			// Find the workload to derive its selector, namespace, and name.
			snap, err := snapshot.Load(cmd.Context(), client, fmt.Sprintf("SpaceID = '%s'", ref.spaceID.String()))
			if err != nil {
				return err
			}
			w := findWorkloadBySlug(snap, ref.unitSlug)
			if w == nil {
				return fmt.Errorf("no workload found in Unit %s/%s", ref.spaceSlug, ref.unitSlug)
			}
			if len(w.PodLabels) == 0 {
				return fmt.Errorf("workload %s has no pod-template labels — cannot derive a safe PDB selector", w.Name)
			}

			pdbSlug := w.Name + "-pdb"
			manifest := renderPDB(w.Name+"-pdb", w.Namespace, policyKey, policyVal, w.PodLabels)

			plan := ensurePDBPlan{
				Action: "create-pdb", DryRun: dryRun, Space: ref.spaceSlug,
				Namespace: w.Namespace, Unit: pdbSlug, Manifest: manifest,
			}
			out := cmd.OutOrStdout()
			if dryRun {
				if output == outputJSON {
					return printJSON(out, plan)
				}
				fprintln(out, fmt.Sprintf("Dry run — would create PDB Unit %s/%s:", ref.spaceSlug, pdbSlug))
				fprintln(out, "")
				fprintln(out, manifest)
				fprintln(out, "Re-run with --commit --change-desc \"…\" to create the Unit. It is not applied until you apply it.")
				return nil
			}

			created, err := cub.CreateUnit(cmd.Context(), client, goclientnew.Unit{
				Slug:                  pdbSlug,
				DisplayName:           pdbSlug,
				Data:                  base64.StdEncoding.EncodeToString([]byte(manifest)),
				ToolchainType:         "Kubernetes/YAML",
				SpaceID:               ref.spaceID,
				Labels:                map[string]string{"managed-by": unitManagedByLabel},
				LastChangeDescription: changeDesc,
			})
			if err != nil {
				return err
			}
			plan.UnitID = created.UnitID.String()
			plan.Revision = created.HeadRevisionNum
			if output == outputJSON {
				return printJSON(out, plan)
			}
			fprintln(out, fmt.Sprintf("Created PDB Unit %s/%s (revision %d). Apply it to deploy: cub unit apply --space %s %s",
				ref.spaceSlug, pdbSlug, created.HeadRevisionNum, ref.spaceSlug, pdbSlug))
			return nil
		},
	}
	addOutputFlag(cmd, &output)
	commit.Bind(cmd)
	cmd.Flags().StringVar(&minAvailable, "min-available", "", "PDB minAvailable (int or percentage, e.g. 2 or 80%)")
	cmd.Flags().StringVar(&maxUnavailable, "max-unavailable", "", "PDB maxUnavailable (int or percentage; default 1)")
	return cmd
}

// findWorkloadBySlug returns the first workload whose origin Unit slug matches.
func findWorkloadBySlug(snap *snapshot.Snapshot, unitSlug string) *workload.WorkloadEntity {
	for _, c := range snap.Clusters {
		for _, w := range c.Workloads {
			if w.Origin.UnitSlug == unitSlug {
				return w
			}
		}
	}
	return nil
}

// renderPDB produces literal PodDisruptionBudget YAML with the given selector.
func renderPDB(name, namespace, policyKey, policyVal string, labels map[string]string) string {
	var b strings.Builder
	b.WriteString("apiVersion: policy/v1\n")
	b.WriteString("kind: PodDisruptionBudget\n")
	b.WriteString("metadata:\n")
	b.WriteString("  name: " + name + "\n")
	if namespace != "" {
		b.WriteString("  namespace: " + namespace + "\n")
	}
	b.WriteString("spec:\n")
	b.WriteString("  " + policyKey + ": " + policyVal + "\n")
	b.WriteString("  selector:\n")
	b.WriteString("    matchLabels:\n")
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		b.WriteString("      " + k + ": " + labels[k] + "\n")
	}
	return b.String()
}
