// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// This example shows how to register and use a kubescape validation function with ConfigHub.
package main

import (
	"context"
	"log"
	"os"
	osexec "os/exec"
	"time"

	"github.com/confighub/sdk/configkit/k8skit"
	"github.com/confighub/sdk/core/function/executor"
	"github.com/confighub/sdk/core/function/handler"
	"github.com/confighub/sdk/core/worker"
	"github.com/confighub/sdk/core/workerapi"
)

func main() {
	// Pre-download kubescape artifacts (frameworks, controls, etc.) into the
	// default cache (~/.kubescape) so that scans don't need to fetch them.
	downloadKubescapeArtifacts()

	exec := executor.NewEmptyExecutor()
	exec.RegisterToolchain(k8skit.NewK8sResourceProvider(), true)
	exec.RegisterFunction(workerapi.ToolchainKubernetesYAML, handler.FunctionRegistration{
		FunctionSignature: GetVetKubescapeSignature(),
		Function:          VetKubescapeFunction,
	})

	connector, err := worker.NewConnector(worker.ConnectorOptions{
		WorkerID:         os.Getenv("CONFIGHUB_WORKER_ID"),
		WorkerSecret:     os.Getenv("CONFIGHUB_WORKER_SECRET"),
		ConfigHubURL:     os.Getenv("CONFIGHUB_URL"),
		FunctionExecutor: exec,
	})

	if err != nil {
		log.Fatalf("Failed to create connector: %v", err)
	}

	err = connector.Start()
	if err != nil {
		log.Fatalf("Failed to start connector: %v", err)
	}
}

// downloadKubescapeArtifacts runs `kubescape download artifacts` to populate
// the default cache (~/.kubescape). This avoids network I/O during scans.
func downloadKubescapeArtifacts() {
	log.Println("Downloading kubescape artifacts...")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := osexec.CommandContext(ctx, kubescapeBinary, "download", "artifacts")
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("Warning: failed to download kubescape artifacts: %v\n%s", err, out)
		return
	}
	log.Println("Kubescape artifacts downloaded.")
}
