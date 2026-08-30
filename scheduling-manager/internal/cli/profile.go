// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

// The placement profile library. The library machinery -- the Space, the stored
// Invocations, the parameter binding, and the install/list/apply and fleet-edit
// commands -- is managerkit/profiles. What is scheduling-specific is which
// profiles are seeded and what the help text calls them.

import (
	"github.com/spf13/cobra"

	api "github.com/confighub/sdk/core/function/api"

	"github.com/confighub/examples/managerkit/profiles"
	"github.com/confighub/examples/scheduling-manager/internal/cub"
)

// workloadKindsWhereData scopes fleet-edit to pod-bearing workload Units, so a
// placement profile never reaches a Unit with no pod template to place.
const workloadKindsWhereData = "kind IN ('Deployment', 'StatefulSet', 'DaemonSet', 'ReplicaSet', 'Job', 'CronJob', 'Pod')"

const gpuYQ = `.spec.template.spec.nodeSelector.pool = "gpu" | .spec.template.spec.tolerations = [{"key": "nvidia.com/gpu", "operator": "Exists", "effect": "NoSchedule"}]`
const spotYQ = `.spec.template.spec.nodeSelector.pool = "spot" | .spec.template.spec.tolerations = [{"key": "spot", "operator": "Equal", "value": "true", "effect": "NoSchedule"}]`
const nodePoolYQ = `.spec.template.spec.nodeSelector.pool = $params.pool`

// library is this tool's profile library.
var library = profiles.Library{
	Tool:           "scheduling-manager",
	Noun:           "placement profile",
	Target:         "a workload",
	ParamExample:   "--param pool=gpu for node-pool",
	Preflight:      cub.Preflight,
	InvocationName: InvocationName,
	FleetEdit: profiles.FleetEdit{
		WhereData: workloadKindsWhereData,
		Scope:     "pod-bearing workload kinds",
		Example:   `fleet-edit --profile placement-gpu --environment prod --component ml --commit --change-desc "…"`,
	},
	Profiles: []profiles.Spec{
		{
			Slug:        "placement-gpu",
			Description: "set-yq: pin to the gpu node pool + tolerate the nvidia.com/gpu taint",
			Function:    "set-yq",
			Args:        []api.FunctionArgument{profiles.Arg("yq-expression", gpuYQ)},
		},
		{
			Slug:        "placement-spot",
			Description: "set-yq: pin to the spot node pool + tolerate the spot taint",
			Function:    "set-yq",
			Args:        []api.FunctionArgument{profiles.Arg("yq-expression", spotYQ)},
		},
		{
			Slug:        "node-pool",
			Description: "set-yq: pin nodeSelector.pool to the given pool (param: pool)",
			Function:    "set-yq",
			Args: []api.FunctionArgument{
				profiles.Arg("yq-expression", nodePoolYQ),
				profiles.YQParamArg("pool"),
			},
			Params: []profiles.Param{{Name: "pool"}},
		},
	},
}

func newProfileCmd() *cobra.Command   { return library.Command() }
func newFleetEditCmd() *cobra.Command { return library.FleetEditCommand() }
