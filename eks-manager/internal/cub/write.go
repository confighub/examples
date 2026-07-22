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

// InvokeMutation runs a mutating function over the Units matched by sel. An
// empty ch.Description is a dry-run — the server previews the mutations without
// writing; a non-empty one commits, recording ch.Description on the new
// revision. Units are edited, never applied to a cluster (that is a separate
// apply step).
func InvokeMutation(ctx context.Context, c *cubapi.Client, fn string, args []api.FunctionArgument, sel cubapi.Selector, ch cubapi.Change) (*cubapi.Result, error) {
	return cubapi.InvokeFunction(ctx, c,
		api.FunctionInvocation{FunctionName: fn, Arguments: args}, sel, ch)
}

// SetPath sets a scalar at a path on resources of the given type. valueFn names
// the typed setter: set-string-path, set-int-path, or set-bool-path.
func SetPath(ctx context.Context, c *cubapi.Client, valueFn, resourceType, path, value string, sel cubapi.Selector, ch cubapi.Change) (*cubapi.Result, error) {
	return InvokeMutation(ctx, c, valueFn, []api.FunctionArgument{
		{ParameterName: "resource-type", Value: resourceType},
		{ParameterName: "path", Value: path},
		{ParameterName: "attribute-value", Value: value},
	}, sel, ch)
}

// CreateUnit creates a new Unit in the Space identified by u.SpaceID. The Unit
// is created but not applied — deploying it to a cluster is a separate apply
// step. Set u.LastChangeDescription to record the reason on the initial revision.
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

// CreateAttribute registers an Attribute in the given Space. Attributes carry path registrations
// (which paths belong to which attribute name, per resource type), so they are the mechanism by
// which a client-side table becomes something server-side functions can consult.
func CreateAttribute(ctx context.Context, c *cubapi.Client, a goclientnew.Attribute) (*goclientnew.Attribute, error) {
	res, err := c.API.CreateAttributeWithResponse(ctx, a.SpaceID, &goclientnew.CreateAttributeParams{}, a)
	if cubapi.IsAPIError(err, res) {
		return nil, cubapi.InterpretErrorGeneric(err, res)
	}
	if res.JSON200 == nil {
		return nil, fmt.Errorf("unexpected response from create attribute API")
	}
	return res.JSON200, nil
}
