// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Command cub-autoscale manages Kubernetes autoscaling as data in ConfigHub
// across a fleet: HorizontalPodAutoscalers and their conversion to KEDA
// ScaledObjects, in a form usable by an AI agent in a terminal. The HPA→KEDA
// conversion runs a ConfigHub function (convert-hpa-to-keda) in an embedded
// executor in-process — no server-side function or worker required.
//
// It can be run standalone (cub-autoscale ...) or as a cub plugin
// (cub autoscale ...).
package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/confighub/sdk/core/plugin"

	"github.com/confighub/examples/autoscale-manager/internal/cli"
	"github.com/confighub/examples/autoscale-manager/internal/version"
)

func main() {
	// The embedded function executor logs at INFO on each invoke; keep the CLI's
	// output clean by surfacing only warnings and errors.
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})))

	manifest := plugin.Manifest{
		Name:    "cub-autoscale",
		Version: version.Version,
		Commands: []plugin.Command{{
			Name:    "autoscale",
			Summary: "Manage HorizontalPodAutoscalers and KEDA ScaledObjects across a ConfigHub fleet",
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
