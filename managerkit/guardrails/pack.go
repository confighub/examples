// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package guardrails installs and inspects a tool's policy pack: a set of
// validating Triggers defined once in a policy Space and enforced fleet-wide
// through a shared Trigger Filter.
//
// Every one of these example tools ships a pack, and the mechanism is the same
// each time -- create the Space, create the Triggers, create the Filter that
// selects them, point each in-scope Space's TriggerFilterID at it, and never
// clobber a Space that already selects Triggers its own way. What differs is
// only the rules. A tool supplies those; this supplies the rest.
package guardrails

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/confighub/sdk/core/cubapi"
	api "github.com/confighub/sdk/core/function/api"
	goclientnew "github.com/confighub/sdk/core/openapi/goclient-new"
)

// ValidatingFunction is the function every rule's expression is run by.
//
// vet-cel evaluates a CEL expression once per resource with the Kubernetes
// admission-policy libraries available (quantity, url, ip, cidr, regex,
// format). The generic vet-celexpr accepts the same expressions and returns the
// same verdicts, but knows nothing about Kubernetes.
const ValidatingFunction = "vet-cel"

// Rule is one validating policy in a pack.
//
// Expression is CEL over `r`, the resource under test, run by vet-cel. It should
// pass for resources the rule does not govern, which is why these expressions
// conventionally open with a kind test: a rule that fails everything it does not
// apply to is not a rule, it is an outage.
//
// A rule that is not a CEL expression at all -- schema validation, say -- sets
// Function and Arguments instead.
type Rule struct {
	Slug        string
	Description string
	Expression  string

	// Function overrides the validating function. Empty means vet-cel with
	// Expression as its argument.
	Function string
	// Arguments are passed when Function is set. Ignored otherwise.
	Arguments []api.FunctionArgument
}

// invocation is the function and arguments this rule's Trigger runs.
func (r Rule) invocation() (string, []api.FunctionArgument) {
	if r.Function != "" {
		return r.Function, r.Arguments
	}
	return ValidatingFunction, []api.FunctionArgument{{ParameterName: "expression", Value: r.Expression}}
}

// Pack is a tool's set of guardrail rules and the ConfigHub entities that carry
// them.
type Pack struct {
	// App names the tool, and labels the policy Space it owns.
	App string
	// DefaultSpace is the policy Space slug used when --policy-space is not given.
	DefaultSpace string
	// FilterSlug names the shared Filter that selects this pack's Triggers.
	FilterSlug string
	// Label is the value of the Pack label the Filter selects on.
	Label string
	// Rules are the policies. They install with Warn=true: advisory
	// ApplyWarnings, never blocking, until someone promotes one with
	// `cub trigger update <slug> --space <policy-space> --unwarn`.
	Rules []Rule
}

// Skip is an in-scope Space that was deliberately left alone, and why.
type Skip struct {
	Space  string `json:"space"`
	Reason string `json:"reason"`
}

// Plan is what an install would do, and after a commit, what it did.
type Plan struct {
	PolicySpace       string   `json:"policySpace"`
	Filter            string   `json:"filter"`
	Triggers          []string `json:"triggers"`
	Committed         bool     `json:"committed"`
	PolicySpaceExists bool     `json:"policySpaceExists"`
	Wire              []string `json:"wire"`
	AlreadyWired      []string `json:"alreadyWired"`
	Skipped           []Skip   `json:"skipped"`
}

// spaceInfo is the Space metadata the plan reasons about.
type spaceInfo struct {
	SpaceID         string
	Slug            string
	WhereTrigger    string
	TriggerFilterID string
}

// BuildPlan works out which Spaces this pack would wire, and which it would
// leave alone. It writes nothing.
//
// A Space that already selects Triggers some other way is reported rather than
// modified: overwriting its TriggerFilterID would silently drop whatever policy
// it already has, and there is no way to tell from here whether that was
// deliberate.
func (p Pack) BuildPlan(ctx context.Context, client *cubapi.Client, policySpace, whereSpace string) (Plan, error) {
	plan := Plan{PolicySpace: policySpace, Filter: policySpace + "/" + p.FilterSlug}
	for _, r := range p.Rules {
		plan.Triggers = append(plan.Triggers, r.Slug)
	}

	k8sSpaces, err := KubernetesSpaces(ctx, client)
	if err != nil {
		return plan, fmt.Errorf("find Kubernetes/YAML spaces: %w", err)
	}
	candidates := make([]string, 0, len(k8sSpaces))
	for slug := range k8sSpaces {
		candidates = append(candidates, slug)
	}
	return p.PlanFor(ctx, client, policySpace, candidates, whereSpace)
}

