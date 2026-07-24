// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package cub is k8s-to-score's gateway to ConfigHub. It builds an
// authenticated API client from the ambient cub session — the same session
// `cub auth login` creates — and reads Unit config data out of a Space.
//
// All ConfigHub I/O goes through github.com/confighub/sdk/core/cubapi; this
// tool never shells out to the cub CLI, and it never mutates: every call here
// is a read.
package cub

import (
	"context"
	"encoding/base64"
	"fmt"
	"sort"
	"sync"

	"github.com/confighub/sdk/core/cubapi"
)

// Client returns a memoized, authenticated ConfigHub API client. Building the
// client performs no network I/O; use Preflight to verify the session.
func Client(ctx context.Context) (*cubapi.Client, error) {
	clientOnce.Do(func() {
		client, clientErr = cubapi.ResolveClient(ctx, cubapi.ClientOptions{UserAgent: "k8s-to-score"})
	})
	return client, clientErr
}

var (
	clientOnce sync.Once
	client     *cubapi.Client
	clientErr  error
)

// Preflight builds the client and verifies the session against the server —
// hitting the API rather than only reading local state, so an expired session
// fails here with a remediation message instead of midway through a conversion.
func Preflight(ctx context.Context) (*cubapi.Client, error) {
	c, err := Client(ctx)
	if err != nil {
		return nil, fmt.Errorf("not authenticated to ConfigHub — run `cub auth login` and retry: %w", err)
	}
	if _, err := c.VerifyAuth(ctx); err != nil {
		return nil, fmt.Errorf("not authenticated to ConfigHub — run `cub auth login` and retry: %w", err)
	}
	return c, nil
}

// Unit is the slice of a ConfigHub Unit this tool needs: its identity and its
// config data.
type Unit struct {
	Slug string
	Data []byte
}

// ListUnits returns the Units of a Space, optionally narrowed by a `--where`
// clause, sorted by slug so output is deterministic across runs.
//
// Units whose data is empty are dropped: they carry no Kubernetes resource to
// convert.
func ListUnits(ctx context.Context, c *cubapi.Client, space, where string) ([]Unit, error) {
	sp, err := cubapi.ResolveSpace(ctx, c, space)
	if err != nil {
		return nil, fmt.Errorf("space %q: %w", space, err)
	}

	w := cubapi.NewWhere(where).SpaceID(sp.SpaceID)
	// An empty Select asks for all fields, which is what carries Unit.Data —
	// the default field set omits it.
	extended, err := cubapi.ListUnits(ctx, c, w, cubapi.ListOpts{})
	if err != nil {
		return nil, fmt.Errorf("listing units in space %q: %w", space, err)
	}

	units := make([]Unit, 0, len(extended))
	for _, eu := range extended {
		if eu == nil || eu.Unit == nil || eu.Unit.Data == "" {
			continue
		}
		// The API carries Unit config data base64-encoded on the wire.
		data, err := base64.StdEncoding.DecodeString(eu.Unit.Data)
		if err != nil {
			return nil, fmt.Errorf("unit %s: decoding config data: %w", eu.Unit.Slug, err)
		}
		units = append(units, Unit{Slug: eu.Unit.Slug, Data: data})
	}
	sort.Slice(units, func(i, j int) bool { return units[i].Slug < units[j].Slug })
	return units, nil
}
