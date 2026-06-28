// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/confighub/examples/rbac-manager-for-agents/internal/cub"
	"github.com/confighub/examples/rbac-manager-for-agents/internal/rbac"
	"github.com/confighub/examples/rbac-manager-for-agents/internal/snapshot"
)

type grantRow struct {
	Cluster     string `json:"cluster"`
	Subject     string `json:"subject"`
	Scope       string `json:"scope,omitempty"` // "" = cluster-wide
	RoleRef     string `json:"roleRef"`
	RoleRefKind string `json:"roleRefKind"`
	Builtin     bool   `json:"builtin,omitempty"`
	Binding     string `json:"binding"`
	Unit        string `json:"unit"`
}

func newWhoCanCmd() *cobra.Command {
	var output, apiGroup, namespace, name string
	var scope scopeFlags
	cmd := &cobra.Command{
		Use:   "who-can <verb> <resource>",
		Short: "Find every subject that can perform an action, fleet-wide",
		Long: `who-can answers "who can VERB RESOURCE?" across the fleet, with provenance:
the cluster, the granting binding and role, and the namespace scope.

RESOURCE may include a subresource (e.g. pods/log). Use --api-group for
non-core groups (e.g. --api-group apps for deployments), --namespace to restrict
to grants effective in a namespace, and --name to honor resourceNames.

Only cluster-admin is credited among Kubernetes builtins (its rules are known);
bindings to admin/edit/view/system:* are reported by 'findings', not here.`,
		Example: `  cub-rbac who-can get secrets
  cub-rbac who-can create pods/exec --namespace payments
  cub-rbac who-can update deployments --api-group apps`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cub.Preflight(cmd.Context())
			if err != nil {
				return err
			}
			snap, err := snapshot.Load(cmd.Context(), client, scope.scope())
			if err != nil {
				return err
			}
			query := rbac.AccessQuery{
				Verb:      args[0],
				Resource:  args[1],
				APIGroup:  apiGroup,
				Namespace: namespace,
				Name:      name,
			}
			rows := grantRows(rbac.WhoCan(snap.Clusters, query))
			if output == outputTable {
				printGrantTable(cmd, rows)
				return nil
			}
			return printJSON(cmd.OutOrStdout(), rows)
		},
	}
	addOutputFlag(cmd, &output)
	addScopeFlags(cmd, &scope)
	cmd.Flags().StringVar(&apiGroup, "api-group", "", "API group of the resource (default core)")
	cmd.Flags().StringVar(&namespace, "namespace", "", "restrict to grants effective in this namespace")
	cmd.Flags().StringVar(&name, "name", "", "specific object name (honors resourceNames)")
	return cmd
}

func grantRows(grants []rbac.Grant) []grantRow {
	rows := make([]grantRow, 0, len(grants))
	for _, g := range grants {
		rows = append(rows, grantRow{
			Cluster:     g.Cluster,
			Subject:     g.SubjectKey,
			Scope:       g.Scope,
			RoleRef:     g.RoleRefName,
			RoleRefKind: g.Binding.RoleRef.Kind,
			Builtin:     g.ViaBuiltinRole,
			Binding:     g.Binding.Name,
			Unit:        g.Binding.Origin.UnitSlug,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Cluster != rows[j].Cluster {
			return rows[i].Cluster < rows[j].Cluster
		}
		return rows[i].Subject < rows[j].Subject
	})
	return rows
}

func printGrantTable(cmd *cobra.Command, rows []grantRow) {
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "CLUSTER\tSUBJECT\tSCOPE\tROLE\tBINDING\tUNIT")
	for _, r := range rows {
		scope := r.Scope
		if scope == "" {
			scope = "cluster-wide"
		}
		role := r.RoleRef
		if r.Builtin {
			role += " (builtin)"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", r.Cluster, r.Subject, scope, role, r.Binding, r.Unit)
	}
	_ = tw.Flush()
	fprintln(cmd.OutOrStdout(), fmt.Sprintf("\n%d grants", len(rows)))
}

// --- access (inverse query) ---

type subjectGrantRow struct {
	Cluster     string `json:"cluster"`
	RoleRef     string `json:"roleRef"`
	RoleRefKind string `json:"roleRefKind"`
	Resolved    bool   `json:"resolved"` // whether the role exists in the snapshot
	Scope       string `json:"scope,omitempty"`
	Binding     string `json:"binding"`
	Unit        string `json:"unit"`
}

func newAccessCmd() *cobra.Command {
	var output string
	var scope scopeFlags
	cmd := &cobra.Command{
		Use:   "access <subject>",
		Short: "List every role a subject holds, fleet-wide (inverse of who-can)",
		Long: `access lists every role a subject holds across the fleet.

SUBJECT is "Kind:Name", or "ServiceAccount:namespace/name" for a ServiceAccount:
  User:alice@example.com
  Group:oidc:developers
  ServiceAccount:apps/ci-deployer`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			subject, err := parseSubjectRef(args[0])
			if err != nil {
				return err
			}
			client, err := cub.Preflight(cmd.Context())
			if err != nil {
				return err
			}
			snap, err := snapshot.Load(cmd.Context(), client, scope.scope())
			if err != nil {
				return err
			}
			rows := subjectGrantRows(rbac.SubjectAccess(snap.Clusters, subject))
			if output == outputTable {
				printSubjectGrantTable(cmd, rows)
				return nil
			}
			return printJSON(cmd.OutOrStdout(), rows)
		},
	}
	addOutputFlag(cmd, &output)
	addScopeFlags(cmd, &scope)
	return cmd
}