// PlanFor is BuildPlan over a candidate set the caller chose, for a tool whose
// config lives in a recognizable subset of Spaces rather than in every Space
// holding Kubernetes config. Wiring the rest would attach rules that have
// nothing to say there.
//
// whereSpace, when set, narrows the candidates further.
func (p Pack) PlanFor(ctx context.Context, client *cubapi.Client, policySpace string, candidates []string, whereSpace string) (Plan, error) {
	plan := Plan{PolicySpace: policySpace, Filter: policySpace + "/" + p.FilterSlug}
	for _, r := range p.Rules {
		plan.Triggers = append(plan.Triggers, r.Slug)
	}

	want := make(map[string]bool, len(candidates))
	for _, slug := range candidates {
		want[slug] = true
	}
	spaces, err := listSpaces(ctx, client, whereSpace)
	if err != nil {
		return plan, fmt.Errorf("list spaces: %w", err)
	}
	filterID, _ := p.FilterID(ctx, client, policySpace)
	if _, err := cubapi.ResolveSpace(ctx, client, policySpace); err == nil {
		plan.PolicySpaceExists = true
	}

	for _, s := range spaces {
		if s.Slug == policySpace || !want[s.Slug] {
			continue
		}
		switch {
		case s.TriggerFilterID != "" && filterID != "" && s.TriggerFilterID == filterID:
			plan.AlreadyWired = append(plan.AlreadyWired, s.Slug)
		case s.TriggerFilterID != "":
			plan.Skipped = append(plan.Skipped, Skip{s.Slug, "has a different TriggerFilterID — add the guardrail Filter to that Filter's set"})
		case s.WhereTrigger != "" && !strings.Contains(s.WhereTrigger, s.SpaceID):
			plan.Skipped = append(plan.Skipped, Skip{s.Slug, "custom WhereTrigger — point it at the guardrail Filter as well"})
		default:
			own, err := spaceTriggerCount(ctx, client, s.SpaceID)
			if err != nil {
				return plan, err
			}
			if own > 0 {
				plan.Skipped = append(plan.Skipped, Skip{s.Slug, fmt.Sprintf("has %d Trigger(s) of its own — add the guardrail Filter to its WhereTrigger to keep both", own)})
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

// Execute applies a plan: the policy Space, the Triggers, the Filter, and the
// TriggerFilterID on each Space the plan chose to wire. It is idempotent --
// every entity is ensured rather than created -- so re-running after adding a
// rule installs the new Trigger and leaves the rest alone.
//
// progress, when non-nil, is called with a line per step for a CLI to print.
func (p Pack) Execute(ctx context.Context, client *cubapi.Client, policySpace string, plan Plan, progress func(string)) error {
	say := func(s string) {
		if progress != nil {
			progress(s)
		}
	}

	ps, err := cubapi.EnsureSpace(ctx, client, goclientnew.Space{
		Slug:   policySpace,
		Labels: map[string]string{"app": p.App, "role": "policy"},
	})
	if err != nil {
		return fmt.Errorf("create policy space %s: %w", policySpace, err)
	}
	for _, r := range p.Rules {
		fn, args := r.invocation()
		if _, err := cubapi.EnsureTrigger(ctx, client, goclientnew.Trigger{
			SpaceID:       ps.SpaceID,
			Slug:          r.Slug,
			Description:   r.Description,
			Event:         "Mutation",
			ToolchainType: "Kubernetes/YAML",
			FunctionName:  fn,
			Arguments:     cubapi.Arguments(args),
			Warn:          true,
			Labels:        map[string]string{"Pack": p.Label},
		}); err != nil {
			return fmt.Errorf("create trigger %s: %w", r.Slug, err)
		}
	}
	flt, err := cubapi.EnsureFilter(ctx, client, goclientnew.Filter{
		SpaceID: ps.SpaceID,
		Slug:    p.FilterSlug,
		From:    "Trigger",
		Where:   p.filterWhere(),
	})
	if err != nil {
		return fmt.Errorf("create filter %s: %w", p.FilterSlug, err)
	}
	say(fmt.Sprintf("Policy pack ready in %s.", policySpace))

	filterRef := policySpace + "/" + p.FilterSlug
	for _, slug := range plan.Wire {
		sp, err := cubapi.ResolveSpace(ctx, client, slug)
		if err != nil {
			return fmt.Errorf("wire space %s: %w", slug, err)
		}
		if err := cubapi.SetSpaceTriggerFilter(ctx, client, sp, flt.FilterID); err != nil {
			return fmt.Errorf("wire space %s: %w", slug, err)
		}
		say("  wired " + slug + " → " + filterRef)
	}
	return nil
}

// filterWhere selects this pack's Triggers and nothing else. Two packs can share
// a policy Space, so the clause has to key on the pack label rather than the
// Space: selecting the Space's Triggers would wire each pack's Spaces to both.
func (p Pack) filterWhere() string {
	return "Labels.Pack = '" + p.Label + "'"
}

// FilterID resolves the pack's Filter, empty when it is not installed yet.
func (p Pack) FilterID(ctx context.Context, client *cubapi.Client, policySpace string) (string, error) {
	ps, err := cubapi.ResolveSpace(ctx, client, policySpace)
	if err != nil {
		return "", err
	}
	flt, err := cubapi.ResolveFilter(ctx, client, ps.SpaceID, p.FilterSlug)
	if err != nil {
		return "", err
	}
	return flt.FilterID.String(), nil
}
