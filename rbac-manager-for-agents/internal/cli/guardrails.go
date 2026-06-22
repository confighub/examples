// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/confighub/examples/rbac-manager-for-agents/internal/cub"
)

const (
	defaultPolicySpace  = "policy-guardrails"
	guardrailFilterSlug = "rbac-guardrails"
	guardrailPackLabel  = "rbac-guardrails"
)

// Guardrail CEL expressions, validated against RBAC manifests with
// `cub function local <manifest> vet-celexpr '<expr>'`.
const (
	celNoWildcards    = "!(r.kind in ['Role', 'ClusterRole']) || !has(r.rules) || !r.rules.exists(rule, (has(rule.verbs) && rule.verbs.exists(v, v == '*')) || (has(rule.resources) && rule.resources.exists(x, x == '*')) || (has(rule.apiGroups) && rule.apiGroups.exists(g, g == '*')))"
	celNoEscalation   = "!(r.kind in ['Role', 'ClusterRole']) || !has(r.rules) || !r.rules.exists(rule, has(rule.verbs) && rule.verbs.exists(v, v in ['escalate', 'bind', 'impersonate']))"
	celNoClusterAdmin = "r.kind != 'ClusterRoleBinding' || r.roleRef.name != 'cluster-admin'"
)

type guardrailTrigger struct {
	slug string
	desc string
	expr string
}

var guardrailTriggers = []guardrailTrigger{
	{"no-rbac-wildcards", "Warns on Roles/ClusterRoles with wildcard verbs, resources, or apiGroups. Fix: enumerate the specific verbs/resources the role needs.", celNoWildcards},
	{"no-rbac-privilege-escalation", "Warns on Roles/ClusterRoles granting escalate, bind, or impersonate. Fix: remove these verbs; they allow privilege escalation.", celNoEscalation},
	{"no-cluster-admin-binding", "Warns on ClusterRoleBindings to cluster-admin. Fix: bind a scoped role instead.", celNoClusterAdmin},
}

func newGuardrailsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "guardrails",
		Short: "Install and inspect the RBAC guardrail policy pack",
		Long: `guardrails manages a small pack of RBAC validation policies, defined once in a
policy Space and enforced fleet-wide via a shared Trigger Filter:

  no-rbac-wildcards             no wildcard verbs/resources/apiGroups
  no-rbac-privilege-escalation  no escalate/bind/impersonate verbs
  no-cluster-admin-binding      no ClusterRoleBindings to cluster-admin

Triggers are created with Warn=true (advisory ApplyWarnings, never blocking), so
installing on an existing fleet never blocks anyone. Promote one to blocking
later with: cub trigger update <slug> --space <policy-space> --unwarn`,
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
	PolicySpace       string      `json:"policySpace"`
	Filter            string      `json:"filter"`
	Triggers          []string    `json:"triggers"`
	Committed         bool        `json:"committed"`
	PolicySpaceExists bool        `json:"policySpaceExists"`
	Wire              []string    `json:"wire"`
	AlreadyWired      []string    `json:"alreadyWired"`
	Skipped           []skipEntry `json:"skipped"`
}

