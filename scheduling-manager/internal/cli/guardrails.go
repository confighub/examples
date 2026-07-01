// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/confighub/sdk/core/cubapi"
	api "github.com/confighub/sdk/core/function/api"
	goclientnew "github.com/confighub/sdk/core/openapi/goclient-new"

	"github.com/confighub/examples/scheduling-manager/internal/cub"
)

const (
	defaultSchedulingPolicySpace = "scheduling-policy"
	schedFilterSlug              = "scheduling-guardrails"
	schedPackLabel               = "scheduling-guardrails"
)

const controllerKinds = "['Deployment', 'StatefulSet', 'DaemonSet', 'ReplicaSet']"

// celTolerationNeedsPlacement passes unless a controller has tolerations but
// neither a non-empty nodeSelector nor a required node affinity (i.e. it tolerates
// a taint but doesn't actually pin where it lands). vet-cel: the resource is 'r'.
const celTolerationNeedsPlacement = "!(r.kind in " + controllerKinds + ")" +
	" || !has(r.spec.template.spec.tolerations)" +
	" || (has(r.spec.template.spec.nodeSelector) && size(r.spec.template.spec.nodeSelector) > 0)" +
	" || (has(r.spec.template.spec.affinity) && has(r.spec.template.spec.affinity.nodeAffinity) && has(r.spec.template.spec.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution))"

type guardrailTrigger struct {
	slug string
	desc string
	expr string
}

var guardrailTriggers = []guardrailTrigger{
	{"workload-toleration-needs-placement",
		"Warns on a controller that tolerates a taint but has no nodeSelector or required node affinity (may schedule onto general nodes). Fix: `cub-scheduling set-node-selector` / `set-node-affinity`, or a placement profile.",
		celTolerationNeedsPlacement},
}

func newGuardrailsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "guardrails",
		Short: "Install and inspect the placement guardrail policy pack",
		Long: `guardrails manages a pack of placement validation policies, defined once in a
policy Space and enforced fleet-wide via a shared Trigger Filter:

  workload-toleration-needs-placement   a controller that tolerates a taint must
                                        pin where it lands (nodeSelector or
                                        required node affinity)

The rule is a plain per-resource vet-cel check (a single Unit answers it), created
with Warn=true (advisory ApplyWarnings, never blocking). Promote it to blocking
with: cub trigger update <slug> --space scheduling-policy --unwarn`,
	}
	cmd.AddCommand(newGuardrailsInstallCmd(), newGuardrailsStatusCmd())
	return cmd
}

// --- install ---

type skipEntry struct {
	Space  string `json:"space"`
	Reason string `json:"reason"`
}

type guardrailsPlan struct {
	PolicySpace  string      `json:"policySpace"`
	Filter       string      `json:"filter"`
	Triggers     []string    `json:"triggers"`
	Committed    bool        `json:"committed"`
	Wire         []string    `json:"wire"`
	AlreadyWired []string    `json:"alreadyWired"`
	Skipped      []skipEntry `json:"skipped"`
}

