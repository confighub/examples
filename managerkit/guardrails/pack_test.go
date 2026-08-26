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

// Two packs can share a policy Space, so each Filter must select on the pack
// label. A clause that selected the Space's Triggers would wire every Space of
// one pack to the other pack's rules as well.
func TestPackFilterWhereSelectsOnlyThisPack(t *testing.T) {
	netpol := Pack{Label: "netpol-guardrails"}
	workload := Pack{Label: "workload-guardrails"}

	if got, want := netpol.filterWhere(), "Labels.Pack = 'netpol-guardrails'"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if netpol.filterWhere() == workload.filterWhere() {
		t.Error("two packs produced the same Filter clause")
	}
	if strings.Contains(netpol.filterWhere(), "SpaceID") {
		t.Errorf("the clause keys on the Space, not the pack: %q", netpol.filterWhere())
	}
}
