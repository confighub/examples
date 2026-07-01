// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/confighub/sdk/cliutil"
	"github.com/confighub/sdk/core/cubapi"
	api "github.com/confighub/sdk/core/function/api"
	goclientnew "github.com/confighub/sdk/core/openapi/goclient-new"

	"github.com/confighub/examples/scheduling-manager/internal/cub"
)

// profilesSpace holds the parameterized placement profiles (stored Invocations).
const profilesSpace = "scheduling-profiles"

type profileParam struct {
	name     string
	dataType string
}

type profileSpec struct {
	slug        string
	description string
	function    string
	args        []api.FunctionArgument
	params      []profileParam
}

func arg(fnParam string, value any) api.FunctionArgument {
	return api.FunctionArgument{ParameterName: fnParam, Value: value}
}

// yqParamArg binds set-yq's `param` varArg to a profile parameter via a template
// ref, so the yq expression can read $params.<name>.
func yqParamArg(name string) api.FunctionArgument {
	return api.FunctionArgument{ParameterName: "param", Value: name + "={{ .Params." + name + " }}", Evaluator: api.EvaluatorTemplate}
}

const gpuYQ = `.spec.template.spec.nodeSelector.pool = "gpu" | .spec.template.spec.tolerations = [{"key": "nvidia.com/gpu", "operator": "Exists", "effect": "NoSchedule"}]`
const spotYQ = `.spec.template.spec.nodeSelector.pool = "spot" | .spec.template.spec.tolerations = [{"key": "spot", "operator": "Equal", "value": "true", "effect": "NoSchedule"}]`
const nodePoolYQ = `.spec.template.spec.nodeSelector.pool = $params.pool`

func defaultProfiles() []profileSpec {
	return []profileSpec{
		{
			slug:        "placement-gpu",
			description: "set-yq: pin to the gpu node pool + tolerate the nvidia.com/gpu taint",
			function:    "set-yq",
			args:        []api.FunctionArgument{arg("yq-expression", gpuYQ)},
		},
		{
			slug:        "placement-spot",
			description: "set-yq: pin to the spot node pool + tolerate the spot taint",
			function:    "set-yq",
			args:        []api.FunctionArgument{arg("yq-expression", spotYQ)},
		},
		{
			slug:        "node-pool",
			description: "set-yq: pin nodeSelector.pool to the given pool (param: pool)",
			function:    "set-yq",
			args:        []api.FunctionArgument{arg("yq-expression", nodePoolYQ), yqParamArg("pool")},
			params:      []profileParam{{name: "pool"}},
		},
	}
}

func newProfileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "The placement profile library — reusable, parameterized edits (stored Invocations)",
		Long: `profile manages the scheduling-profiles Space: named placement edits stored as
ConfigHub Invocations (e.g. placement-gpu, placement-spot, node-pool).
'profile apply' invokes one over a workload, dry-run by default.`,
	}
	cmd.AddCommand(newProfileInstallCmd(), newProfileListCmd(), newProfileApplyCmd())
	return cmd
}

func newProfileInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Create the scheduling-profiles Space and seed the default placement profiles",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := cub.Preflight(ctx)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			lib, err := cubapi.EnsureSpace(ctx, client, goclientnew.Space{
				Slug:   profilesSpace,
				Labels: map[string]string{"app": "scheduling-manager", "role": "profiles"},
			})
			if err != nil {
				return fmt.Errorf("create profiles space: %w", err)
			}
			fprintln(out, "Space "+profilesSpace+" ready")
			for _, spec := range defaultProfiles() {
				if _, err := cubapi.EnsureInvocation(ctx, client, buildInvocation(lib.SpaceID, spec)); err != nil {
					return fmt.Errorf("create profile %s: %w", spec.slug, err)
				}
				fprintln(out, "Profile "+profilesSpace+"/"+spec.slug+" ready")
			}
			fprintln(out, "\nProfiles installed. Apply one with: "+InvocationName()+" profile apply <slug> <space>/<unit>")
			return nil
		},
	}
}

