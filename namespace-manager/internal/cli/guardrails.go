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

	"github.com/confighub/sdk/cliutil"
	"github.com/confighub/sdk/core/cubapi"
	api "github.com/confighub/sdk/core/function/api"
	goclientnew "github.com/confighub/sdk/core/openapi/goclient-new"

	"github.com/confighub/examples/namespace-manager/internal/cub"
	"github.com/confighub/examples/namespace-manager/internal/nsmanager"
	"github.com/confighub/examples/namespace-manager/internal/snapshot"
)

const (
	defaultNsPolicySpace = "namespace-policy"
	nsFilterSlug         = "namespace-guardrails"
	nsPackLabel          = "namespace-guardrails"
	// findingAnnotation is written by `guardrails annotate` onto the Namespace
	// Unit of each incomplete namespace, and read by the envelope-finding Trigger
	// (annotate-then-validate).
	findingAnnotation = "namespace.confighub.com/finding"
)

// Guardrail CEL expressions, evaluated per resource (r aliases object).
const (
	// A Namespace must carry the pod-security enforce label.
	celHasPodSecurity = "r.kind != 'Namespace' || (has(r.metadata.labels) && 'pod-security.kubernetes.io/enforce' in r.metadata.labels)"
	// Annotate-then-validate: warn while an envelope finding annotation is present.
	celNoEnvelopeFinding = "!has(r.metadata.annotations) || !('" + findingAnnotation + "' in r.metadata.annotations)"
)

type guardrailTrigger struct {
	slug string
	desc string
	expr string
}

var guardrailTriggers = []guardrailTrigger{
	{"namespace-has-pod-security", "Warns on a Namespace with no pod-security.kubernetes.io/enforce label. Fix: `cub-namespace apply-envelope --space <s>`.", celHasPodSecurity},
	{"namespace-envelope-finding", "Warns while a namespace.confighub.com/finding annotation is present (set by `cub-namespace guardrails annotate`). Fix: close the envelope gap, then re-run annotate.", celNoEnvelopeFinding},
}

func newGuardrailsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "guardrails",
		Short: "Install, inspect, and feed the namespace-envelope guardrail policy pack",
		Long: `guardrails manages a pack of namespace-governance validation policies, defined
once in a policy Space and enforced fleet-wide via a shared Trigger Filter:

  namespace-has-pod-security   a Namespace must carry a pod-security enforce label
  namespace-envelope-finding   warns while an envelope finding is annotated (§ annotate)

Triggers are created with Warn=true (advisory ApplyWarnings, never blocking).
Promote one to blocking later with:
  cub trigger update <slug> --space <policy-space> --unwarn

The envelope-finding rule realizes the annotate-then-validate model: the manager
can't set ApplyWarnings directly (only a failed Trigger can), so 'guardrails
annotate' writes a finding annotation onto each incomplete namespace's Namespace
Unit and this Trigger turns it into a warning.

The namespace-name invariant (metadata.namespace == normalizeName(Component)) is
enforced separately, by a cluster-selected mutating set-namespace Trigger — the
promotable active-correction option, wired outside this advisory pack.`,
	}
	cmd.AddCommand(newGuardrailsInstallCmd(), newGuardrailsStatusCmd(), newGuardrailsAnnotateCmd())
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
		Long: `install creates the policy Space, the Warn=true guardrail Triggers, and the
shared Trigger Filter, then points each in-scope Space's TriggerFilterID at it.

Dry-run by default: it prints the plan and changes nothing. Re-run with --commit
to apply. Spaces that already select Triggers another way are reported, not
modified. Narrow which Spaces get wired with --where-space.`,
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
	cmd.Flags().StringVar(&policySpace, "policy-space", defaultNsPolicySpace, "Space that holds the guardrail Triggers and Filter")
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
	plan := guardrailsPlan{PolicySpace: policySpace, Filter: policySpace + "/" + nsFilterSlug}
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
	if _, err := cubapi.ResolveSpace(ctx, client, policySpace); err == nil {
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
		Labels: map[string]string{"app": "namespace-manager", "role": "policy"},
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
			FunctionName:  "vet-celexpr",
			Arguments:     cubapi.Arguments([]api.FunctionArgument{{Value: t.expr}}),
			Warn:          true,
			Labels:        map[string]string{"Pack": nsPackLabel},
		}); err != nil {
			return fmt.Errorf("create trigger %s: %w", t.slug, err)
		}
	}
	flt, err := cubapi.EnsureFilter(ctx, client, goclientnew.Filter{
		SpaceID: ps.SpaceID,
		Slug:    nsFilterSlug,
		From:    "Trigger",
		Where:   "Labels.Pack = '" + nsPackLabel + "'",
	})
	if err != nil {
		return fmt.Errorf("create filter %s: %w", nsFilterSlug, err)
	}
	fprintln(out, fmt.Sprintf("Policy pack ready in %s.", policySpace))

	filterRef := policySpace + "/" + nsFilterSlug
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
		Short: "List Units with namespace-governance ApplyWarnings or ApplyGates",
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

