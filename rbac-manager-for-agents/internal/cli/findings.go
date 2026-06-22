// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/confighub/examples/rbac-manager-for-agents/internal/cub"
	"github.com/confighub/examples/rbac-manager-for-agents/internal/rbac"
	"github.com/confighub/examples/rbac-manager-for-agents/internal/snapshot"
)

type findingRow struct {
	Severity  rbac.Severity `json:"severity"`
	Analyzer  string        `json:"analyzer"`
	Cluster   string        `json:"cluster"`
	Kind      string        `json:"kind"`
	Namespace string        `json:"namespace,omitempty"`
	Name      string        `json:"name"`
	Unit      string        `json:"unit"`
	Message   string        `json:"message"`
}

func newFindingsCmd() *cobra.Command {
	var output, severityFilter, analyzerFilter string
	var scope scopeFlags
	cmd := &cobra.Command{
		Use:   "findings",
		Short: "Report RBAC hygiene findings across the fleet",
		Long: `findings runs the RBAC hygiene analyzers over the fleet and reports issues:

  wildcard-rules             wildcard verbs/resources/apiGroups in a role
  privilege-escalation-verbs escalate / bind / impersonate
  risky-grants               secrets, pod exec/attach, webhook/CRD writes
  cluster-admin-bindings     bindings to cluster-admin or equivalent superuser roles
  orphaned-bindings          bindings whose role does not exist on the cluster
  unbound-service-accounts   ServiceAccounts with no bindings

These are analysis-only; enforcement is server-side via Triggers/ApplyGates.
Filter with --severity (high|medium|low) and --analyzer.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if severityFilter != "" {
				switch severityFilter {
				case "high", "medium", "low":
				default:
					return fmt.Errorf("invalid --severity %q: use high, medium, or low", severityFilter)
				}
			}
			if err := cub.Preflight(cmd.Context()); err != nil {
				return err
			}
			snap, err := snapshot.Load(cmd.Context(), scope.scope())
			if err != nil {
				return err
			}
			rows := findingRows(rbac.AnalyzeFleet(snap.Clusters), severityFilter, analyzerFilter)
			if output == outputTable {
				printFindingsTable(cmd, rows)
				return nil
			}
			return printJSON(cmd.OutOrStdout(), rows)
		},
	}
	addOutputFlag(cmd, &output)
	addScopeFlags(cmd, &scope)
	cmd.Flags().StringVar(&severityFilter, "severity", "", "filter by severity: high | medium | low")
	cmd.Flags().StringVar(&analyzerFilter, "analyzer", "", "filter by analyzer name (e.g. wildcard-rules)")
	return cmd
}

func findingRows(findings []rbac.Finding, severityFilter, analyzerFilter string) []findingRow {
	rows := make([]findingRow, 0, len(findings))
	for _, f := range findings {
		if severityFilter != "" && string(f.Severity) != severityFilter {
			continue
		}
		if analyzerFilter != "" && f.Analyzer != analyzerFilter {
			continue
		}
		rows = append(rows, findingRow{
			Severity:  f.Severity,
			Analyzer:  f.Analyzer,
			Cluster:   f.Cluster,
			Kind:      f.ResourceKind,
			Namespace: f.Namespace,
			Name:      f.ResourceName,
			Unit:      f.Origin.UnitSlug,
			Message:   f.Message,
		})
	}
	return rows
}

func printFindingsTable(cmd *cobra.Command, rows []findingRow) {
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "SEVERITY\tANALYZER\tCLUSTER\tKIND\tRESOURCE\tMESSAGE")
	for _, r := range rows {
		name := r.Name
		if r.Namespace != "" {
			name = r.Namespace + "/" + r.Name
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			strings.ToUpper(string(r.Severity)), r.Analyzer, r.Cluster, r.Kind, name, r.Message)
	}
	_ = tw.Flush()
	fprintln(cmd.OutOrStdout(), fmt.Sprintf("\n%d findings", len(rows)))
}
