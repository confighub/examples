// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

// The observability profile library. The library machinery -- the Space, the
// stored Invocations, the parameter binding, and the install/list/apply and
// fleet-edit commands -- is managerkit/profiles. What is observability-specific
// is which profiles are seeded and what the help text calls them.

import (
	"github.com/spf13/cobra"

	api "github.com/confighub/sdk/core/function/api"

	"github.com/confighub/examples/managerkit/profiles"
	"github.com/confighub/examples/observability-manager/internal/cub"
)

// workloadKindsWhereData scopes fleet-edit to pod-bearing workload Units, so a
// profile that operates on containers never reaches an unrelated Unit.
const workloadKindsWhereData = "kind IN ('Deployment', 'StatefulSet', 'DaemonSet', 'ReplicaSet', 'Job', 'CronJob', 'Pod')"

// otelSidecarValue is the sidecar container set-path writes; the container name
// is injected from the path's merge key, and the image is a profile parameter.
const otelSidecarValue = "image: {{ .Params.image }}\nports:\n- name: otlp-grpc\n  containerPort: 4317\n"

// library is this tool's profile library.
var library = profiles.Library{
	Tool:           "observability-manager",
	Target:         "a workload",
	ParamExample:   "--param image=otel/opentelemetry-collector:0.100 for otel-sidecar",
	Preflight:      cub.Preflight,
	InvocationName: InvocationName,
	FleetEdit: profiles.FleetEdit{
		WhereData: workloadKindsWhereData,
		Scope:     "pod-bearing workload kinds",
		Example:   `fleet-edit --profile otel-sidecar --environment prod --param image=otel/opentelemetry-collector:0.100 --commit --change-desc "…"`,
	},
	Profiles: []profiles.Spec{
		{
			Slug:        "otel-sidecar",
			Description: "set-path: inject/replace an otel-collector sidecar container (param: image)",
			Function:    "set-path",
			Args: []api.FunctionArgument{
				profiles.Arg("path", "spec.template.spec.containers.?name=otel-collector"),
				{ParameterName: "value", Value: otelSidecarValue, Evaluator: api.EvaluatorTemplate},
			},
			Params: []profiles.Param{{Name: "image"}},
		},
	},
}

func newProfileCmd() *cobra.Command   { return library.Command() }
func newFleetEditCmd() *cobra.Command { return library.FleetEditCommand() }
