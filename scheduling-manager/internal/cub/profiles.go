// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cub

import (
	"context"

	"github.com/confighub/sdk/core/cubapi"
	goclientnew "github.com/confighub/sdk/core/openapi/goclient-new"
)

// ListInvocations returns the stored Invocations in a Space (the profile library),
// unwrapping the ExtendedInvocation envelope.
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
