// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// This example shows how to register and use a kubepug API deprecation validation function with ConfigHub.
package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/confighub/sdk/configkit/k8skit"
	"github.com/confighub/sdk/core/function/executor"
	"github.com/confighub/sdk/core/function/handler"
	"github.com/confighub/sdk/core/worker"
	"github.com/confighub/sdk/core/workerapi"
)

const kubepugDatabaseURL = "https://kubepug.xyz/data/data.json"

func main() {
	// Download the kubepug deprecation database once at startup so that
	// each function invocation uses the local copy instead of downloading
	// it on every call. This avoids network latency during function execution.
	downloadKubepugDatabase()

	exec := executor.NewEmptyExecutor()
	exec.RegisterToolchain(k8skit.NewK8sResourceProvider())
	exec.RegisterFunction(workerapi.ToolchainKubernetesYAML, handler.FunctionRegistration{
		FunctionSignature: GetVetKubepugSignature(),
		Function:          VetKubepugFunction,
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

func downloadKubepugDatabase() {
	log.Printf("Downloading kubepug database from %s...", kubepugDatabaseURL)

	resp, err := http.Get(kubepugDatabaseURL) //nolint:gosec // URL is a compile-time constant
	if err != nil {
		log.Printf("Warning: failed to download kubepug database: %v (will download per invocation)", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Warning: kubepug database download returned %d (will download per invocation)", resp.StatusCode)
		return
	}

	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = os.TempDir()
	}
	dbDir := filepath.Join(cacheDir, "kubepug")
	os.MkdirAll(dbDir, 0o755)
	dbPath := filepath.Join(dbDir, "data.json")

	f, err := os.Create(dbPath)
	if err != nil {
		log.Printf("Warning: failed to create kubepug database file: %v (will download per invocation)", err)
		return
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		log.Printf("Warning: failed to write kubepug database: %v (will download per invocation)", err)
		os.Remove(dbPath)
		return
	}

	kubepugDatabasePath = dbPath
	log.Printf("Kubepug database cached at %s", dbPath)
}
