// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

// This tool's guardrail pack. The mechanism -- policy Space, Triggers, shared
// Filter, wiring each in-scope Space, and reporting the Units a Trigger marked
// -- is managerkit/guardrails'. What is this tool's is the rules.

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/confighub/sdk/core/cubapi"

	"github.com/confighub/examples/managerkit/guardrails"
	"github.com/confighub/examples/rbac-manager-for-agents/internal/cub"
)

// Guardrail CEL expressions, evaluated per resource (r aliases the resource).
const (
	celNoWildcards    = "!(r.kind in ['Role', 'ClusterRole']) || !has(r.rules) || !r.rules.exists(rule, (has(rule.verbs) && rule.verbs.exists(v, v == '*')) || (has(rule.resources) && rule.resources.exists(x, x == '*')) || (has(rule.apiGroups) && rule.apiGroups.exists(g, g == '*')))"
	celNoEscalation   = "!(r.kind in ['Role', 'ClusterRole']) || !has(r.rules) || !r.rules.exists(rule, has(rule.verbs) && rule.verbs.exists(v, v in ['escalate', 'bind', 'impersonate']))"
	celNoClusterAdmin = "r.kind != 'ClusterRoleBinding' || r.roleRef.name != 'cluster-admin'"
)

var pack = guardrails.Pack{
	Label: "rbac-guardrails",
	Rules: []guardrails.Rule{
		{Slug: "no-rbac-wildcards", Description: "Warns on Roles/ClusterRoles with wildcard verbs, resources, or apiGroups. Fix: enumerate the specific verbs/resources the role needs.", Expression: celNoWildcards},
		{Slug: "no-rbac-privilege-escalation", Description: "Warns on Roles/ClusterRoles granting escalate, bind, or impersonate. Fix: remove these verbs; they allow privilege escalation.", Expression: celNoEscalation},
		{Slug: "no-cluster-admin-binding", Description: "Warns on ClusterRoleBindings to cluster-admin. Fix: bind a scoped role instead.", Expression: celNoClusterAdmin},
	},
}

func preflight(ctx context.Context) (*cubapi.Client, error) { return cub.Preflight(ctx) }

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
	cmd.AddCommand(pack.InstallCmd(preflight), pack.StatusCmd(preflight))
	return cmd
}
