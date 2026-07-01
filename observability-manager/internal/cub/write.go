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
// ch.Description is a dry-run; a non-empty one commits.
func InvokeMutation(ctx context.Context, c *cubapi.Client, fn string, args []api.FunctionArgument, sel cubapi.Selector, ch cubapi.Change) (*cubapi.Result, error) {
	return cubapi.InvokeFunction(ctx, c, api.FunctionInvocation{FunctionName: fn, Arguments: args}, sel, ch)
}

// InvokeSetPath runs the set-path function (path + YAML value) over the Units
// matched by sel — find-or-append a document at a path (e.g. a sidecar container
// at spec.template.spec.containers.?name=<x>).
func InvokeSetPath(ctx context.Context, c *cubapi.Client, path, valueYAML string, sel cubapi.Selector, ch cubapi.Change) (*cubapi.Result, error) {
	return cubapi.InvokeFunction(ctx, c,
		api.FunctionInvocation{
			FunctionName: "set-path",
			Arguments:    []api.FunctionArgument{{Value: path}, {Value: valueYAML}},
		}, sel, ch)
}

// CreateUnit creates a new Unit (e.g. a generated ServiceMonitor) in u.SpaceID.
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

// ListInvocations returns the stored Invocations in a Space (the profile library).
func ListInvocations(ctx context.Context, c *cubapi.Client, spaceID goclientnew.UUID) ([]*goclientnew.Invocation, error) {
	res, err := c.API.ListInvocationsWithResponse(ctx, spaceID, &goclientnew.ListInvocationsParams{})
	if cubapi.IsAPIError(err, res) {
		return nil, cubapi.InterpretErrorGeneric(err, res)
	}
	var out []*goclientnew.Invocation
	if res.JSON200 != nil {
		for _, ei := range *res.JSON200 {
			if ei.Invocation != nil {
				out = append(out, ei.Invocation)
			}
		}
	}
	return out, nil
}
