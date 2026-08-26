// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package clikit

import (
	"testing"

	api "github.com/confighub/sdk/core/function/api"
)

func TestParseScore(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want api.Score
	}{
		{"", api.ScoreNone},
		{"High", api.ScoreHigh},
		{"high", api.ScoreHigh},
		{"HIGH", api.ScoreHigh},
		{"critical", api.ScoreCritical},
		{"Medium", api.ScoreMedium},
		{"low", api.ScoreLow},
	} {
		got, err := ParseScore(tc.in)
		if err != nil || got != tc.want {
			t.Errorf("ParseScore(%q) = %q, %v; want %q", tc.in, got, err, tc.want)
		}
	}
	for _, bad := range []string{"urgent", "info", "hi"} {
		if _, err := ParseScore(bad); err == nil {
			t.Errorf("ParseScore(%q) was accepted", bad)
		}
	}
}

func TestFilterFlagsPredicate(t *testing.T) {
	for _, tc := range []struct {
		name string
		f    FilterFlags
		want string
	}{
		{"empty", FilterFlags{}, ""},
		{"one label", FilterFlags{Component: "checkout"}, "Space.Labels.Component = 'checkout'"},
		{"raw only", FilterFlags{Where: "Slug LIKE 'x%'"}, "Slug LIKE 'x%'"},
		{
			"raw and labels are ANDed, raw first",
			FilterFlags{Where: "Slug LIKE 'x%'", Environment: "prod"},
			"Slug LIKE 'x%' AND Space.Labels.Environment = 'prod'",
		},
		{
			"every label, in a stable order",
			FilterFlags{Component: "c", Environment: "e", Region: "r", Owner: "o", Layer: "l", Variant: "v"},
			"Space.Labels.Component = 'c' AND Space.Labels.Environment = 'e' AND " +
				"Space.Labels.Region = 'r' AND Space.Labels.Owner = 'o' AND " +
				"Space.Labels.Layer = 'l' AND Space.Labels.Variant = 'v'",
		},
		// A value carrying a quote would otherwise end the literal early.
		{"quote is doubled", FilterFlags{Owner: "o'brien"}, "Space.Labels.Owner = 'o''brien'"},
	} {
		if got := tc.f.Predicate(); got != tc.want {
			t.Errorf("%s:\n got %q\nwant %q", tc.name, got, tc.want)
		}
	}
}

// A tool-specific label scope joins the predicate like any other, so a tool can
// scope by a label the others have no use for without the flag spreading.
func TestFilterFlagsLabel(t *testing.T) {
	var f FilterFlags
	f.Label("cluster", "Cluster", "select Units whose Space has Labels.Cluster = <value>")
	if got := f.Predicate(); got != "" {
		t.Errorf("unset extra label contributed %q", got)
	}
	f.extra[0].value = "prod-use1"
	f.Environment = "prod"
	want := "Space.Labels.Environment = 'prod' AND Space.Labels.Cluster = 'prod-use1'"
	if got := f.Predicate(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	f.extra[0].value = "it's"
	f.Environment = ""
	if got := f.Predicate(); got != "Space.Labels.Cluster = 'it''s'" {
		t.Errorf("quote escaping: got %q", got)
	}
}
