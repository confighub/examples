// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Command cub-eks manages AWS EKS clusters as data in ConfigHub, using
// Crossplane managed resources as the substrate, in a form usable by an AI agent
// in a terminal: fleet inventory of clusters, node capacity and addons; a
// version matrix; a change-impact plan that grades a pending edit as in-place,
// rolling, or a replacement; and (in later milestones) guardrailed
// config-as-data lifecycle operations.
//
// The command shape is eksctl's (create cluster, upgrade cluster, scale
// nodegroup); the substrate is not. eksctl emits CloudFormation and holds its
// state there; Crossplane managed resources are continuously reconciled desired
// state, and ConfigHub makes that state a versioned, fleet-queryable,
// promotable, gated source of record.
//
// It can be run standalone (cub-eks ...) or as a cub plugin (cub eks ...). All
// ConfigHub I/O goes through the ConfigHub API directly via the
// github.com/confighub/sdk/core/cubapi package, using the ambient cub session.
package main

import (
	"fmt"
	"os"

	"github.com/confighub/sdk/core/plugin"

	"github.com/confighub/examples/eks-manager/internal/cli"
	"github.com/confighub/examples/eks-manager/internal/version"
)

func main() {
	// When cub installs or upgrades this plugin it invokes the binary as a
	// hook; HandleHook writes cub-plugin.yaml into the plugin directory and we
	// exit without running the normal command tree. The command token is "eks"
	// (so the plugin is invoked as `cub eks ...`), while the binary basename is
	// cub-eks (used as the entrypoint by default).
	manifest := plugin.Manifest{
		Name:    "cub-eks",
		Version: version.Version,
		Commands: []plugin.Command{{
			Name:    "eks",
			Summary: "Manage AWS EKS clusters as Crossplane managed resources across a ConfigHub fleet",
		}},
	}
	if handled, err := plugin.HandleHook(manifest); handled {
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	if err := cli.NewRoot().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
