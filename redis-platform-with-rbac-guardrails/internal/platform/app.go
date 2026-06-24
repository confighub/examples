// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package platform

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
)

func Load(path string) (Model, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Model{}, fmt.Errorf("read fixture: %w", err)
	}
	var model Model
	if err := json.Unmarshal(data, &model); err != nil {
		return Model{}, fmt.Errorf("parse fixture: %w", err)
	}
	return model, nil
}

func Explain(model Model, outputDir string) string {
	return fmt.Sprintf(`This is a read-only setup plan for redis-platform-with-rbac-guardrails.
No ConfigHub state will be mutated.
No live infrastructure will be mutated.

The app models:
- Component: %s
- Variants: %s
- Pieces: %s

When run normally, it writes local files under:
- %s

Those files show:
- the payments component map
- an RBAC snapshot
- who can get Secrets in payments-prod
- one visible RBAC finding
- a dry-run hardening edit
`, model.Component, joinVariantNames(model.Variants), joinPieceNames(model.Pieces), outputDir)
}

func ExplainJSON(model Model, fixturePath string) map[string]any {
	return map[string]any{
		"example_name":       "redis-platform-with-rbac-guardrails",
		"component":          model.Component,
		"mutates":            false,
		"mutates_confighub":  false,
		"mutates_live_infra": false,
		"fixture":            fixturePath,
		"app":                "payments-rbac",
		"agent_facing":       true,
		"outputs":            []string{"sample-output/component-map.json", "sample-output/snapshot.json", "sample-output/who-can-get-secrets-prod-us.json", "sample-output/findings.json", "sample-output/proposed-edit.json"},
	}
}

func ComponentMap(model Model) map[string]any {
	return map[string]any{
		"component":   model.Component,
		"description": model.Description,
		"variants":    model.Variants,
		"pieces":      model.Pieces,
	}
}

func Snapshot(model Model) map[string]any {
	byPiece := map[string]int{}
	namespaces := []string{}
	seenNamespaces := map[string]bool{}
	for _, unit := range model.Units {
		byPiece[unit.Piece]++
		if unit.Namespace != "" && !seenNamespaces[unit.Namespace] {
			seenNamespaces[unit.Namespace] = true
			namespaces = append(namespaces, unit.Namespace)
		}
	}
	slices.Sort(namespaces)
	unitsByPiece := make([]map[string]any, 0, len(model.Pieces))
	for _, piece := range model.Pieces {
		unitsByPiece = append(unitsByPiece, map[string]any{"piece": piece.Name, "count": byPiece[piece.Name]})
	}
	return map[string]any{
		"component":    model.Component,
		"variantCount": len(model.Variants),
		"pieceCount":   len(model.Pieces),
		"unitCount":    len(model.Units),
		"unitsByPiece": unitsByPiece,
		"namespaces":   namespaces,
	}
}

func Query(model Model, name string) (AccessQuery, bool) {
	for _, query := range model.Queries {
		if query.Name == name {
			return query, true
		}
	}
	return AccessQuery{}, false
}

func Findings(model Model) map[string]any {
	return map[string]any{"component": model.Component, "findings": model.Findings}
}

func ProposedEditPlan(model Model) map[string]any {
	return map[string]any{"component": model.Component, "proposedEdits": model.ProposedEdits}
}

func Verify(model Model) error {
	if model.Component != "payments-platform" {
		return fmt.Errorf("unexpected component %q", model.Component)
	}
	if len(model.Variants) != 5 {
		return fmt.Errorf("expected 5 variants, got %d", len(model.Variants))
	}
	if len(model.Units) != 7 {
		return fmt.Errorf("expected 7 units, got %d", len(model.Units))
	}
	query, ok := Query(model, "who-can-get-secrets-prod-us")
	if !ok {
		return fmt.Errorf("missing who-can query")
	}
	if len(query.Grants) != 2 {
		return fmt.Errorf("expected 2 secret grants, got %d", len(query.Grants))
	}
	if len(model.Findings) != 1 || model.Findings[0].ID != "payments-api-secret-list-prod-us" {
		return fmt.Errorf("expected payments API secret-list finding")
	}
	if len(model.ProposedEdits) != 1 || model.ProposedEdits[0].Mode != "dry-run" {
		return fmt.Errorf("expected one dry-run proposed edit")
	}
	return nil
}

func joinVariantNames(variants []Variant) string {
	names := make([]string, len(variants))
	for i, variant := range variants {
		names[i] = variant.Name
	}
	return joinNames(names)
}

func joinPieceNames(pieces []Piece) string {
	names := make([]string, len(pieces))
	for i, piece := range pieces {
		names[i] = piece.Name
	}
	return joinNames(names)
}

func joinNames(names []string) string {
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	default:
		return fmt.Sprintf("%s, and %s", joinComma(names[:len(names)-1]), names[len(names)-1])
	}
}

func joinComma(names []string) string {
	out := ""
	for i, name := range names {
		if i > 0 {
			out += ", "
		}
		out += name
	}
	return out
}
