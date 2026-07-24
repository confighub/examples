// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package version carries the build version, injected at link time by the
// release build.
package version

// Version is overridden with -ldflags "-X .../internal/version.Version=x.y.z".
var Version = "dev"
