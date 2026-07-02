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

	"github.com/confighub/examples/autoscale-manager/internal/cub"
)

// profilesSpace holds the parameterized autoscaling profiles (stored Invocations).
const profilesSpace = "autoscale-profiles"

const profileDescAnnotation = "autoscale.confighub.com/description"

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

// cpuMetricYQ replaces spec.metrics with a single cpu Utilization target at pct.
func cpuMetricYQ(pct int) string {
	return fmt.Sprintf(`.spec.metrics = [{"type": "Resource", "resource": {"name": "cpu", "target": {"type": "Utilization", "averageUtilization": %d}}}]`, pct)
}

const hpaRangeYQ = `.spec.minReplicas = ($params.min | tonumber) | .spec.maxReplicas = ($params.max | tonumber)`

func defaultProfiles() []profileSpec {
	return []profileSpec{
		{
			slug:        "hpa-conservative",
			description: "set-yq: scale out early — cpu target 60% average Utilization (more headroom)",
			function:    "set-yq",
			args:        []api.FunctionArgument{arg("yq-expression", cpuMetricYQ(60))},
		},
		{
			slug:        "hpa-aggressive",
			description: "set-yq: pack tighter — cpu target 85% average Utilization (fewer replicas)",
			function:    "set-yq",
			args:        []api.FunctionArgument{arg("yq-expression", cpuMetricYQ(85))},
		},
		{
			slug:        "hpa-range",
			description: "set-yq: set minReplicas/maxReplicas (params: min, max)",
			function:    "set-yq",
			args:        []api.FunctionArgument{arg("yq-expression", hpaRangeYQ), yqParamArg("min"), yqParamArg("max")},
			params:      []profileParam{{name: "min"}, {name: "max"}},
		},
	}
}

func newProfileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "The autoscaling profile library — reusable, parameterized edits (stored Invocations)",
		Long: `profile manages the autoscale-profiles Space: named autoscaling edits stored as
ConfigHub Invocations (e.g. hpa-conservative, hpa-aggressive, hpa-range).
'profile apply' invokes one over an HPA Unit, dry-run by default.`,
	}
	cmd.AddCommand(newProfileInstallCmd(), newProfileListCmd(), newProfileApplyCmd())
	return cmd
}

func newProfileInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Create the autoscale-profiles Space and seed the default profiles",
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
				Labels: map[string]string{"app": "autoscale-manager", "role": "profiles"},
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
		Annotations:   map[string]string{profileDescAnnotation: spec.description},
	}
}

func newProfileListCmd() *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the autoscaling profiles in the autoscale-profiles Space",
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
			invs, err := cubapi.ListInvocations(ctx, client,
				cubapi.NewWhere(fmt.Sprintf("SpaceID = '%s'", sp.SpaceID.String())),
				cubapi.ListOpts{Select: "Slug,FunctionName,Parameters,Annotations"})
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
			for _, ei := range invs {
				if ei.Invocation == nil {
					continue
				}
				inv := ei.Invocation
				r := row{Slug: inv.Slug, Function: inv.FunctionName}
				for _, p := range inv.Parameters {
					r.Parameters = append(r.Parameters, p.ParameterName)
				}
				if inv.Annotations != nil {
					r.Description = inv.Annotations[profileDescAnnotation]
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
		Short: "Apply an autoscaling profile to an HPA Unit (dry-run unless --commit)",
		Long: `apply invokes a stored autoscaling profile over an HPA Unit. Supply profile
parameters with --param name=value (e.g. --param min=3 --param max=10 for hpa-range).

Dry-run unless --commit --change-desc; never bypasses ApplyGates.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			profileSlug := args[0]
			paramMap, err := parseParams(params)
			if err != nil {
				return err
			}
			changeDesc, dryRun, err := commit.Validate(fmt.Sprintf("apply autoscaling profile %s to %s", profileSlug, args[1]))
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
