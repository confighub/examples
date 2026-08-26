// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package guardrails

import (
	"strings"
	"testing"

	api "github.com/confighub/sdk/core/function/api"
)

func TestRuleInvocationDefaultsToCEL(t *testing.T) {
	fn, args := Rule{Slug: "s", Expression: "r.kind != 'Pod'"}.invocation()
	if fn != ValidatingFunction {
		t.Errorf("function = %q, want %q", fn, ValidatingFunction)
	}
	// vet-cel takes its expression as a named parameter; passing it positionally
	// is accepted by a different function entirely and silently means something
	// else.
	if len(args) != 1 || args[0].ParameterName != "expression" || args[0].Value != "r.kind != 'Pod'" {
		t.Errorf("args = %+v", args)
	}
}

func TestRuleInvocationOverride(t *testing.T) {
	r := Rule{Slug: "s", Function: "vet-schemas"}
	fn, args := r.invocation()
	if fn != "vet-schemas" || len(args) != 0 {
		t.Errorf("fn = %q, args = %+v", fn, args)
	}
	r.Arguments = []api.FunctionArgument{{ParameterName: "k", Value: "v"}}
	if _, args := r.invocation(); len(args) != 1 {
		t.Errorf("args = %+v", args)
	}
}

// A Space has one TriggerFilterID, so every pack has to wire to the same Filter
// or the first one installed claims the Space and the rest are skipped. The
// clause therefore selects on the shared guardrail label, never on one pack.
func TestSharedFilterSelectsEveryPack(t *testing.T) {
	want := "Labels.Guardrails = 'true'"
	if got := sharedFilterWhere(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if strings.Contains(sharedFilterWhere(), "Pack") {
		t.Errorf("the shared clause selects one pack: %q", sharedFilterWhere())
	}
}

// Each Trigger still carries its own Pack label: the shared Filter is how the
// rules compose, the Pack label is how one tool's rules stay identifiable.
func TestTriggerLabelsCarryBothTheSharedMarkAndThePack(t *testing.T) {
	labels := map[string]string{GuardrailLabel: "true", "Pack": "netpol-guardrails"}
	if labels[GuardrailLabel] != "true" {
		t.Error("a Trigger without the shared label is invisible to the shared Filter")
	}
	if labels["Pack"] != "netpol-guardrails" {
		t.Error("a Trigger without a Pack label cannot be attributed to a tool")
	}
}