func newGuardrailsInstallCmd() *cobra.Command {
	var policySpace, whereSpace, output string
	var commit bool
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install the guardrail pack and wire in-scope Spaces (dry-run by default)",
		Long: `install creates the policy Space, the three Warn=true guardrail Triggers, and the
shared Trigger Filter, then points each in-scope Space's TriggerFilterID at that
Filter.

Dry-run by default: it prints the plan and changes nothing. Re-run with --commit
to apply. Spaces that already select Triggers another way (a custom WhereTrigger,
a different TriggerFilterID, or their own Triggers) are reported, not modified.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cub.Preflight(cmd.Context()); err != nil {
				return err
			}
			plan, err := buildGuardrailsPlan(cmd.Context(), policySpace, whereSpace)
			if err != nil {
				return err
			}
			if commit {
				if err := executeGuardrails(cmd, policySpace, plan); err != nil {
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
	cmd.Flags().StringVar(&policySpace, "policy-space", defaultPolicySpace, "Space that holds the guardrail Triggers and Filter")
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

func buildGuardrailsPlan(ctx context.Context, policySpace, whereSpace string) (guardrailsPlan, error) {
	plan := guardrailsPlan{
		PolicySpace: policySpace,
		Filter:      policySpace + "/" + guardrailFilterSlug,
	}
	for _, t := range guardrailTriggers {
		plan.Triggers = append(plan.Triggers, t.slug)
	}

	spaces, err := listSpaces(ctx, whereSpace)
	if err != nil {
		return plan, fmt.Errorf("list spaces: %w", err)
	}
	k8sSpaces, err := k8sSpaceSlugs(ctx)
	if err != nil {
		return plan, fmt.Errorf("find Kubernetes/YAML spaces: %w", err)
	}

	// Existing policy filter ID, if any, to detect already-wired Spaces.
	filterID, _ := guardrailFilterID(ctx, policySpace)
	if _, err := cub.Run(ctx, "space", "get", policySpace, "--quiet"); err == nil {
		plan.PolicySpaceExists = true
	}

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
			ownTriggers, err := spaceTriggerCount(ctx, s.Slug)
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

func executeGuardrails(cmd *cobra.Command, policySpace string, plan guardrailsPlan) error {
	ctx := cmd.Context()
	out := cmd.OutOrStdout()

	if _, err := cub.Run(ctx, "space", "create", policySpace, "--label", "app=rbac-manager", "--label", "role=policy", "--allow-exists"); err != nil {
		return fmt.Errorf("create policy space %s: %w", policySpace, err)
	}
	for _, t := range guardrailTriggers {
		if _, err := cub.Run(ctx, "trigger", "create", "--space", policySpace, "--warn", "--allow-exists",
			"--label", "Pack="+guardrailPackLabel, "--description", t.desc,
			t.slug, "Mutation", "Kubernetes/YAML", "vet-celexpr", t.expr); err != nil {
			return fmt.Errorf("create trigger %s: %w", t.slug, err)
		}
	}
	if _, err := cub.Run(ctx, "filter", "create", "--space", policySpace, "--allow-exists",
		guardrailFilterSlug, "Trigger", "--where-field", "Labels.Pack = '"+guardrailPackLabel+"'"); err != nil {
		return fmt.Errorf("create filter %s: %w", guardrailFilterSlug, err)
	}
	fprintln(out, fmt.Sprintf("Policy pack ready in %s.", policySpace))

	filterRef := policySpace + "/" + guardrailFilterSlug
	for _, slug := range plan.Wire {
		if _, err := cub.Run(ctx, "space", "update", slug, "--where-trigger", "-", "--trigger-filter", filterRef); err != nil {
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
		Short: "List Units with RBAC ApplyWarnings or ApplyGates",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cub.Preflight(cmd.Context()); err != nil {
				return err
			}
			rows, err := guardrailStatusRows(cmd.Context())
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

// guardrailStatusRows lists Units carrying ApplyWarnings or ApplyGates. The
// --where grammar is AND-only (no OR/parentheses), so it runs one query per
// condition and merges by space/unit.
func guardrailStatusRows(ctx context.Context) ([]statusRow, error) {
	byKey := map[string]statusRow{}
	for _, cond := range []string{"LEN(ApplyWarnings) > 0", "LEN(ApplyGates) > 0"} {
		units, err := listFlaggedUnits(ctx, "ToolchainType = 'Kubernetes/YAML' AND "+cond)
		if err != nil {
			return nil, err
		}
		for _, u := range units {
			byKey[u.Space.Slug+"/"+u.Unit.Slug] = statusRow{
				Space:    u.Space.Slug,
				Unit:     u.Unit.Slug,
				Warnings: len(u.Unit.ApplyWarnings),
				Gates:    len(u.Unit.ApplyGates),
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

type flaggedUnit struct {
	Unit struct {
		Slug          string         `json:"Slug"`
		ApplyWarnings map[string]any `json:"ApplyWarnings"`
		ApplyGates    map[string]any `json:"ApplyGates"`
	} `json:"Unit"`
	Space struct {
		Slug string `json:"Slug"`
	} `json:"Space"`
}

func listFlaggedUnits(ctx context.Context, where string) ([]flaggedUnit, error) {
	out, err := cub.Run(ctx, "unit", "list", "--space", "*", "--where", where,
		"--select", "Slug,SpaceID,ApplyWarnings,ApplyGates", "-o", "json")
	if err != nil {
		return nil, err
	}
	var units []flaggedUnit
	if out != "" {
		if err := json.Unmarshal([]byte(out), &units); err != nil {
			return nil, err
		}
	}
	return units, nil
}

// --- cub helpers ---

func listSpaces(ctx context.Context, whereSpace string) ([]spaceInfo, error) {
	// WhereTrigger / TriggerFilterID are not in the default column set, so they
	// come back null unless explicitly selected — select them so the
	// already-wired / custom-config classification is correct.
	const selectFields = "Slug,SpaceID,WhereTrigger,TriggerFilterID"
	args := []string{"space", "list", "--select", selectFields, "-o", "json"}
	if whereSpace != "" {
		args = []string{"space", "list", "--where", whereSpace, "--select", selectFields, "-o", "json"}
	}
	out, err := cub.Run(ctx, args...)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		Space struct {
			SpaceID         string `json:"SpaceID"`
			Slug            string `json:"Slug"`
			WhereTrigger    string `json:"WhereTrigger"`
			TriggerFilterID string `json:"TriggerFilterID"`
		} `json:"Space"`
	}
	if out != "" {
		if err := json.Unmarshal([]byte(out), &rows); err != nil {
			return nil, err
		}
	}
	infos := make([]spaceInfo, 0, len(rows))
	for _, r := range rows {
		infos = append(infos, spaceInfo{
			SpaceID:         r.Space.SpaceID,
			Slug:            r.Space.Slug,
			WhereTrigger:    r.Space.WhereTrigger,
			TriggerFilterID: r.Space.TriggerFilterID,
		})
	}
	return infos, nil
}

func k8sSpaceSlugs(ctx context.Context) (map[string]bool, error) {
	out, err := cub.Run(ctx, "unit", "list", "--space", "*", "--where", "ToolchainType = 'Kubernetes/YAML'", "-o", "jq=.[].Space.Slug")
	if err != nil {
		return nil, err
	}
	set := map[string]bool{}
	for line := range strings.SplitSeq(out, "\n") {
		s := strings.Trim(strings.TrimSpace(line), `"`)
		if s != "" {
			set[s] = true
		}
	}
	return set, nil
}

func guardrailFilterID(ctx context.Context, policySpace string) (string, error) {
	out, err := cub.Run(ctx, "filter", "get", guardrailFilterSlug, "--space", policySpace, "-o", "jq=.Filter.FilterID")
	if err != nil {
		return "", err
	}
	return strings.Trim(strings.TrimSpace(out), `"`), nil
}

func spaceTriggerCount(ctx context.Context, space string) (int, error) {
	out, err := cub.Run(ctx, "trigger", "list", "--space", space, "-o", "jq=.[].Trigger.Slug")
	if err != nil {
		return 0, err
	}
	count := 0
	for line := range strings.SplitSeq(out, "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count, nil
}
