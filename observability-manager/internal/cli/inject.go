// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/confighub/sdk/cliutil"

	"github.com/confighub/examples/observability-manager/internal/cub"
)

// newInjectSidecarCmd injects (or updates) a telemetry sidecar container into a
// workload's pod template via set-path (find-or-append by container name).
func newInjectSidecarCmd() *cobra.Command {
	var output, name, image string
	var otlpPort int
	var commit cliutil.CommitFlags
	cmd := &cobra.Command{
		Use:   "inject-sidecar <space>/<unit>",
		Short: "Inject (or update) a telemetry sidecar container into a workload",
		Long: `inject-sidecar adds an OpenTelemetry / telemetry collector sidecar container to a
workload's pod template, using set-path to find-or-append the container by name:
if a container with --name already exists it is replaced, otherwise it is
appended.

  inject-sidecar web-prod/web --image otel/opentelemetry-collector:0.100

--name defaults to otel-collector; --otlp-grpc-port defaults to 4317. Applies to
Deployment / StatefulSet / DaemonSet / ReplicaSet / Job (pod template at
spec.template.spec). Dry-run unless --commit --change-desc; never bypasses
ApplyGates.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if image == "" {
				return fmt.Errorf("--image is required")
			}
			path := fmt.Sprintf("spec.template.spec.containers.?name=%s", name)
			// set-path injects the container name from the path's merge key; the value
			// carries the rest of the container.
			value := fmt.Sprintf("image: %q\nports:\n- name: otlp-grpc\n  containerPort: %d\n", image, otlpPort)

			changeDesc, dryRun, err := commit.Validate(fmt.Sprintf("inject %s sidecar into %s", name, args[0]))
			if err != nil {
				return err
			}
			client, err := cub.Preflight(cmd.Context())
			if err != nil {
				return err
			}
			ref, err := parseUnitRef(cmd.Context(), client, args[0])
			if err != nil {
				return err
			}
			res, err := cub.InvokeSetPath(cmd.Context(), client, path, value, ref.selector(), changeOf(changeDesc, dryRun))
			if err != nil {
				return err
			}
			return reportMutation(cmd, "inject-sidecar", ref.spaceSlug, dryRun, output, res)
		},
	}
	addOutputFlag(cmd, &output)
	commit.Bind(cmd)
	cmd.Flags().StringVar(&name, "name", "otel-collector", "sidecar container name")
	cmd.Flags().StringVar(&image, "image", "", "sidecar container image (required)")
	cmd.Flags().IntVar(&otlpPort, "otlp-grpc-port", 4317, "OTLP gRPC container port")
	return cmd
}
