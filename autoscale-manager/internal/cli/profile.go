// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

// The autoscaling profile library. The library machinery -- the Space, the
// stored Invocations, the parameter binding, and the install/list/apply and
// fleet-edit commands -- is managerkit/profiles. What is autoscaling-specific is
// which profiles are seeded and what the help text calls them.

import (
	"fmt"

	"github.com/spf13/cobra"

	api "github.com/confighub/sdk/core/function/api"

	"github.com/confighub/examples/autoscale-manager/internal/cub"
	"github.com/confighub/examples/managerkit/profiles"
)

// autoscalerKindsWhereData scopes fleet-edit to HPA / ScaledObject resources.
const autoscalerKindsWhereData = "kind IN ('HorizontalPodAutoscaler', 'ScaledObject')"

// cpuMetricYQ replaces spec.metrics with a single cpu Utilization target at pct.
func cpuMetricYQ(pct int) string {
	return fmt.Sprintf(`.spec.metrics = [{"type": "Resource", "resource": {"name": "cpu", "target": {"type": "Utilization", "averageUtilization": %d}}}]`, pct)
}

const hpaRangeYQ = `.spec.minReplicas = ($params.min | tonumber) | .spec.maxReplicas = ($params.max | tonumber)`

// library is this tool's profile library.
var library = profiles.Library{
	Tool:           "autoscale-manager",
	Domain:         "autoscale.confighub.com",
	Noun:           "autoscaling profile",
	Target:         "an HPA Unit",
	ParamExample:   "--param min=3 --param max=10 for hpa-range",
	Preflight:      cub.Preflight,
	InvocationName: InvocationName,
	FleetEdit: profiles.FleetEdit{
		WhereData: autoscalerKindsWhereData,
		Scope:     "HorizontalPodAutoscaler / ScaledObject Units",
		Example:   `fleet-edit --profile hpa-conservative --environment prod --commit --change-desc "…"`,
	},
	Profiles: []profiles.Spec{
		{
			Slug:        "hpa-conservative",
			Description: "set-yq: scale out early — cpu target 60% average Utilization (more headroom)",
			Function:    "set-yq",
			Args:        []api.FunctionArgument{profiles.Arg("yq-expression", cpuMetricYQ(60))},
		},
		{
			Slug:        "hpa-aggressive",
			Description: "set-yq: pack tighter — cpu target 85% average Utilization (fewer replicas)",
			Function:    "set-yq",
			Args:        []api.FunctionArgument{profiles.Arg("yq-expression", cpuMetricYQ(85))},
		},
		{
			Slug:        "hpa-range",
			Description: "set-yq: set minReplicas/maxReplicas (params: min, max)",
			Function:    "set-yq",
			Args: []api.FunctionArgument{
				profiles.Arg("yq-expression", hpaRangeYQ),
				profiles.YQParamArg("min"),
				profiles.YQParamArg("max"),
			},
			Params: []profiles.Param{{Name: "min"}, {Name: "max"}},
		},
	},
}

func newProfileCmd() *cobra.Command   { return library.Command() }
func newFleetEditCmd() *cobra.Command { return library.FleetEditCommand() }
