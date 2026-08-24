// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cub

import (
	"context"
	"fmt"
	"strings"

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

// CreateUnit creates a new Unit (e.g. a generated ServiceMonitor) in u.SpaceID and
// writes data as its configuration. Configuration is not a field of a Unit, so it goes
// in a second call to the Unit's data endpoint; the returned Unit is the one that write
// produced, so its HeadRevisionNum names the revision holding the configuration.
func CreateUnit(ctx context.Context, c *cubapi.Client, u goclientnew.Unit, data string) (*goclientnew.Unit, error) {
	res, err := c.API.CreateUnitWithResponse(ctx, u.SpaceID, &goclientnew.CreateUnitParams{}, u)
	if cubapi.IsAPIError(err, res) {
		return nil, cubapi.InterpretErrorGeneric(err, res)
	}
	if res.JSON200 == nil {
		return nil, fmt.Errorf("unexpected response from create unit API")
	}
	// A write to a Unit answers with the operation's result; the Unit is inside it.
	created := res.JSON200.Unit
	if created == nil {
		return nil, fmt.Errorf("create unit returned no unit")
	}
	if data == "" {
		return created, nil
	}
	return PutUnitData(ctx, c, created.SpaceID, created.UnitID, data, u.LastChangeDescription)
}

// PutUnitData writes a Unit's configuration through the Unit's data endpoint,
// recording changeDesc on the revision it cuts.
func PutUnitData(ctx context.Context, c *cubapi.Client, spaceID, unitID goclientnew.UUID, data, changeDesc string) (*goclientnew.Unit, error) {
	params := &goclientnew.UploadUnitDataParams{}
	if changeDesc != "" {
		params.LastChangeDescription = &changeDesc
	}
	res, err := c.API.UploadUnitDataWithBodyWithResponse(ctx, spaceID, unitID, params,
		"application/octet-stream", strings.NewReader(data))
	if cubapi.IsAPIError(err, res) {
		return nil, cubapi.InterpretErrorGeneric(err, res)
	}
	if res.JSON200 == nil || res.JSON200.Unit == nil {
		return nil, fmt.Errorf("unexpected response from unit data API")
	}
	return res.JSON200.Unit, nil
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
