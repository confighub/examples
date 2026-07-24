// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// The --explain pair is the contract the examples repo expects of a runnable
// example: a human-readable plan and a machine-readable one, neither of which
// touches ConfigHub or live infrastructure.

const explainText = `k8s-to-score — Kubernetes config in a ConfigHub Space -> Score workload specs

  ConfigHub Space
    Unit: checkout-deployment   (Deployment)  ─┐
    Unit: checkout-service      (Service)      ├─>  score/checkout.yaml
    Unit: checkout-config       (ConfigMap)    │      metadata.name: checkout
    Unit: checkout-ingress      (Ingress)     ─┘      containers, service, resources

Mapping:
  Deployment / StatefulSet   -> a Score workload (one file each)
    containers               -> containers[*].image, command, args
    env / envFrom            -> containers[*].variables
    resources                -> containers[*].resources.limits / .requests
    livenessProbe            -> containers[*].livenessProbe (httpGet, exec)
    readinessProbe           -> containers[*].readinessProbe
  Service (selector match)   -> service.ports
  Ingress                    -> resources[*] of type route (+ dns when no host)
  PVC / emptyDir mounts      -> resources[*] of type volume, containers[*].volumes
  ConfigMap mounts           -> containers[*].files with literal content
  Secret mounts / env        -> placeholders, each reported as a warning

Not represented (reported, never silently dropped):
  replicas, init containers, tcpSocket and grpc probes, non-workload kinds

Mutations: none. Every ConfigHub call this tool makes is a read.

Run it:
  k8s-to-score --space <space> --out-dir score/
  score-k8s init --no-sample && score-k8s generate score/*.yaml
`

func printExplain(cmd *cobra.Command) error {
	fmt.Fprint(cmd.OutOrStdout(), explainText)
	return nil
}

func printExplainJSON(cmd *cobra.Command) error {
	plan := map[string]any{
		"example_name":       "k8s-to-score",
		"mutates":            false,
		"mutates_confighub":  false,
		"mutates_live_infra": false,
		"reads": map[string]any{
			"confighub_space": opts.space,
			"where":           opts.where,
		},
		"writes": map[string]any{
			"out_dir": opts.outDir,
			"shape":   "one score.dev/v1b1 Workload file per Deployment or StatefulSet",
		},
		"score_api_version": "score.dev/v1b1",
		"mapping": map[string]string{
			"Deployment":            "workload",
			"StatefulSet":           "workload (k8s.score.dev/kind annotation)",
			"Service":               "service.ports",
			"Ingress":               "resources[*] type=route",
			"PersistentVolumeClaim": "resources[*] type=volume",
			"ConfigMap":             "containers[*].variables and containers[*].files",
			"Secret":                "placeholders, reported as warnings",
		},
		"not_represented": []string{
			"replicas", "initContainers", "tcpSocket probes", "grpc probes", "non-workload kinds",
		},
		"evaluation_modes": map[string]any{
			"fast_preview": map[string]any{
				"mutates":  false,
				"commands": []string{"k8s-to-score --explain", "k8s-to-score --explain-json | jq"},
			},
			"fast_operational_evaluation": map[string]any{
				"mutates_confighub":  false,
				"mutates_live_infra": false,
				"commands": []string{
					"k8s-to-score --from-dir testdata/sample --out-dir /tmp/score",
					"k8s-to-score --space <space> --out-dir score/ --report-json",
				},
			},
		},
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(plan)
}
