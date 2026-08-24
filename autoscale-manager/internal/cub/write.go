// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cub

import (
	"context"
	"fmt"
	"net/http"
	"strings"

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

// rawBodyErr reports failure for an endpoint whose success body is the configuration
// itself. cubapi.IsAPIError cannot be used for those: it treats a nil JSON200 field as a
// failure, and a raw-body response has no JSON200 to be non-nil.
func rawBodyErr(err error, resp interface {
	StatusCode() int
	Status() string
}) error {
	if err != nil {
		return err
	}
	if resp == nil {
		return fmt.Errorf("no response from server")
	}
	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("request failed: %s", resp.Status())
	}
	return nil
}

// GetUnitData returns a Unit's config data as text. Configuration is not a field of a
// Unit — it is read through the Unit's own data endpoint.
func GetUnitData(ctx context.Context, c *cubapi.Client, ref UnitRef) (string, error) {
	res, err := c.API.DownloadUnitDataWithResponse(ctx, ref.SpaceID, ref.UnitID)
	if err := rawBodyErr(err, res); err != nil {
		return "", fmt.Errorf("get data for unit %s/%s: %w", ref.SpaceSlug, ref.UnitSlug, err)
	}
	return string(res.Body), nil
}

// PutUnitData writes new config data to a Unit through the Unit's data endpoint,
// recording changeDesc on the new revision. The Unit is edited, not applied.
func PutUnitData(ctx context.Context, c *cubapi.Client, ref UnitRef, data, changeDesc string) error {
	params := &goclientnew.UploadUnitDataParams{}
	if changeDesc != "" {
		params.LastChangeDescription = &changeDesc
	}
	res, err := c.API.UploadUnitDataWithBodyWithResponse(ctx, ref.SpaceID, ref.UnitID, params,
		"application/octet-stream", strings.NewReader(data))
	if cubapi.IsAPIError(err, res) {
		return cubapi.InterpretErrorGeneric(err, res)
	}
	return nil
}