// parseSubjectRef parses "Kind:Name" (or "ServiceAccount:ns/name").
func parseSubjectRef(s string) (rbac.SubjectRef, error) {
	kindStr, rest, ok := strings.Cut(s, ":")
	if !ok || kindStr == "" || rest == "" {
		return rbac.SubjectRef{}, fmt.Errorf("invalid subject %q: expected Kind:Name (e.g. User:alice, Group:devs, ServiceAccount:ns/name)", s)
	}
	kind, err := normalizeSubjectKind(kindStr)
	if err != nil {
		return rbac.SubjectRef{}, err
	}
	ref := rbac.SubjectRef{Kind: kind}
	if kind == "ServiceAccount" {
		ns, nm, ok := strings.Cut(rest, "/")
		if !ok || ns == "" || nm == "" {
			return rbac.SubjectRef{}, fmt.Errorf("invalid ServiceAccount subject %q: expected ServiceAccount:namespace/name", s)
		}
		ref.Namespace, ref.Name = ns, nm
	} else {
		ref.Name = rest
	}
	return ref, nil
}

func normalizeSubjectKind(s string) (string, error) {
	switch strings.ToLower(s) {
	case "user":
		return "User", nil
	case "group":
		return "Group", nil
	case "serviceaccount", "sa":
		return "ServiceAccount", nil
	default:
		return "", fmt.Errorf("unknown subject kind %q: use User, Group, or ServiceAccount", s)
	}
}

func subjectGrantRows(grants []rbac.SubjectGrant) []subjectGrantRow {
	rows := make([]subjectGrantRow, 0, len(grants))
	for _, g := range grants {
		rows = append(rows, subjectGrantRow{
			Cluster:     g.Cluster,
			RoleRef:     g.RoleRefName,
			RoleRefKind: g.Binding.RoleRef.Kind,
			Resolved:    g.Role != nil,
			Scope:       g.Scope,
			Binding:     g.Binding.Name,
			Unit:        g.Binding.Origin.UnitSlug,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Cluster != rows[j].Cluster {
			return rows[i].Cluster < rows[j].Cluster
		}
		return rows[i].RoleRef < rows[j].RoleRef
	})
	return rows
}

func printSubjectGrantTable(cmd *cobra.Command, rows []subjectGrantRow) {
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "CLUSTER\tROLE\tKIND\tSCOPE\tBINDING\tUNIT")
	for _, r := range rows {
		scope := r.Scope
		if scope == "" {
			scope = "cluster-wide"
		}
		role := r.RoleRef
		if !r.Resolved {
			role += " (unresolved)"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", r.Cluster, role, r.RoleRefKind, scope, r.Binding, r.Unit)
	}
	_ = tw.Flush()
	fprintln(cmd.OutOrStdout(), fmt.Sprintf("\n%d grants", len(rows)))
}