func buildInvocation(spaceID goclientnew.UUID, spec profileSpec) goclientnew.Invocation {
	params := make([]goclientnew.FunctionParameter, 0, len(spec.params))
	for _, p := range spec.params {
		dt := p.dataType
		if dt == "" {
			dt = "string"
		}
		params = append(params, goclientnew.FunctionParameter{ParameterName: p.name, DataType: dt, Required: true})
	}
	return goclientnew.Invocation{
		SpaceID:       spaceID,
		Slug:          spec.slug,
		DisplayName:   spec.slug,
		ToolchainType: "Kubernetes/YAML",
		FunctionName:  spec.function,
		Arguments:     cubapi.Arguments(spec.args),
		Parameters:    params,
		Annotations:   map[string]string{"scheduling.confighub.com/description": spec.description},
	}
}

func newProfileListCmd() *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the placement profiles in the scheduling-profiles Space",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := cub.Preflight(ctx)
			if err != nil {
				return err
			}
			sp, err := cubapi.ResolveSpace(ctx, client, profilesSpace)
			if err != nil {
				return fmt.Errorf("resolve %s (run `%s profile install`): %w", profilesSpace, InvocationName(), err)
			}
			invs, err := cub.ListInvocations(ctx, client, sp.SpaceID)
			if err != nil {
				return err
			}
			type row struct {
				Slug        string   `json:"slug"`
				Function    string   `json:"function"`
				Parameters  []string `json:"parameters,omitempty"`
				Description string   `json:"description,omitempty"`
			}
			rows := make([]row, 0, len(invs))
			for _, inv := range invs {
				r := row{Slug: inv.Slug, Function: inv.FunctionName}
				for _, p := range inv.Parameters {
					r.Parameters = append(r.Parameters, p.ParameterName)
				}
				if inv.Annotations != nil {
					r.Description = inv.Annotations["scheduling.confighub.com/description"]
				}
				rows = append(rows, r)
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].Slug < rows[j].Slug })
			if output == outputTable {
				tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
				fmt.Fprintln(tw, "PROFILE\tFUNCTION\tPARAMS\tDESCRIPTION")
				for _, r := range rows {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", r.Slug, r.Function, dash(strings.Join(r.Parameters, ",")), r.Description)
				}
				_ = tw.Flush()
				return nil
			}
			return printJSON(cmd.OutOrStdout(), rows)
		},
	}
	addOutputFlag(cmd, &output)
	return cmd
}

func newProfileApplyCmd() *cobra.Command {
	var output string
	var params []string
	var commit cliutil.CommitFlags
	cmd := &cobra.Command{
		Use:   "apply <profile> <space>/<unit>",
		Short: "Apply a placement profile to a workload (dry-run unless --commit)",
		Long: `apply invokes a stored placement profile over a workload Unit. Supply profile
parameters with --param name=value (e.g. --param pool=gpu for node-pool).

Dry-run unless --commit --change-desc; never bypasses ApplyGates.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			profileSlug := args[0]
			paramMap, err := parseParams(params)
			if err != nil {
				return err
			}
			changeDesc, dryRun, err := commit.Validate(fmt.Sprintf("apply placement profile %s to %s", profileSlug, args[1]))
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			client, err := cub.Preflight(ctx)
			if err != nil {
				return err
			}
			lib, err := cubapi.ResolveSpace(ctx, client, profilesSpace)
			if err != nil {
				return fmt.Errorf("resolve %s (run `%s profile install`): %w", profilesSpace, InvocationName(), err)
			}
			inv, err := cubapi.ResolveInvocation(ctx, client, lib.SpaceID, profileSlug)
			if err != nil {
				return fmt.Errorf("resolve profile %q: %w", profileSlug, err)
			}
			ref, err := parseUnitRef(ctx, client, args[1])
			if err != nil {
				return err
			}
			res, err := cubapi.InvokeStoredInvocation(ctx, client, inv.InvocationID, paramMap, ref.selector(), changeOf(changeDesc, dryRun))
			if err != nil {
				return err
			}
			return reportMutation(cmd, "profile apply "+profileSlug, ref.spaceSlug, dryRun, output, res)
		},
	}
	addOutputFlag(cmd, &output)
	commit.Bind(cmd)
	cmd.Flags().StringArrayVar(&params, "param", nil, "profile parameter as name=value (repeatable)")
	return cmd
}

func parseParams(params []string) (map[string]any, error) {
	out := map[string]any{}
	for _, p := range params {
		k, v, ok := strings.Cut(p, "=")
		if !ok || k == "" {
			return nil, fmt.Errorf("bad --param %q, want name=value", p)
		}
		out[k] = v
	}
	return out, nil
}