func newGuardrailsInstallCmd() *cobra.Command {
	var policySpace, whereSpace, output string
	var commit bool
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install the guardrail pack and wire in-scope Spaces (dry-run by default)",
		Long: `install creates the policy Space, the Warn=true guardrail Trigger, and the shared
Trigger Filter, then points each in-scope Space's TriggerFilterID at it.

Dry-run by default. Re-run with --commit to apply. Spaces that already select
Triggers another way are reported, not modified.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cub.Preflight(cmd.Context())
			if err != nil {
				return err
			}
			plan, err := buildGuardrailsPlan(cmd.Context(), client, policySpace, whereSpace)
			if err != nil {
				return err
			}
			if commit {
				if err := executeGuardrails(cmd, client, policySpace, plan); err != nil {
					return err
				}
				plan.Committed = true
			}
			if output == outputTable {
				printGuardrailsPlan(cmd, plan)
				return nil
			}
			return printJSON(cmd.OutOrStdout(), plan)
		},
	}
	cmd.Flags().StringVar(&policySpace, "policy-space", defaultSchedulingPolicySpace, "Space that holds the guardrail Triggers and Filter")
	cmd.Flags().StringVar(&whereSpace, "where-space", "", "ConfigHub filter over Spaces to narrow which Spaces get wired")
	cmd.Flags().BoolVar(&commit, "commit", false, "apply the plan (default is dry-run)")
	addOutputFlag(cmd, &output)
	return cmd
}

type spaceInfo struct {
	SpaceID         string
	Slug            string
	WhereTrigger    string
	TriggerFilterID string
}

func buildGuardrailsPlan(ctx context.Context, client *cubapi.Client, policySpace, whereSpace string) (guardrailsPlan, error) {
	plan := guardrailsPlan{PolicySpace: policySpace, Filter: policySpace + "/" + schedFilterSlug}
	for _, t := range guardrailTriggers {
		plan.Triggers = append(plan.Triggers, t.slug)
	}
	spaces, err := listSpacesForGuardrails(ctx, client, whereSpace)
	if err != nil {
		return plan, fmt.Errorf("list spaces: %w", err)
	}
	k8sSpaces, err := k8sSpaceSlugs(ctx, client)
	if err != nil {
		return plan, fmt.Errorf("find Kubernetes/YAML spaces: %w", err)
	}
	filterID, _ := guardrailFilterID(ctx, client, policySpace)
	for _, s := range spaces {
		if s.Slug == policySpace || !k8sSpaces[s.Slug] {
			continue
		}
		switch {
		case s.TriggerFilterID != "" && filterID != "" && s.TriggerFilterID == filterID:
			plan.AlreadyWired = append(plan.AlreadyWired, s.Slug)
		case s.TriggerFilterID != "":
			plan.Skipped = append(plan.Skipped, skipEntry{s.Slug, "has a different TriggerFilterID — add the guardrail Filter to that Filter's set"})
		case s.WhereTrigger != "" && !strings.Contains(s.WhereTrigger, s.SpaceID):
			plan.Skipped = append(plan.Skipped, skipEntry{s.Slug, "custom WhereTrigger — point it at the guardrail Filter as well"})
		default:
			ownTriggers, err := spaceTriggerCount(ctx, client, s.SpaceID)
			if err != nil {
				return plan, err
			}
			if ownTriggers > 0 {
				plan.Skipped = append(plan.Skipped, skipEntry{s.Slug, fmt.Sprintf("has %d Trigger(s) of its own — add the guardrail Filter to its WhereTrigger to keep both", ownTriggers)})
			} else {
				plan.Wire = append(plan.Wire, s.Slug)
			}
		}
	}
	sort.Strings(plan.Wire)
	sort.Strings(plan.AlreadyWired)
	sort.Slice(plan.Skipped, func(i, j int) bool { return plan.Skipped[i].Space < plan.Skipped[j].Space })
	return plan, nil
}

func executeGuardrails(cmd *cobra.Command, client *cubapi.Client, policySpace string, plan guardrailsPlan) error {
	ctx := cmd.Context()
	out := cmd.OutOrStdout()
	ps, err := cubapi.EnsureSpace(ctx, client, goclientnew.Space{
		Slug:   policySpace,
		Labels: map[string]string{"app": "scheduling-manager", "role": "policy"},
	})
	if err != nil {
		return fmt.Errorf("create policy space %s: %w", policySpace, err)
	}
	for _, t := range guardrailTriggers {
		if _, err := cubapi.EnsureTrigger(ctx, client, goclientnew.Trigger{
			SpaceID:       ps.SpaceID,
			Slug:          t.slug,
			Description:   t.desc,
			Event:         "Mutation",
			ToolchainType: "Kubernetes/YAML",
			FunctionName:  "vet-cel",
			Arguments:     cubapi.Arguments([]api.FunctionArgument{{ParameterName: "expression", Value: t.expr}}),
			Warn:          true,
			Labels:        map[string]string{"Pack": schedPackLabel},
		}); err != nil {
			return fmt.Errorf("create trigger %s: %w", t.slug, err)
		}
	}
	flt, err := cubapi.EnsureFilter(ctx, client, goclientnew.Filter{
		SpaceID: ps.SpaceID,
		Slug:    schedFilterSlug,
		From:    "Trigger",
		Where:   "Labels.Pack = '" + schedPackLabel + "'",
	})
	if err != nil {
		return fmt.Errorf("create filter %s: %w", schedFilterSlug, err)
	}
	fprintln(out, fmt.Sprintf("Policy pack ready in %s.", policySpace))
	filterRef := policySpace + "/" + schedFilterSlug
	for _, slug := range plan.Wire {
		sp, err := cubapi.ResolveSpace(ctx, client, slug)
		if err != nil {
			return fmt.Errorf("wire space %s: %w", slug, err)
		}
		if err := cubapi.SetSpaceTriggerFilter(ctx, client, sp, flt.FilterID); err != nil {
			return fmt.Errorf("wire space %s: %w", slug, err)
		}
		fprintln(out, "  wired "+slug+" → "+filterRef)
	}
	return nil
}

func printGuardrailsPlan(cmd *cobra.Command, plan guardrailsPlan) {
	out := cmd.OutOrStdout()
	verb := "Plan (dry-run)"
	if plan.Committed {
		verb = "Applied"
	}
	fprintln(out, fmt.Sprintf("%s — policy pack %q, filter %q", verb, plan.PolicySpace, plan.Filter))
	fprintln(out, "  triggers: "+strings.Join(plan.Triggers, ", "))
	fprintln(out, fmt.Sprintf("  spaces to wire (%d): %s", len(plan.Wire), strings.Join(plan.Wire, ", ")))
	if len(plan.AlreadyWired) > 0 {
		fprintln(out, fmt.Sprintf("  already wired (%d): %s", len(plan.AlreadyWired), strings.Join(plan.AlreadyWired, ", ")))
	}
	if len(plan.Skipped) > 0 {
		tw := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
		fmt.Fprintln(tw, "  SKIPPED\tREASON")
		for _, s := range plan.Skipped {
			fmt.Fprintf(tw, "  %s\t%s\n", s.Space, s.Reason)
		}
		_ = tw.Flush()
	}
	if !plan.Committed {
		fprintln(out, "\nNothing changed. Re-run with --commit to apply.")
	}
}

// --- status ---

func newGuardrailsStatusCmd() *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "List Units with placement ApplyWarnings or ApplyGates",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cub.Preflight(cmd.Context())
			if err != nil {
				return err
			}
			rows, err := guardrailStatusRows(cmd.Context(), client)
			if err != nil {
				return err
			}
			if output == outputTable {
				tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
				fmt.Fprintln(tw, "SPACE\tUNIT\tWARNINGS\tGATES")
				for _, r := range rows {
					fmt.Fprintf(tw, "%s\t%s\t%d\t%d\n", r.Space, r.Unit, r.Warnings, r.Gates)
				}
				_ = tw.Flush()
				fprintln(cmd.OutOrStdout(), fmt.Sprintf("\n%d Units flagged", len(rows)))
				return nil
			}
			return printJSON(cmd.OutOrStdout(), rows)
		},
	}
	addOutputFlag(cmd, &output)
	return cmd
}

type statusRow struct {
	Space    string `json:"space"`
	Unit     string `json:"unit"`
	Warnings int    `json:"warnings"`
	Gates    int    `json:"gates"`
}

func guardrailStatusRows(ctx context.Context, client *cubapi.Client) ([]statusRow, error) {
	byKey := map[string]statusRow{}
	for _, cond := range []string{"LEN(ApplyWarnings) > 0", "LEN(ApplyGates) > 0"} {
		units, err := cubapi.ListUnits(ctx, client,
			cubapi.NewWhere("ToolchainType = 'Kubernetes/YAML'").And(cond),
			cubapi.ListOpts{Include: "SpaceID", Select: "Slug,SpaceID,ApplyWarnings,ApplyGates"})
		if err != nil {
			return nil, err
		}
		for _, eu := range units {
			if eu.Unit == nil {
				continue
			}
			space := ""
			if eu.Space != nil {
				space = eu.Space.Slug
			}
			byKey[space+"/"+eu.Unit.Slug] = statusRow{
				Space: space, Unit: eu.Unit.Slug,
				Warnings: len(eu.Unit.ApplyWarnings), Gates: len(eu.Unit.ApplyGates),
			}
		}
	}
	rows := make([]statusRow, 0, len(byKey))
	for _, r := range byKey {
		rows = append(rows, r)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Space != rows[j].Space {
			return rows[i].Space < rows[j].Space
		}
		return rows[i].Unit < rows[j].Unit
	})
	return rows, nil
}

// --- query helpers ---

func listSpacesForGuardrails(ctx context.Context, client *cubapi.Client, whereSpace string) ([]spaceInfo, error) {
	where := cubapi.Where{}
	if whereSpace != "" {
		where = cubapi.NewWhere(whereSpace)
	}
	spaces, err := cubapi.ListSpaces(ctx, client, where,
		cubapi.ListOpts{Select: "Slug,SpaceID,WhereTrigger,TriggerFilterID"})
	if err != nil {
		return nil, err
	}
	infos := make([]spaceInfo, 0, len(spaces))
	for _, es := range spaces {
		if es.Space == nil {
			continue
		}
		triggerFilterID := ""
		if es.Space.TriggerFilterID != nil {
			triggerFilterID = es.Space.TriggerFilterID.String()
		}
		infos = append(infos, spaceInfo{
			SpaceID: es.Space.SpaceID.String(), Slug: es.Space.Slug,
			WhereTrigger: es.Space.WhereTrigger, TriggerFilterID: triggerFilterID,
		})
	}
	return infos, nil
}

func k8sSpaceSlugs(ctx context.Context, client *cubapi.Client) (map[string]bool, error) {
	units, err := cubapi.ListUnits(ctx, client, cubapi.NewWhere("ToolchainType = 'Kubernetes/YAML'"),
		cubapi.ListOpts{Include: "SpaceID", Select: "Slug,SpaceID"})
	if err != nil {
		return nil, err
	}
	set := map[string]bool{}
	for _, eu := range units {
		if eu.Space != nil && eu.Space.Slug != "" {
			set[eu.Space.Slug] = true
		}
	}
	return set, nil
}

func guardrailFilterID(ctx context.Context, client *cubapi.Client, policySpace string) (string, error) {
	ps, err := cubapi.ResolveSpace(ctx, client, policySpace)
	if err != nil {
		return "", err
	}
	flt, err := cubapi.ResolveFilter(ctx, client, ps.SpaceID, schedFilterSlug)
	if err != nil {
		return "", err
	}
	return flt.FilterID.String(), nil
}

func spaceTriggerCount(ctx context.Context, client *cubapi.Client, spaceID string) (int, error) {
	triggers, err := cubapi.ListTriggers(ctx, client,
		cubapi.NewWhere(fmt.Sprintf("SpaceID = '%s'", spaceID)),
		cubapi.ListOpts{Select: "Slug,SpaceID"})
	if err != nil {
		return 0, err
	}
	return len(triggers), nil
}
