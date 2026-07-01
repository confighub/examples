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

	"github.com/confighub/examples/observability-manager/internal/cub"
	"github.com/confighub/examples/observability-manager/internal/observability"
	"github.com/confighub/examples/observability-manager/internal/snapshot"
)

type ensureSMPlan struct {
	Action    string `json:"action"`
	DryRun    bool   `json:"dryRun"`
	Space     string `json:"space"`
	Namespace string `json:"namespace,omitempty"`
	Unit      string `json:"unit"`
	UnitID    string `json:"unitId,omitempty"`
	Revision  int64  `json:"revision,omitempty"`
	Manifest  string `json:"manifest"`
}

func newEnsureServiceMonitorCmd() *cobra.Command {
	var output, portName string
	var commit cliutil.CommitFlags
	cmd := &cobra.Command{
		Use:   "ensure-servicemonitor <space>/<service-unit>",
		Short: "Author a ServiceMonitor whose selector is derived from a Service",
		Long: `ensure-servicemonitor reads a Service Unit's labels, namespace, and metrics port
and authors a new ServiceMonitor Unit that selects it. The ServiceMonitor is
created in the same Space as the Service (its own Unit, per one-resource-per-Unit),
but not applied — deploying it is a separate 'cub unit apply'.

The endpoint port defaults to the Service's metrics port (--port to override).
Refuses when the Service has no metrics port and no --port. Dry-run unless
--commit --change-desc.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			changeDesc, dryRun, err := commit.Validate(fmt.Sprintf("ensure ServiceMonitor for %s", args[0]))
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
			snap, err := snapshot.Load(cmd.Context(), client, fmt.Sprintf("SpaceID = '%s'", ref.spaceID.String()))
			if err != nil {
				return err
			}
			svc := findServiceBySlug(snap, ref.unitSlug)
			if svc == nil {
				return fmt.Errorf("no Service found in Unit %s/%s", ref.spaceSlug, ref.unitSlug)
			}
			if len(svc.Labels) == 0 {
				return fmt.Errorf("Service %s has no labels — cannot derive a ServiceMonitor selector", svc.Name)
			}
			port := portName
			if port == "" {
				port = serviceMetricsPort(svc)
			}
			if port == "" {
				return fmt.Errorf("Service %s has no named metrics port — pass --port", svc.Name)
			}

			smSlug := svc.Name + "-sm"
			manifest := renderServiceMonitor(smSlug, svc.Namespace, svc.Labels, port)
			plan := ensureSMPlan{Action: "create-servicemonitor", DryRun: dryRun, Space: ref.spaceSlug, Namespace: svc.Namespace, Unit: smSlug, Manifest: manifest}
			out := cmd.OutOrStdout()
			if dryRun {
				if output == outputJSON {
					return printJSON(out, plan)
				}
				fprintln(out, fmt.Sprintf("Dry run — would create ServiceMonitor Unit %s/%s:", ref.spaceSlug, smSlug))
				fprintln(out, "")
				fprintln(out, manifest)
				fprintln(out, "Re-run with --commit --change-desc \"…\" to create the Unit.")
				return nil
			}
			created, err := cub.CreateUnit(cmd.Context(), client, goclientnew.Unit{
				Slug:                  smSlug,
				DisplayName:           smSlug,
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
			fprintln(out, fmt.Sprintf("Created ServiceMonitor Unit %s/%s (revision %d). Apply it to deploy: cub unit apply --space %s %s",
				ref.spaceSlug, smSlug, created.HeadRevisionNum, ref.spaceSlug, smSlug))
			return nil
		},
	}
	addOutputFlag(cmd, &output)
	commit.Bind(cmd)
	cmd.Flags().StringVar(&portName, "port", "", "endpoint port name (default: the Service's metrics port)")
	return cmd
}

func findServiceBySlug(snap *snapshot.Snapshot, unitSlug string) *observability.ServiceEntity {
	for _, c := range snap.Clusters {
		for _, s := range c.Services {
			if s.Origin.UnitSlug == unitSlug {
				return s
			}
		}
	}
	return nil
}

// serviceMetricsPort returns the Service's metrics port name, if any.
func serviceMetricsPort(svc *observability.ServiceEntity) string {
	for _, p := range svc.Ports {
		switch strings.ToLower(p.Name) {
		case "metrics", "http-metrics", "monitoring", "prometheus", "telemetry":
			return p.Name
		}
	}
	return ""
}

func renderServiceMonitor(name, namespace string, labels map[string]string, port string) string {
	var b strings.Builder
	b.WriteString("apiVersion: monitoring.coreos.com/v1\n")
	b.WriteString("kind: ServiceMonitor\n")
	b.WriteString("metadata:\n")
	b.WriteString("  name: " + name + "\n")
	if namespace != "" {
		b.WriteString("  namespace: " + namespace + "\n")
	}
	b.WriteString("spec:\n")
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
	b.WriteString("  endpoints:\n")
	b.WriteString("  - port: " + port + "\n")
	return b.String()
}
