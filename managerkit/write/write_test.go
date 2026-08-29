// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package write

import (
	"context"
	"testing"

	"github.com/confighub/sdk/core/cubapi"
	goclientnew "github.com/confighub/sdk/core/openapi/goclient-new"
	"github.com/google/uuid"
)

// A malformed target is rejected before the Space lookup, so no request is made
// for an argument that was never a target.
func TestParseUnitRefRejectsMalformedTarget(t *testing.T) {
	for _, bad := range []string{"nospace", "/unit", "space/", ""} {
		if _, err := ParseUnitRef(context.Background(), nil, bad); err == nil {
			t.Errorf("ParseUnitRef(%q) was accepted", bad)
		}
	}
}

func TestUnitRefSelector(t *testing.T) {
	id := goclientnew.UUID(uuid.MustParse("11111111-1111-1111-1111-111111111111"))
	sel := UnitRef{SpaceID: id, SpaceSlug: "prod", UnitSlug: "web"}.Selector()
	want := "SpaceID = '11111111-1111-1111-1111-111111111111' AND Slug = 'web'"
	if sel.Where != want {
		t.Errorf("selector = %q, want %q", sel.Where, want)
	}
}

func TestChangeIsDryRunWhenUndescribed(t *testing.T) {
	// An empty Description is what makes a call a dry run, so a dry run is
	// spelled by not describing one.
	if ch := Change("scale to 3", true); !ch.DryRun() {
		t.Errorf("dry run carried a description: %+v", ch)
	}
	ch := Change("scale to 3", false)
	if ch.DryRun() || ch.Description != "scale to 3" {
		t.Errorf("commit = %+v", ch)
	}
}

// A command that runs several functions over the same Unit shows one row,
// mutated if any function changed it.
func TestSummarizeAggregatesPerUnit(t *testing.T) {
	res := &cubapi.Result{Outcomes: []cubapi.UnitOutcome{
		{UnitSlug: "web", Success: true, HasMutations: false},
		{UnitSlug: "web", Success: true, HasMutations: true},
		{UnitSlug: "api", Success: true, HasMutations: true},
	}}
	rep := Summarize("bulk", "prod", false, res)
	if len(rep.Outcomes) != 2 {
		t.Fatalf("outcomes = %+v, want one row per Unit", rep.Outcomes)
	}
	// Sorted by slug, so api leads.
	if rep.Outcomes[0].Unit != "api" || rep.Outcomes[1].Unit != "web" {
		t.Errorf("not sorted: %+v", rep.Outcomes)
	}
	if !rep.Outcomes[1].Mutated {
		t.Error("web mutated by one of two functions should count as mutated")
	}
	if rep.Mutated != 2 || rep.Committed != 2 {
		t.Errorf("mutated=%d committed=%d, want 2/2", rep.Mutated, rep.Committed)
	}
}

// Committed follows the server's Success, which it reports separately from
// Error: a failed outcome that carried no message must not count as committed.
func TestSummarizeDoesNotCommitAFailure(t *testing.T) {
	res := &cubapi.Result{Outcomes: []cubapi.UnitOutcome{
		{UnitSlug: "web", Success: false, HasMutations: true},
	}}
	rep := Summarize("edit", "", false, res)
	if rep.Mutated != 1 {
		t.Errorf("mutated = %d, want 1", rep.Mutated)
	}
	if rep.Committed != 0 {
		t.Errorf("committed = %d, want 0 — the outcome did not succeed", rep.Committed)
	}
}

// On a dry run nothing is committed, however the outcomes read.
func TestSummarizeDryRunCommitsNothing(t *testing.T) {
	res := &cubapi.Result{Outcomes: []cubapi.UnitOutcome{
		{UnitSlug: "web", Success: true, HasMutations: true},
	}}
	rep := Summarize("edit", "", true, res)
	if !rep.DryRun || rep.Mutated != 1 || rep.Committed != 0 {
		t.Errorf("dry run = %+v", rep)
	}
}

func TestSummarizeTolerantOfNilResults(t *testing.T) {
	rep := Summarize("edit", "", false, nil, nil)
	if len(rep.Outcomes) != 0 || rep.Mutated != 0 {
		t.Errorf("nil results = %+v", rep)
	}
}
