// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package fleet reads a fleet-wide view of ConfigHub config: the Units a filter
// selects, the resources inside them, and the Unit / Space / Target metadata
// that says where each one lives.
//
// It is the part of these example tools that is not about any particular kind of
// config. A tool supplies the resource types it cares about and a function that
// turns one resource into its own model; everything between -- scoping, paging
// the two queries, the join, the cluster key -- is here.
package fleet

import (
	"context"
	"fmt"
	"sync"

	"github.com/confighub/sdk/core/cubapi"
)

// Session is one tool's authenticated connection to ConfigHub, built on first
// use from the cub plugin environment (CUB_SERVER / CUB_TOKEN) or the local
// ~/.confighub session, and reused thereafter.
//
// The zero value is usable; set UserAgent so the server can tell the tools
// apart. A Session must not be copied after first use.
type Session struct {
	// UserAgent identifies the tool to the server, e.g. "cub-netpol".
	UserAgent string

	once   sync.Once
	client *cubapi.Client
	err    error
}

// Client returns the memoized client. Building it performs no network I/O; use
// Preflight to verify the session against the server.
func (s *Session) Client(ctx context.Context) (*cubapi.Client, error) {
	s.once.Do(func() {
		s.client, s.err = cubapi.ResolveClient(ctx, cubapi.ClientOptions{UserAgent: s.UserAgent})
	})
	return s.client, s.err
}

// Preflight is the standard gate for any ConfigHub-touching command: it builds
// the client and verifies the session against the server, rather than only
// reading local state, so an expired token is reported here instead of surfacing
// as an unrelated failure further in. It returns the ready client, or an error
// carrying the remediation.
func (s *Session) Preflight(ctx context.Context) (*cubapi.Client, error) {
	c, err := s.Client(ctx)
	if err != nil {
		return nil, notAuthenticated(err)
	}
	if _, err := c.VerifyAuth(ctx); err != nil {
		return nil, notAuthenticated(err)
	}
	return c, nil
}

func notAuthenticated(err error) error {
	return fmt.Errorf("not authenticated to ConfigHub — run `cub auth login` (interactive) and retry: %w", err)
}
