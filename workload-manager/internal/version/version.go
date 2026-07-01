// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package version holds the cub-workload build version. The value is overridden
// at release time via -ldflags "-X .../internal/version.Version=<v>".
package version

// Version is the cub-workload version. "dev" for local builds.
var Version = "dev"
