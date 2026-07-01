// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/confighub/sdk/cliutil"
	"github.com/confighub/sdk/core/cubapi"
	api "github.com/confighub/sdk/core/function/api"

	"github.com/confighub/examples/workload-manager/internal/cub"
)

// changeOf turns the commit flags' dry-run/description into a cubapi.Change
// (empty Description = dry-run).
func changeOf(changeDesc string, dryRun bool) cubapi.Change {
	if dryRun {
		return cubapi.Change{}
	}
	return cubapi.Change{Description: changeDesc}
}

// newHardenCmd applies the security-context and automount defaults to a workload.
func newHardenCmd() *cobra.Command {
	var output string
	var commit cliutil.CommitFlags
	cmd := &cobra.Command{
		Use:   "harden <space>/<unit>",
		Short: "Apply pod/container security-context defaults and disable SA-token automount",
		Long: `harden runs the hermetic, idempotent security defaulting functions over a
workload Unit:

  - set-pod-container-security-context-defaults (runAsNonRoot, seccomp
    RuntimeDefault, drop ALL, readOnlyRootFilesystem, no privilege escalation)
  - set-automount-service-account-token-false

The Unit is edited, not applied — rolling it out is a separate 'cub unit apply'.
Dry-run unless --commit --change-desc; never bypasses ApplyGates.

Note: if a container legitimately needs a writable root filesystem or the
ServiceAccount token, record an exception rather than hardening it blindly.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			changeDesc, dryRun, err := commit.Validate(fmt.Sprintf("harden workload %s", args[0]))
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
			ch := changeOf(changeDesc, dryRun)
			sec, err := cub.InvokeMutation(cmd.Context(), client, "set-pod-container-security-context-defaults", nil, ref.selector(), ch)
			if err != nil {
				return err
			}
			automount, err := cub.InvokeMutation(cmd.Context(), client, "set-automount-service-account-token-false", nil, ref.selector(), ch)
			if err != nil {
				return err
			}
			return reportMutation(cmd, "harden", ref.spaceSlug, dryRun, output, sec, automount)
		},
	}
	addOutputFlag(cmd, &output)
	commit.Bind(cmd)
	return cmd
}

// resourceTier maps a named tier to (cpu, memory) requests.
var resourceTiers = map[string]struct{ cpu, memory string }{
	"small":  {"100m", "128Mi"},
	"medium": {"250m", "256Mi"},
	"large":  {"500m", "512Mi"},
}

// newSetResourcesCmd sets container requests/limits via set-container-resources.
func newSetResourcesCmd() *cobra.Command {
	var output, container, tier, cpu, memory string
	var limitFactor int
	var commit cliutil.CommitFlags
	cmd := &cobra.Command{
		Use:   "set-resources <space>/<unit>",
		Short: "Set container CPU/memory requests and limits (by tier or explicit values)",
		Long: `set-resources runs set-container-resources over a workload Unit, setting cpu and
memory requests and deriving limits as requests × --limit-factor.

Pick a --tier (small=100m/128Mi, medium=250m/256Mi, large=500m/512Mi) or give
explicit --cpu and --memory. --container selects the container by name (default
'*' = all). Operation is 'all' (set unconditionally).

Dry-run unless --commit --change-desc; never bypasses ApplyGates.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if tier != "" {
				t, ok := resourceTiers[tier]
				if !ok {
					return fmt.Errorf("unknown --tier %q (want small | medium | large)", tier)
				}
				cpu, memory = t.cpu, t.memory
			}
			if cpu == "" || memory == "" {
				return fmt.Errorf("provide --tier, or both --cpu and --memory")
			}
			changeDesc, dryRun, err := commit.Validate(fmt.Sprintf("set resources on %s (%s/%s, limit-factor %d)", args[0], cpu, memory, limitFactor))
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
			resArgs := []api.FunctionArgument{
				{ParameterName: "container-name", Value: container},
				{ParameterName: "operation", Value: "all"},
				{ParameterName: "cpu", Value: cpu},
				{ParameterName: "memory", Value: memory},
				{ParameterName: "limit-factor", Value: limitFactor},
			}
			res, err := cub.InvokeMutation(cmd.Context(), client, "set-container-resources", resArgs, ref.selector(), changeOf(changeDesc, dryRun))
			if err != nil {
				return err
			}
			return reportMutation(cmd, "set-resources", ref.spaceSlug, dryRun, output, res)
		},
	}
	addOutputFlag(cmd, &output)
	commit.Bind(cmd)
	cmd.Flags().StringVar(&container, "container", "*", "container name to set (default '*' = all)")
	cmd.Flags().StringVar(&tier, "tier", "", "resource tier: small | medium | large")
	cmd.Flags().StringVar(&cpu, "cpu", "", "cpu request quantity (e.g. 250m); overrides --tier")
	cmd.Flags().StringVar(&memory, "memory", "", "memory request quantity (e.g. 256Mi); overrides --tier")
	cmd.Flags().IntVar(&limitFactor, "limit-factor", 2, "limit = request × this factor (0 = no limits)")
	return cmd
}

