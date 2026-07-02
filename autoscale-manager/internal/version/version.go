// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package version holds the cub-autoscale build version. Overridden at release
// time via -ldflags "-X .../internal/version.Version=<v>".
package version

// Version is the cub-autoscale version. "dev" for local builds.
var Version = "dev"
