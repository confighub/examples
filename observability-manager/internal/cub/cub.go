// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package cub is cub-observability's gateway to ConfigHub: it builds an
// authenticated API client from the ambient cub session and exposes the
// authentication preflight every ConfigHub-touching command runs first.
package cub

import (
	"context"
	"fmt"
	"sync"

	"github.com/confighub/sdk/core/cubapi"
)

// Client returns a memoized, authenticated ConfigHub API client.
func Client(ctx context.Context) (*cubapi.Client, error) {
	clientOnce.Do(func() {
		client, clientErr = cubapi.ResolveClient(ctx, cubapi.ClientOptions{UserAgent: "cub-observability"})
	})
	return client, clientErr
}

var (
	clientOnce sync.Once
	client     *cubapi.Client
	clientErr  error
)

// Preflight builds the client and verifies the session against the server.
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
