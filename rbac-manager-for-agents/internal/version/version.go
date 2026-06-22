// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package version holds the cub-rbac build version. The value is overridden at
// release time via -ldflags "-X .../internal/version.Version=<v>".
package version

// Version is the cub-rbac version. "dev" for local builds.
var Version = "dev"
