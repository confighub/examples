// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package version holds the cub-netpol build version. The value is overridden at
// release time via -ldflags "-X .../internal/version.Version=<v>".
package version

// Version is the cub-netpol version. "dev" for local builds.
var Version = "dev"