// --- annotate (the producing half of annotate-then-validate) ---

type annotateResult struct {
	Cluster   string `json:"cluster"`
	Namespace string `json:"namespace"`
	Unit      string `json:"unit"`
	Finding   string `json:"finding"`
	OK        bool   `json:"ok"`
	Error     string `json:"error,omitempty"`
}

func newGuardrailsAnnotateCmd() *cobra.Command {
	var output, clusterFilter string
	var filter filterFlags
	var commit cliutil.CommitFlags
	cmd := &cobra.Command{
		Use:   "annotate",
		Short: "Write a finding annotation onto the Namespace Unit of each incomplete namespace (dry-run unless --commit)",
		Long: `annotate runs the envelope analysis and writes a namespace.confighub.com/finding
annotation (value = the missing members) onto the v1/Namespace Unit of each
incomplete namespace. Paired with the namespace-envelope-finding Trigger from
'guardrails install', this turns a set-aware finding into an advisory
ApplyWarning — the manager cannot set warnings directly, only a failed Trigger
can.

Only namespaces that have a Namespace Unit are annotated (there is nowhere else
to put the annotation). Re-run after fixing config. Dry run unless
--commit --change-desc.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			changeDesc, dryRun, err := commit.Validate("annotate Namespace Units with envelope findings")
			if err != nil {
				return err
			}
			client, err := cub.Preflight(cmd.Context())
			if err != nil {
				return err
			}
			snap, err := snapshot.Load(cmd.Context(), client, filter.predicate())
			if err != nil {
				return err
			}

			var results []annotateResult
			for _, c := range snap.Clusters {
				if clusterFilter != "" && c.Cluster != clusterFilter {
					continue
				}
				for _, e := range nsmanager.AnalyzeCluster(c) {
					if e.Complete || e.UnitSlug == "" || e.SpaceID == "" {
						continue // complete, or no Namespace Unit to annotate
					}
					r := annotateResult{Cluster: e.Cluster, Namespace: e.Namespace, Unit: e.UnitSlug, Finding: strings.Join(e.Missing, ",")}
					if !dryRun {
						err := annotateUnit(cmd.Context(), client, e.SpaceID, e.UnitSlug, r.Finding, changeDesc)
						r.OK = err == nil
						if err != nil {
							r.Error = err.Error()
						}
					}
					results = append(results, r)
				}
			}
			sort.Slice(results, func(i, j int) bool {
				if results[i].Cluster != results[j].Cluster {
					return results[i].Cluster < results[j].Cluster
				}
				return results[i].Namespace < results[j].Namespace
			})

			if output == outputJSON {
				return printJSON(cmd.OutOrStdout(), results)
			}
			out := cmd.OutOrStdout()
			for _, r := range results {
				fprintln(out, fmt.Sprintf("%s/%s  %s  (finding: %s)", r.Cluster, r.Namespace, r.Unit, r.Finding))
			}
			if dryRun {
				fprintln(out, fmt.Sprintf("\nDry run — %d Namespace Unit(s) would be annotated. Re-run with --commit --change-desc \"…\".", len(results)))
			} else {
				fprintln(out, fmt.Sprintf("\nAnnotated %d Namespace Unit(s).", len(results)))
			}
			return nil
		},
	}
	addOutputFlag(cmd, &output)
	addFilterFlags(cmd, &filter)
	cmd.Flags().StringVar(&clusterFilter, "cluster", "", "only annotate Units in this cluster")
	commit.Bind(cmd)
	return cmd
}

// annotateUnit sets the finding annotation on one Unit via the set-annotation
// function (a committed Unit-data mutation).
func annotateUnit(ctx context.Context, client *cubapi.Client, spaceID, unitSlug, value, changeDesc string) error {
	_, err := cubapi.InvokeFunction(ctx, client,
		api.FunctionInvocation{
			FunctionName: "set-annotation",
			Arguments: []api.FunctionArgument{
				{ParameterName: "annotation-key", Value: findingAnnotation},
				{ParameterName: "annotation-value", Value: value},
			},
		},
		cubapi.Selector{Where: fmt.Sprintf("SpaceID = '%s' AND Slug = '%s'", spaceID, unitSlug)},
		cubapi.Change{Description: changeDesc})
	return err
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
	flt, err := cubapi.ResolveFilter(ctx, client, ps.SpaceID, nsFilterSlug)
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
