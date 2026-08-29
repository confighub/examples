// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package managerkit holds what these example tools agree on. Its guardrails
// subpackage installs a policy pack.
//
// What used to live here as fleet and clikit is now in the SDK, where any tool
// can import it: github.com/confighub/sdk/core/cubapi reads a fleet
// (SnapshotLoader, MemoizedClient), and github.com/confighub/sdk/cliutil is the
// command-line surface (QueryFlags and its Space-label shorthands, the output
// and table helpers). What stayed is what is a convention of these tools rather
// than a ConfigHub primitive.
package managerkit

// CommonSpace is the Space these tools put whatever an organization keeps in
// common: the guardrail Triggers, the Filter that selects them, the profile
// library, the Attributes a validating function reads.
//
// One Space rather than one per tool. A Space has a single TriggerFilterID, so
// per-tool policy Spaces cannot compose -- the first pack installed would claim
// a Space and every later one would find it taken. Profiles have no such
// constraint, but an operator should still have one place to look rather than
// one per tool they happen to have installed.
//
// Every command that writes here takes a flag to put it elsewhere.
const CommonSpace = "common"
