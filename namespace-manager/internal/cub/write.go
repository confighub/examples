// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cub

import (
	"bytes"
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

// BulkCloneUnits clones the Units matched by srcWhere into the Space(s) matched
// by destWhere, via the BulkCreateUnits API, applying a no-op merge patch (the
// clones are re-homed afterwards with set-namespace). Unlike a function
// invocation, unit creation has no server-side dry-run, so callers preview the
// plan client-side and only call this on commit. allowExists makes re-runs
// idempotent (an already-present clone is reported, not re-created). The clones
// are created but not applied to a cluster.
func BulkCloneUnits(ctx context.Context, c *cubapi.Client, srcWhere, destWhere string, allowExists bool) ([]goclientnew.UnitCreateOrUpdateResponse, error) {
	include := "SpaceID,TargetID,UpstreamUnitID"
	params := &goclientnew.BulkCreateUnitsParams{
		Where:      &srcWhere,
		WhereSpace: &destWhere,
		Include:    &include,
	}
	if allowExists {
		t := "true"
		params.AllowExists = &t
	}
	res, err := c.API.BulkCreateUnitsWithBodyWithResponse(ctx, params,
		"application/merge-patch+json", bytes.NewReader([]byte("null")))
	if cubapi.IsAPIError(err, res) {
		return nil, cubapi.InterpretErrorGeneric(err, res)
	}
	switch {
	case res.JSON200 != nil:
		return *res.JSON200, nil
	case res.JSON207 != nil:
		return *res.JSON207, nil
	}
	return nil, fmt.Errorf("unexpected response from bulk create units API")
}
