// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package cub is cub-scheduling's gateway to ConfigHub: one authenticated session, built
// from the ambient cub credentials, shared by every command.
package cub

import (
	"context"

	"github.com/confighub/sdk/core/cubapi"
)

var session = cubapi.MemoizedClient{UserAgent: "cub-scheduling"}

// Client returns the memoized, authenticated ConfigHub API client.
func Client(ctx context.Context) (*cubapi.Client, error) { return session.Client(ctx) }

// Preflight is the standard gate for any ConfigHub-touching command: it verifies
// the session against the server before the command does anything else.
func Preflight(ctx context.Context) (*cubapi.Client, error) { return session.Preflight(ctx) }
