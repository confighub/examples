// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

// The workload profile library. The library machinery -- the Space, the stored
// Invocations, the parameter binding, and the install/list/apply and fleet-edit
// commands -- is managerkit/profiles. What is workload-specific is which
// profiles are seeded and what the help text calls them.

import (
	"fmt"

	"github.com/spf13/cobra"

	api "github.com/confighub/sdk/core/function/api"

	"github.com/confighub/examples/managerkit/profiles"
	"github.com/confighub/examples/workload-manager/internal/cub"
)

// workloadKindsWhereData scopes fleet-edit to pod-bearing workload Units, so a
// profile that operates on containers/pod templates never hits an unrelated Unit.
const workloadKindsWhereData = "kind IN ('Deployment', 'StatefulSet', 'DaemonSet', 'ReplicaSet', 'Job', 'CronJob', 'Pod')"

// terminationMsgYQ sets terminationMessagePolicy on every container of a
// workload's pod template.
const terminationMsgYQ = `.spec.template.spec.containers[].terminationMessagePolicy = "FallbackToLogsOnError"`

// antiAffinitySoftYQ adds preferred pod anti-affinity across nodes, using the
// pod-template labels as the selector.
const antiAffinitySoftYQ = `.spec.template.spec.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution = [{"weight": 100, "podAffinityTerm": {"topologyKey": "kubernetes.io/hostname", "labelSelector": {"matchLabels": .spec.template.metadata.labels}}}]`

// resTier is one resource tier: requests at cpu/mem, limits at twice that. The
// profile's own parameter namespace must be an identifier
// (^[A-Za-z_][A-Za-z0-9_]*$), so the profile param is "container" even though it
// binds the function's kebab-case "container-name" argument.
func resTier(slug, cpu, mem string) profiles.Spec {
	return profiles.Spec{
		Slug:        slug,
		Description: fmt.Sprintf("set-container-resources requests %s/%s, limits ×2", cpu, mem),
		Function:    "set-container-resources",
		Params:      []profiles.Param{{Name: "container"}},
		Args: []api.FunctionArgument{
			profiles.TmplArg("container-name", "container"),
			profiles.Arg("operation", "all"),
			profiles.Arg("cpu", cpu),
			profiles.Arg("memory", mem),
			profiles.Arg("limit-factor", 2),
		},
	}
}

// library is this tool's profile library.
var library = profiles.Library{
	Tool:           "workload-manager",
	Target:         "a workload",
	ParamExample:   "--param container=web for the resource tiers",
	Preflight:      cub.Preflight,
	InvocationName: InvocationName,
	FleetEdit: profiles.FleetEdit{
		WhereData: workloadKindsWhereData,
		Scope:     "pod-bearing workload kinds",
		Example:   `fleet-edit --profile harden-restricted --environment prod --commit --change-desc "…"`,
	},
	Profiles: []profiles.Spec{
		resTier("resources-small", "100m", "128Mi"),
		resTier("resources-medium", "250m", "256Mi"),
		resTier("resources-large", "500m", "512Mi"),
		{
			Slug:        "harden-restricted",
			Description: "set-pod-container-security-context-defaults (runAsNonRoot, seccomp, drop ALL, readOnlyRootFilesystem)",
			Function:    "set-pod-container-security-context-defaults",
		},
		{
			Slug:        "probes-http",
			Description: "set-container-probe-defaults (HTTP liveness/readiness/startup on the first port)",
			Function:    "set-container-probe-defaults",
		},
		{
			Slug:        "anti-affinity-soft",
			Description: "set-yq: preferred pod anti-affinity across nodes (selector from pod-template labels)",
			Function:    "set-yq",
			Args:        []api.FunctionArgument{profiles.Arg("yq-expression", antiAffinitySoftYQ)},
		},
		{
			Slug:        "termination-message-policy",
			Description: "set-yq: terminationMessagePolicy: FallbackToLogsOnError on all containers",
			Function:    "set-yq",
			Args:        []api.FunctionArgument{profiles.Arg("yq-expression", terminationMsgYQ)},
		},
	},
}

func newProfileCmd() *cobra.Command   { return library.Command() }
func newFleetEditCmd() *cobra.Command { return library.FleetEditCommand() }
