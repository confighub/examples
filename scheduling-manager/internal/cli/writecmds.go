// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/confighub/sdk/cliutil"

	"github.com/confighub/examples/scheduling-manager/internal/cub"
)

// podSpecPath is the pod-template spec path for the common workload controllers.
// (Pod and CronJob use different paths; those are handled via profiles.)
const podSpecPath = ".spec.template.spec"

// newSetNodeSelectorCmd sets the pod-template nodeSelector map on a workload.
func newSetNodeSelectorCmd() *cobra.Command {
	var output string
	var selectors []string
	var commit cliutil.CommitFlags
	cmd := &cobra.Command{
		Use:   "set-node-selector <space>/<unit>",
		Short: "Set the pod-template nodeSelector (which nodes the workload may land on)",
		Long: `set-node-selector replaces the workload's pod-template nodeSelector with the given
key=value labels, pinning it to matching nodes. Repeat --selector for multiple
labels.

  set-node-selector web-prod/web --selector pool=gpu --selector arch=arm64

Applies to Deployment / StatefulSet / DaemonSet / ReplicaSet / Job (pod template
at spec.template.spec). Dry-run unless --commit --change-desc; never bypasses
ApplyGates.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(selectors) == 0 {
				return fmt.Errorf("at least one --selector key=value is required")
			}
			pairs, err := parseKeyValues(selectors)
			if err != nil {
				return err
			}
			expr := fmt.Sprintf("%s.nodeSelector = {%s}", podSpecPath, mapLiteral(pairs))
			changeDesc, dryRun, err := commit.Validate(fmt.Sprintf("set nodeSelector on %s", args[0]))
			if err != nil {
				return err
			}
			return runYQ(cmd, "set-node-selector", args[0], expr, dryRun, changeDesc, output)
		},
	}
	addOutputFlag(cmd, &output)
	commit.Bind(cmd)
	cmd.Flags().StringArrayVar(&selectors, "selector", nil, "node label as key=value (repeatable)")
	return cmd
}

// newSetTolerationsCmd sets the pod-template tolerations on a workload.
func newSetTolerationsCmd() *cobra.Command {
	var output string
	var tolerations []string
	var commit cliutil.CommitFlags
	cmd := &cobra.Command{
		Use:   "set-tolerations <space>/<unit>",
		Short: "Set the pod-template tolerations (which node taints the workload tolerates)",
		Long: `set-tolerations replaces the workload's pod-template tolerations. Each --toleration
is "key[=value][:effect]":

  set-tolerations ml-prod/trainer --toleration nvidia.com/gpu:NoSchedule
  set-tolerations ml-prod/trainer --toleration dedicated=ml:NoSchedule

With a value the operator is Equal; without, it is Exists. Note: a toleration only
*permits* scheduling onto a tainted node — pair it with a nodeSelector or node
affinity to actually land there. Dry-run unless --commit --change-desc.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(tolerations) == 0 {
				return fmt.Errorf("at least one --toleration key[=value][:effect] is required")
			}
			objs, err := tolerationObjects(tolerations)
			if err != nil {
				return err
			}
			expr := fmt.Sprintf("%s.tolerations = [%s]", podSpecPath, strings.Join(objs, ", "))
			changeDesc, dryRun, err := commit.Validate(fmt.Sprintf("set tolerations on %s", args[0]))
			if err != nil {
				return err
			}
			return runYQ(cmd, "set-tolerations", args[0], expr, dryRun, changeDesc, output)
		},
	}
	addOutputFlag(cmd, &output)
	commit.Bind(cmd)
	cmd.Flags().StringArrayVar(&tolerations, "toleration", nil, "toleration as key[=value][:effect] (repeatable)")
	return cmd
}

