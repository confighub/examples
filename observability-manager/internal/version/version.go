// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package version holds the cub-observability build version. Overridden at
// release time via -ldflags "-X .../internal/version.Version=<v>".
package version

// Version is the cub-observability version. "dev" for local builds.
var Version = "dev"
