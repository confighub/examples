// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package managerkit holds what the sibling packages agree on. Its subpackages
// are the library proper: fleet reads a fleet, clikit is the command-line
// surface, guardrails installs a policy pack.
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