// newSetProbesCmd adds probe defaults via set-container-probe-defaults.
func newSetProbesCmd() *cobra.Command {
	var output, livenessPath, readinessPath, startupPath string
	var commit cliutil.CommitFlags
	cmd := &cobra.Command{
		Use:   "set-probes <space>/<unit>",
		Short: "Add liveness/readiness/startup probe defaults to containers missing them",
		Long: `set-probes runs set-container-probe-defaults over a workload Unit, adding HTTP GET
liveness, readiness, and startup probes (on the first container port) to
containers that don't already have them.

Override the probe paths with --liveness-path / --readiness-path / --startup-path
(each defaults to /healthz). Dry-run unless --commit --change-desc; never bypasses
ApplyGates.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			changeDesc, dryRun, err := commit.Validate(fmt.Sprintf("add probe defaults to %s", args[0]))
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
			var fnArgs []api.FunctionArgument
			addArg := func(name, val string) {
				if val != "" {
					fnArgs = append(fnArgs, api.FunctionArgument{ParameterName: name, Value: val})
				}
			}
			addArg("liveness-path", livenessPath)
			addArg("readiness-path", readinessPath)
			addArg("startup-path", startupPath)
			res, err := cub.InvokeMutation(cmd.Context(), client, "set-container-probe-defaults", fnArgs, ref.selector(), changeOf(changeDesc, dryRun))
			if err != nil {
				return err
			}
			return reportMutation(cmd, "set-probes", ref.spaceSlug, dryRun, output, res)
		},
	}
	addOutputFlag(cmd, &output)
	commit.Bind(cmd)
	cmd.Flags().StringVar(&livenessPath, "liveness-path", "", "HTTP path for the liveness probe (default /healthz)")
	cmd.Flags().StringVar(&readinessPath, "readiness-path", "", "HTTP path for the readiness probe (default /healthz)")
	cmd.Flags().StringVar(&startupPath, "startup-path", "", "HTTP path for the startup probe (default /healthz)")
	return cmd
}

// newEnsureSpreadCmd adds pod anti-affinity or a topology spread constraint via
// set-yq, using the pod-template labels as the selector.
func newEnsureSpreadCmd() *cobra.Command {
	var output, antiAffinity, topologyKey string
	var topologySpread bool
	var commit cliutil.CommitFlags
	cmd := &cobra.Command{
		Use:   "ensure-spread <space>/<unit>",
		Short: "Add pod anti-affinity or a topology spread constraint to a workload",
		Long: `ensure-spread edits a workload Unit (Deployment / StatefulSet / ReplicaSet) so a
node/zone loss can't take every replica, using the pod-template labels as the
selector. Choose one:

  --anti-affinity soft   preferredDuringScheduling pod anti-affinity (default)
  --anti-affinity hard   requiredDuringScheduling pod anti-affinity
  --topology-spread      a topologySpreadConstraints entry (maxSkew 1, ScheduleAnyway)

--topology-key sets the spread domain (default kubernetes.io/hostname; use
topology.kubernetes.io/zone to spread across zones).

Prefer 'soft' — a 'hard' anti-affinity can leave replicas Pending when the
cluster has fewer eligible nodes than replicas. The edit is idempotent (it sets,
not appends). Dry-run unless --commit --change-desc; never bypasses ApplyGates.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if topologyKey == "" {
				topologyKey = "kubernetes.io/hostname"
			}
			var yqExpr string
			switch {
			case topologySpread:
				yqExpr = fmt.Sprintf(`.spec.template.spec.topologySpreadConstraints = [{"maxSkew": 1, "topologyKey": %q, "whenUnsatisfiable": "ScheduleAnyway", "labelSelector": {"matchLabels": .spec.template.metadata.labels}}]`, topologyKey)
			case antiAffinity == "hard":
				yqExpr = fmt.Sprintf(`.spec.template.spec.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution = [{"topologyKey": %q, "labelSelector": {"matchLabels": .spec.template.metadata.labels}}]`, topologyKey)
			case antiAffinity == "" || antiAffinity == "soft":
				yqExpr = fmt.Sprintf(`.spec.template.spec.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution = [{"weight": 100, "podAffinityTerm": {"topologyKey": %q, "labelSelector": {"matchLabels": .spec.template.metadata.labels}}}]`, topologyKey)
			default:
				return fmt.Errorf("unknown --anti-affinity %q (want soft | hard)", antiAffinity)
			}
			changeDesc, dryRun, err := commit.Validate(fmt.Sprintf("ensure spread on %s", args[0]))
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
			res, err := cub.MutateUnitYQ(cmd.Context(), client, yqExpr, ref.selector(), changeOf(changeDesc, dryRun))
			if err != nil {
				return err
			}
			return reportMutation(cmd, "ensure-spread", ref.spaceSlug, dryRun, output, res)
		},
	}
	addOutputFlag(cmd, &output)
	commit.Bind(cmd)
	cmd.Flags().StringVar(&antiAffinity, "anti-affinity", "soft", "pod anti-affinity strength: soft | hard")
	cmd.Flags().BoolVar(&topologySpread, "topology-spread", false, "add a topologySpreadConstraints entry instead of anti-affinity")
	cmd.Flags().StringVar(&topologyKey, "topology-key", "", "topology domain key (default kubernetes.io/hostname)")
	return cmd
}
