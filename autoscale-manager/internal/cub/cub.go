// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package cub is cub-autoscale's gateway to ConfigHub: it builds an authenticated
// API client from the ambient cub session and exposes the authentication
// preflight every ConfigHub-touching command runs first.
package cub

import (
	"context"
	"fmt"
	"sync"

	"github.com/confighub/sdk/core/cubapi"
)

func Client(ctx context.Context) (*cubapi.Client, error) {
	clientOnce.Do(func() {
		client, clientErr = cubapi.ResolveClient(ctx, cubapi.ClientOptions{UserAgent: "cub-autoscale"})
	})
	return client, clientErr
}

var (
	clientOnce sync.Once
	client     *cubapi.Client
	clientErr  error
)

func Preflight(ctx context.Context) (*cubapi.Client, error) {
	c, err := Client(ctx)
	if err != nil {
		return nil, fmt.Errorf("not authenticated to ConfigHub — run `cub auth login` (interactive) and retry: %w", err)
	}
	if _, err := c.VerifyAuth(ctx); err != nil {
		return nil, fmt.Errorf("not authenticated to ConfigHub — run `cub auth login` (interactive) and retry: %w", err)
	}
	return c, nil
}
