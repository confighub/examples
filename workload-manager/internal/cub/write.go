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

// InvokeMutation runs a mutating function over the Units matched by sel. An empty
// ch.Description is a dry-run — the server previews the mutations without writing;
// a non-empty one commits, recording ch.Description on the new revision. The
// returned Result carries the per-Unit mutation diff. Units are edited, never
// applied to a cluster (that is a separate apply step).
func InvokeMutation(ctx context.Context, c *cubapi.Client, fn string, args []api.FunctionArgument, sel cubapi.Selector, ch cubapi.Change) (*cubapi.Result, error) {
	return cubapi.InvokeFunction(ctx, c,
		api.FunctionInvocation{FunctionName: fn, Arguments: args}, sel, ch)
}

// MutateUnitYQ runs the mutating set-yq function over the Units matched by sel
// with the given yq expression. Dry-run/commit follows ch.Description as above.
func MutateUnitYQ(ctx context.Context, c *cubapi.Client, yqExpr string, sel cubapi.Selector, ch cubapi.Change) (*cubapi.Result, error) {
	return cubapi.InvokeFunction(ctx, c,
		api.FunctionInvocation{
			FunctionName: "set-yq",
			Arguments:    []api.FunctionArgument{{ParameterName: "yq-expression", Value: yqExpr}},
		}, sel, ch)
}

// CreateUnit creates a new Unit (e.g. a generated PodDisruptionBudget) in the
// Space identified by u.SpaceID. The unit is created but not applied — deploying
// it to a cluster is a separate apply step. Set u.LastChangeDescription to record
// the reason on the initial revision.
func CreateUnit(ctx context.Context, c *cubapi.Client, u goclientnew.Unit) (*goclientnew.Unit, error) {
	res, err := c.API.CreateUnitWithResponse(ctx, u.SpaceID, &goclientnew.CreateUnitParams{}, u)
	if cubapi.IsAPIError(err, res) {
		return nil, cubapi.InterpretErrorGeneric(err, res)
	}
	if res.JSON200 == nil {
		return nil, fmt.Errorf("unexpected response from create unit API")
	}
	return res.JSON200, nil
}
