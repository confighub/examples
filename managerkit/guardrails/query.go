// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package guardrails

import (
	"context"
	"fmt"
	"sort"

	"github.com/confighub/sdk/core/cubapi"
	api "github.com/confighub/sdk/core/function/api"
)

// StatusRow is one Unit carrying an ApplyWarning or an ApplyGate.
type StatusRow struct {
	Space    string `json:"space"`
	Unit     string `json:"unit"`
	Warnings int    `json:"warnings"`
	Gates    int    `json:"gates"`
}

// Status lists the Units a pack's Triggers have marked, warnings and gates
// alike, sorted by space and unit.
//
// It reports what any Trigger attached, not only this pack's: a Unit does not
// record which rule warned it, and a tool that filtered to its own rules would
// under-report a Unit that is blocked for some other reason entirely.
func Status(ctx context.Context, client *cubapi.Client) ([]StatusRow, error) {
	byKey := map[string]StatusRow{}
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
			byKey[space+"/"+eu.Unit.Slug] = StatusRow{
				Space: space, Unit: eu.Unit.Slug,
				Warnings: len(eu.Unit.ApplyWarnings), Gates: len(eu.Unit.ApplyGates),
			}
		}
	}
	rows := make([]StatusRow, 0, len(byKey))
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

// Annotate writes one annotation onto a Unit's resources.
//
// It is the producing half of annotate-then-validate: a manager cannot attach an
// ApplyWarning itself -- only a failed Trigger can -- so a finding that no
// single resource can express is written as an annotation, and a rule in the
// pack warns for as long as the annotation is there.
func Annotate(ctx context.Context, client *cubapi.Client, spaceID, unitSlug, key, value, changeDesc string) error {
	_, err := cubapi.InvokeFunction(ctx, client,
		api.FunctionInvocation{
			FunctionName: "set-annotation",
			Arguments: []api.FunctionArgument{
				{ParameterName: "annotation-key", Value: key},
				{ParameterName: "annotation-value", Value: value},
			},
		},
		cubapi.Selector{Where: fmt.Sprintf("SpaceID = '%s' AND Slug = '%s'", spaceID, unitSlug)},
		cubapi.Change{Description: changeDesc})
	return err
}

// KubernetesSpaces is the set of Space slugs holding at least one
// Kubernetes/YAML Unit. Wiring a Space that holds none would attach Triggers
// that can never run.
func KubernetesSpaces(ctx context.Context, client *cubapi.Client) (map[string]bool, error) {
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

// listSpaces returns the Space metadata the plan reasons about, optionally
// narrowed by a Space filter.
func listSpaces(ctx context.Context, client *cubapi.Client, whereSpace string) ([]spaceInfo, error) {
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

// spaceTriggerCount is how many Triggers a Space defines of its own.
func spaceTriggerCount(ctx context.Context, client *cubapi.Client, spaceID string) (int, error) {
	triggers, err := cubapi.ListTriggers(ctx, client,
		cubapi.NewWhere(fmt.Sprintf("SpaceID = '%s'", spaceID)),
		cubapi.ListOpts{Select: "Slug,SpaceID"})
	if err != nil {
		return 0, err
	}
	return len(triggers), nil
}
