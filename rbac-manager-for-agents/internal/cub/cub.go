// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package cub shells out to the cub CLI. cub-rbac performs all ConfigHub I/O
// through the user's existing cub session rather than talking to the API
// directly, so this package is the single choke point for invoking cub:
// locating the binary, running commands (captured or streamed), and the
// authentication preflight every ConfigHub-touching command should call first.
package cub

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Binary is the cub executable name resolved on PATH. Overridable for tests.
var Binary = "cub"

// ErrNotFound is returned by EnsureAvailable when cub is not on PATH.
var ErrNotFound = errors.New("cub CLI not found on PATH")

// EnsureAvailable verifies the cub binary is on PATH, returning a remediation
// message if not. Call this before any cub invocation so failures are clear.
func EnsureAvailable() error {
	if _, err := exec.LookPath(Binary); err != nil {
		return fmt.Errorf("%w: install cub and ensure it is on your PATH (see https://docs.confighub.com)", ErrNotFound)
	}
	return nil
}

// Run executes `cub <args...>`, capturing stdout. On failure it returns an error
// that names the failing command and includes captured stderr, so callers can
// surface an actionable message without re-running anything.
func Run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, Binary, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s %s: %w\n%s", Binary, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

// RunStreaming executes `cub <args...>` with stdout/stderr wired straight to the
// process's streams, for commands whose human-readable output should pass
// through unmodified (e.g. interactive or progress-heavy operations).
func RunStreaming(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, Binary, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", Binary, strings.Join(args, " "), err)
	}
	return nil
}

// Preflight is the standard gate for any ConfigHub-touching command: it confirms
// cub is installed and that the session is valid against the server. It uses
// `cub auth status`, which verifies the token with the server rather than only
// reading local state, and returns a remediation message on failure.
func Preflight(ctx context.Context) error {
	if err := EnsureAvailable(); err != nil {
		return err
	}
	if _, err := Run(ctx, "auth", "status"); err != nil {
		return fmt.Errorf("not authenticated to ConfigHub — run `cub auth login` (interactive) and retry: %w", err)
	}
	return nil
}
