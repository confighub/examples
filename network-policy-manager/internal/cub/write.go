// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cub

import (
	"context"
	"fmt"

	"github.com/confighub/sdk/core/cubapi"
	api "github.com/confighub/sdk/core/function/api"
	goclientnew "github.com/confighub/sdk/core/openapi/goclient-new"
)

// CreateUnit creates a new Unit (e.g. a generated NetworkPolicy) in the Space
// identified by u.SpaceID. The unit is created but not applied — deploying it to
// a cluster is a separate apply step. Set u.LastChangeDescription to record the
// reason on the initial revision.
func CreateUnit(ctx context.Context, c *cubapi.Client, u goclientnew.Unit) (*goclientnew.Unit, error) {
	res, err := c.API.CreateUnitWithResponse(ctx, u.SpaceID, &goclientnew.CreateUnitParams{}, u)
	if cubapi.IsAPIError(err, res) {
		return nil, cubapi.InterpretErrorGeneric(err, res)
	}
	return res.JSON200, nil
}

// MutateUnitYQ runs the mutating set-yq function over a single Unit (identified
// by space ID + slug) with the given yq expression. An empty Change.Description
// is a dry-run (the server previews mutations without writing); a non-empty one
// commits. The returned Result carries the per-Unit mutation diff.
func MutateUnitYQ(ctx context.Context, c *cubapi.Client, spaceID goclientnew.UUID, unitSlug, yqExpr string, ch cubapi.Change) (*cubapi.Result, error) {
	return cubapi.InvokeFunction(ctx, c,
		api.FunctionInvocation{
			FunctionName: "set-yq",
			Arguments:    []api.FunctionArgument{{ParameterName: "yq-expression", Value: yqExpr}},
		},
		cubapi.Selector{Where: fmt.Sprintf("SpaceID = '%s' AND Slug = '%s'", spaceID.String(), unitSlug)},
		ch)
}
