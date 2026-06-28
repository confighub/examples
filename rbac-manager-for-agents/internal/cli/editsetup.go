// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/confighub/sdk/core/cubapi"
	api "github.com/confighub/sdk/core/function/api"
	goclientnew "github.com/confighub/sdk/core/openapi/goclient-new"

	"github.com/confighub/examples/rbac-manager-for-agents/internal/cub"
	"github.com/confighub/examples/rbac-manager-for-agents/internal/rbac"
)

// newEditInstallCmd creates the shared edit Invocations (parameterized set-yq)
// that the edit / fleet-edit commands invoke. Run once per organization, like
// installing the guardrail Triggers; idempotent via --allow-exists.
func newEditInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Create the shared, parameterized edit Invocations (run once per org)",
		Long: `install creates the Space "` + rbac.EditLibrarySpace + `" and the parameterized
set-yq Invocations that the edit and fleet-edit commands invoke
(` + rbac.InvAddVerb + `, ` + rbac.InvRemoveVerb + `, ` + rbac.InvAddSubject + `,
` + rbac.InvRemoveSubject + `). The fixed yq templates live in these Invocations;
edits supply only the variable values as parameters. Idempotent — safe to re-run.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEditInstall(cmd)
		},
	}
}

func runEditInstall(cmd *cobra.Command) error {
	ctx := cmd.Context()
	client, err := cub.Preflight(ctx)
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()

	lib, err := cubapi.EnsureSpace(ctx, client, goclientnew.Space{
		Slug:   rbac.EditLibrarySpace,
		Labels: map[string]string{"app": "rbac-manager", "role": "edits"},
	})
	if err != nil {
		return fmt.Errorf("create edit library space: %w", err)
	}
	fprintln(out, "Space "+rbac.EditLibrarySpace+" ready")

	for _, spec := range rbac.EditInvocationSpecs {
		if _, err := cubapi.EnsureInvocation(ctx, client, invocationSpec(lib.SpaceID, spec)); err != nil {
			return fmt.Errorf("create invocation %s: %w", spec.Slug, err)
		}
		fprintln(out, "Invocation "+rbac.EditLibrarySpace+"/"+spec.Slug+" ready")
	}
	fprintln(out, "\nEdit Invocations installed. `cub-rbac edit` / `fleet-edit` will use them.")
	return nil
}

// invocationSpec turns an edit spec into the stored set-yq Invocation: declare
// each parameter, then bind set-yq's `param` arguments to those parameters with
// templated values ({{ .Params.<name> }}).
func invocationSpec(spaceID goclientnew.UUID, spec rbac.EditInvocationSpec) goclientnew.Invocation {
	args := []api.FunctionArgument{{ParameterName: "yq-expression", Value: spec.YQExpression}}
	params := make([]goclientnew.FunctionParameter, 0, len(spec.Parameters))
	for _, p := range spec.Parameters {
		dataType := p.DataType
		if dataType == "" {
			dataType = "string"
		}
		params = append(params, goclientnew.FunctionParameter{
			ParameterName: p.Name,
			DataType:      dataType,
			Required:      true,
		})
		args = append(args, api.FunctionArgument{
			ParameterName: "param",
			Value:         p.Name + "={{ .Params." + p.Name + " }}",
			Evaluator:     api.EvaluatorTemplate,
		})
	}
	return goclientnew.Invocation{
		SpaceID:       spaceID,
		Slug:          spec.Slug,
		ToolchainType: "Kubernetes/YAML",
		FunctionName:  "set-yq",
		Arguments:     cubapi.Arguments(args),
		Parameters:    params,
	}
}