// newSetNodeAffinityCmd sets a required node affinity term on a workload.
func newSetNodeAffinityCmd() *cobra.Command {
	var output string
	var required []string
	var commit cliutil.CommitFlags
	cmd := &cobra.Command{
		Use:   "set-node-affinity <space>/<unit>",
		Short: "Set a required node affinity term (harder placement than nodeSelector)",
		Long: `set-node-affinity sets a requiredDuringSchedulingIgnoredDuringExecution node
affinity term from --required "key=v1,v2" match expressions (operator In). Repeat
--required for multiple expressions (ANDed within one term).

  set-node-affinity web-prod/web --required "topology.kubernetes.io/zone=us-east-1a,us-east-1b"

Dry-run unless --commit --change-desc; never bypasses ApplyGates.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(required) == 0 {
				return fmt.Errorf("at least one --required key=v1,v2 is required")
			}
			exprs, err := matchExpressions(required)
			if err != nil {
				return err
			}
			expr := fmt.Sprintf("%s.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms = [{\"matchExpressions\": [%s]}]",
				podSpecPath, strings.Join(exprs, ", "))
			changeDesc, dryRun, err := commit.Validate(fmt.Sprintf("set node affinity on %s", args[0]))
			if err != nil {
				return err
			}
			return runYQ(cmd, "set-node-affinity", args[0], expr, dryRun, changeDesc, output)
		},
	}
	addOutputFlag(cmd, &output)
	commit.Bind(cmd)
	cmd.Flags().StringArrayVar(&required, "required", nil, "required match expression as key=v1,v2 (operator In; repeatable)")
	return cmd
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

// --- parsing + yq literal helpers ---

func parseKeyValues(pairs []string) (map[string]string, error) {
	out := map[string]string{}
	for _, p := range pairs {
		k, v, ok := strings.Cut(p, "=")
		if !ok || k == "" {
			return nil, fmt.Errorf("bad key=value %q", p)
		}
		out[k] = v
	}
	return out, nil
}

// mapLiteral renders a string map as a YAML flow map with sorted keys.
func mapLiteral(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%q: %q", k, m[k]))
	}
	return strings.Join(parts, ", ")
}

// tolerationObjects parses "key[=value][:effect]" specs into yq flow-map objects.
func tolerationObjects(specs []string) ([]string, error) {
	out := make([]string, 0, len(specs))
	for _, s := range specs {
		keyValue := s
		effect := ""
		if kv, e, ok := strings.Cut(s, ":"); ok {
			keyValue = kv
			effect = e
		}
		key, value, hasValue := strings.Cut(keyValue, "=")
		if key == "" {
			return nil, fmt.Errorf("bad --toleration %q (need a key)", s)
		}
		fields := []string{fmt.Sprintf("%q: %q", "key", key)}
		if hasValue {
			fields = append(fields, fmt.Sprintf("%q: %q", "operator", "Equal"), fmt.Sprintf("%q: %q", "value", value))
		} else {
			fields = append(fields, fmt.Sprintf("%q: %q", "operator", "Exists"))
		}
		if effect != "" {
			fields = append(fields, fmt.Sprintf("%q: %q", "effect", effect))
		}
		out = append(out, "{"+strings.Join(fields, ", ")+"}")
	}
	return out, nil
}

// matchExpressions parses "key=v1,v2" specs into In match-expression objects.
func matchExpressions(specs []string) ([]string, error) {
	out := make([]string, 0, len(specs))
	for _, s := range specs {
		key, vals, ok := strings.Cut(s, "=")
		if !ok || key == "" || vals == "" {
			return nil, fmt.Errorf("bad --required %q (want key=v1,v2)", s)
		}
		var quoted []string
		for _, v := range strings.Split(vals, ",") {
			v = strings.TrimSpace(v)
			if v != "" {
				quoted = append(quoted, fmt.Sprintf("%q", v))
			}
		}
		out = append(out, fmt.Sprintf("{%q: %q, %q: %q, %q: [%s]}",
			"key", key, "operator", "In", "values", strings.Join(quoted, ", ")))
	}
	return out, nil
}
