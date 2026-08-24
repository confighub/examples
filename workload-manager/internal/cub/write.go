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
// Space identified by u.SpaceID and writes data as its configuration. Configuration
// is not a field of a Unit, so it goes in a second call to the Unit's data endpoint;
// the returned Unit is the one that write produced, so its HeadRevisionNum names the
// revision holding the configuration. The unit is created but not applied — deploying
// it to a cluster is a separate apply step. Set u.LastChangeDescription to record
// the reason on the revisions.
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
