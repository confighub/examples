// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

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
	if err := cub.Preflight(ctx); err != nil {
		return err
	}
	out := cmd.OutOrStdout()

	if _, err := cub.Run(ctx, "space", "create", rbac.EditLibrarySpace,
		"--label", "app=rbac-manager", "--label", "role=edits", "--allow-exists"); err != nil {
		return fmt.Errorf("create edit library space: %w", err)
	}
	fprintln(out, "Space "+rbac.EditLibrarySpace+" ready")

	for _, spec := range rbac.EditInvocationSpecs {
		if _, err := cub.Run(ctx, invocationCreateArgs(spec)...); err != nil {
			return fmt.Errorf("create invocation %s: %w", spec.Slug, err)
		}
		fprintln(out, "Invocation "+rbac.EditLibrarySpace+"/"+spec.Slug+" ready")
	}
	fprintln(out, "\nEdit Invocations installed. `cub-rbac edit` / `fleet-edit` will use them.")
	return nil
}

// invocationCreateArgs builds the `cub invocation create` argv for one edit spec:
// declare each parameter, then store a set-yq call whose `param` argument values
// are templated from those parameters ({{ .Params.<name> }}).
func invocationCreateArgs(spec rbac.EditInvocationSpec) []string {
	args := []string{"invocation", "create", "--space", rbac.EditLibrarySpace, "--allow-exists",
		spec.Slug, "Kubernetes/YAML"}
	for _, p := range spec.Parameters {
		decl := p.Name
		if p.DataType == "int" {
			decl = p.Name + ":int"
		}
		args = append(args, "--parameter", decl)
	}
	args = append(args, "--", "set-yq", "--yq-expression="+spec.YQExpression)
	for _, p := range spec.Parameters {
		args = append(args, "--param=template:"+p.Name+"={{ .Params."+p.Name+" }}")
	}
	return args
}
