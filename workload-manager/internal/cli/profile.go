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

	"github.com/confighub/examples/workload-manager/internal/cub"
)

// profilesSpace is the Space that holds the parameterized workload profiles
// (stored Invocations). Created by `profile install`.
const profilesSpace = "workload-profiles"

// profileParam declares one profile parameter the caller supplies at apply time.
type profileParam struct {
	name     string
	dataType string // defaults to "string"
}

// profileSpec is a named, reusable workload edit: a function plus fixed arguments,
// with any variable values exposed as parameters (bound via {{ .Params.<name> }}).
type profileSpec struct {
	slug        string
	description string
	function    string
	args        []api.FunctionArgument
	params      []profileParam
}

// tmplArg binds a function parameter to a profile parameter via a Go-template ref.
func tmplArg(fnParam, profileParam string) api.FunctionArgument {
	return api.FunctionArgument{
		ParameterName: fnParam,
		Value:         "{{ .Params." + profileParam + " }}",
		Evaluator:     api.EvaluatorTemplate,
	}
}

func arg(fnParam string, value any) api.FunctionArgument {
	return api.FunctionArgument{ParameterName: fnParam, Value: value}
}

// terminationMsgYQ sets terminationMessagePolicy on every container of a
// workload's pod template.
const terminationMsgYQ = `.spec.template.spec.containers[].terminationMessagePolicy = "FallbackToLogsOnError"`

// antiAffinitySoftYQ adds preferred pod anti-affinity across nodes, using the
// pod-template labels as the selector.
const antiAffinitySoftYQ = `.spec.template.spec.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution = [{"weight": 100, "podAffinityTerm": {"topologyKey": "kubernetes.io/hostname", "labelSelector": {"matchLabels": .spec.template.metadata.labels}}}]`

// defaultProfiles is the seeded library. Resource tiers carry a container-name
// parameter; the rest are fixed.
func defaultProfiles() []profileSpec {
	resTier := func(slug, cpu, mem string) profileSpec {
		return profileSpec{
			slug:        slug,
			description: fmt.Sprintf("set-container-resources requests %s/%s, limits ×2", cpu, mem),
			function:    "set-container-resources",
			// The profile's own parameter namespace must be an identifier
			// (^[A-Za-z_][A-Za-z0-9_]*$), so the profile param is "container" even
			// though it binds the function's kebab-case "container-name" argument.
			params: []profileParam{{name: "container"}},
			args: []api.FunctionArgument{
				tmplArg("container-name", "container"),
				arg("operation", "all"),
				arg("cpu", cpu),
				arg("memory", mem),
				arg("limit-factor", 2),
			},
		}
	}
	return []profileSpec{
		resTier("resources-small", "100m", "128Mi"),
		resTier("resources-medium", "250m", "256Mi"),
		resTier("resources-large", "500m", "512Mi"),
		{
			slug:        "harden-restricted",
			description: "set-pod-container-security-context-defaults (runAsNonRoot, seccomp, drop ALL, readOnlyRootFilesystem)",
			function:    "set-pod-container-security-context-defaults",
		},
		{
			slug:        "probes-http",
			description: "set-container-probe-defaults (HTTP liveness/readiness/startup on the first port)",
			function:    "set-container-probe-defaults",
		},
		{
			slug:        "anti-affinity-soft",
			description: "set-yq: preferred pod anti-affinity across nodes (selector from pod-template labels)",
			function:    "set-yq",
			args:        []api.FunctionArgument{arg("yq-expression", antiAffinitySoftYQ)},
		},
		{
			slug:        "termination-message-policy",
			description: "set-yq: terminationMessagePolicy: FallbackToLogsOnError on all containers",
			function:    "set-yq",
			args:        []api.FunctionArgument{arg("yq-expression", terminationMsgYQ)},
		},
	}
}

func newProfileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "The workload profile library — reusable, parameterized edits (stored Invocations)",
		Long: `profile manages the workload-profiles Space: a library of named, parameterized
edits stored as ConfigHub Invocations. A profile bundles a function with preset
arguments (a resource tier, a hardening pass, a spread rule); 'profile apply'
invokes one over a workload, dry-run by default.`,
	}
	cmd.AddCommand(newProfileInstallCmd(), newProfileListCmd(), newProfileApplyCmd())
	return cmd
}

// newProfileInstallCmd seeds the profile library (idempotent).
func newProfileInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Create the workload-profiles Space and seed the default profiles (run once per org)",
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
				Labels: map[string]string{"app": "workload-manager", "role": "profiles"},
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

// buildInvocation turns a profileSpec into a stored Invocation, declaring its
// parameters and binding the templated args.
func buildInvocation(spaceID goclientnew.UUID, spec profileSpec) goclientnew.Invocation {
	params := make([]goclientnew.FunctionParameter, 0, len(spec.params))
	for _, p := range spec.params {
		dt := p.dataType
		if dt == "" {
			dt = "string"
		}
		params = append(params, goclientnew.FunctionParameter{
			ParameterName: p.name,
			DataType:      dt,
			Required:      true,
		})
	}
	return goclientnew.Invocation{
		SpaceID:       spaceID,
		Slug:          spec.slug,
		DisplayName:   spec.slug,
		ToolchainType: "Kubernetes/YAML",
		FunctionName:  spec.function,
		Arguments:     cubapi.Arguments(spec.args),
		Parameters:    params,
		Annotations:   map[string]string{"workload.confighub.com/description": spec.description},
	}
}

func newProfileListCmd() *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the profiles in the workload-profiles Space",
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
					r.Description = inv.Annotations["workload.confighub.com/description"]
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
		Short: "Apply a profile to a workload (dry-run unless --commit)",
		Long: `apply invokes a stored profile (a parameterized Invocation from the
workload-profiles Space) over a workload Unit. Supply profile parameters with
--param name=value (e.g. --param container-name=web for the resource tiers).

Dry-run unless --commit --change-desc; never bypasses ApplyGates.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			profileSlug := args[0]
			paramMap, err := parseParams(params)
			if err != nil {
				return err
			}
			changeDesc, dryRun, err := commit.Validate(fmt.Sprintf("apply profile %s to %s", profileSlug, args[1]))
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

// parseParams turns --param name=value flags into the map InvokeStoredInvocation
// expects.
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
