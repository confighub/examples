// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Command cub-observability manages the observability posture of Kubernetes
// workloads as data in ConfigHub across a fleet: Prometheus ServiceMonitor
// coverage of metrics-exposing Services, and OpenTelemetry / telemetry sidecar
// injection — in a form usable by an AI agent in a terminal.
//
// It can be run standalone (cub-observability ...) or as a cub plugin
// (cub observability ...). All ConfigHub I/O goes through the ConfigHub API
// directly via the github.com/confighub/sdk/core/cubapi package.
package main

import (
	"fmt"
	"os"

	"github.com/confighub/sdk/core/plugin"

	"github.com/confighub/examples/observability-manager/internal/cli"
	"github.com/confighub/examples/observability-manager/internal/version"
)

func main() {
	manifest := plugin.Manifest{
		Name:    "cub-observability",
		Version: version.Version,
		Commands: []plugin.Command{{
			Name:    "observability",
			Summary: "Manage Prometheus ServiceMonitor coverage and telemetry sidecars across a ConfigHub fleet",
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
