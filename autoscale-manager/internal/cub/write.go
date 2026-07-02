// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cub

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/confighub/sdk/core/cubapi"
	api "github.com/confighub/sdk/core/function/api"
	goclientnew "github.com/confighub/sdk/core/openapi/goclient-new"
)

// MutateUnitYQ runs the mutating set-yq function over the Units matched by sel.
// An empty ch.Description is a dry-run; a non-empty one commits. Units are edited,
// never applied to a cluster.
func MutateUnitYQ(ctx context.Context, c *cubapi.Client, yqExpr string, sel cubapi.Selector, ch cubapi.Change) (*cubapi.Result, error) {
	return cubapi.InvokeFunction(ctx, c,
		api.FunctionInvocation{
			FunctionName: "set-yq",
			Arguments:    []api.FunctionArgument{{ParameterName: "yq-expression", Value: yqExpr}},
		}, sel, ch)
}

// UnitRef resolves a <space>/<unit> to its Space and Unit IDs.
type UnitRef struct {
	SpaceID   goclientnew.UUID
	SpaceSlug string
	UnitID    goclientnew.UUID
	UnitSlug  string
}

// ResolveUnit resolves a Space slug + Unit slug to their IDs.
func ResolveUnit(ctx context.Context, c *cubapi.Client, spaceSlug, unitSlug string) (UnitRef, error) {
	sp, err := cubapi.ResolveSpace(ctx, c, spaceSlug)
	if err != nil {
		return UnitRef{}, fmt.Errorf("resolve space %q: %w", spaceSlug, err)
	}
	units, err := cubapi.ListUnits(ctx, c,
		cubapi.NewWhere(fmt.Sprintf("SpaceID = '%s' AND Slug = '%s'", sp.SpaceID.String(), unitSlug)),
		cubapi.ListOpts{Select: "Slug,SpaceID,UnitID"})
	if err != nil {
		return UnitRef{}, err
	}
	for _, eu := range units {
		if eu.Unit != nil && eu.Unit.Slug == unitSlug {
			return UnitRef{SpaceID: sp.SpaceID, SpaceSlug: spaceSlug, UnitID: eu.Unit.UnitID, UnitSlug: unitSlug}, nil
		}
	}
	return UnitRef{}, fmt.Errorf("unit %q not found in space %q", unitSlug, spaceSlug)
}

// GetUnitData returns a Unit's raw config data (base64-encoded YAML), fetched
// with the single-Unit endpoint so Data is populated.
func GetUnitData(ctx context.Context, c *cubapi.Client, ref UnitRef) (string, error) {
	res, err := c.API.GetUnitWithResponse(ctx, ref.SpaceID, ref.UnitID, &goclientnew.GetUnitParams{})
	if cubapi.IsAPIError(err, res) {
		return "", cubapi.InterpretErrorGeneric(err, res)
	}
	if res.JSON200 == nil || res.JSON200.Unit == nil {
		return "", fmt.Errorf("unit %s/%s not found", ref.SpaceSlug, ref.UnitSlug)
	}
	return res.JSON200.Unit.Data, nil
}

// PatchUnitData writes new config data (base64-encoded YAML) to a Unit via a JSON
// merge-patch, recording changeDesc on the new revision. The Unit is edited, not
// applied.
func PatchUnitData(ctx context.Context, c *cubapi.Client, ref UnitRef, base64Data, changeDesc string) error {
	patch := map[string]any{
		"Data":                  base64Data,
		"LastChangeDescription": changeDesc,
	}
	body, err := json.Marshal(patch)
	if err != nil {
		return err
	}
	res, err := c.API.PatchUnitWithBodyWithResponse(ctx, ref.SpaceID, ref.UnitID,
		&goclientnew.PatchUnitParams{}, "application/merge-patch+json", bytes.NewReader(body))
	if cubapi.IsAPIError(err, res) {
		return cubapi.InterpretErrorGeneric(err, res)
	}
	return nil
}
