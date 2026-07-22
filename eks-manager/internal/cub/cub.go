// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package cub is cub-eks's gateway to ConfigHub. It builds an authenticated API
// client from the ambient cub session — the same session `cub auth login`
// creates, or the credentials cub passes to a plugin via the environment — and
// exposes the authentication preflight every ConfigHub-touching command runs
// first.
//
// All ConfigHub I/O goes through the ConfigHub API directly via the
// github.com/confighub/sdk/core/cubapi package.
package cub

import (
	"context"
	"fmt"
	"sync"

	"github.com/confighub/sdk/core/cubapi"
)

// Client returns a memoized, authenticated ConfigHub API client, built on first
// use from the cub plugin environment (CUB_SERVER / CUB_TOKEN) or the local
// ~/.confighub session. Building the client performs no network I/O; use
// Preflight to verify the session against the server.
func Client(ctx context.Context) (*cubapi.Client, error) {
	clientOnce.Do(func() {
		client, clientErr = cubapi.ResolveClient(ctx, cubapi.ClientOptions{UserAgent: "cub-eks"})
	})
	return client, clientErr
}

var (
	clientOnce sync.Once
	client     *cubapi.Client
	clientErr  error
)

// Preflight is the standard gate for any ConfigHub-touching command: it builds
// the client and verifies the session against the server (hitting GET /me, like
// `cub auth status`, rather than only reading local state). It returns the ready
// client so callers can proceed, or a remediation message on failure.
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
