// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// This example shows how to register and use a kubepug API deprecation validation function with ConfigHub.
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/confighub/sdk/configkit/k8skit"
	"github.com/confighub/sdk/core/channel"
	"github.com/confighub/sdk/core/function/executor"
	"github.com/confighub/sdk/core/function/handler"
	"github.com/confighub/sdk/core/workerapi"
)

const kubepugDatabaseURL = "https://kubepug.xyz/data/data.json"

func main() {
	printCommands := flag.Bool("print-commands", false, "print the commands the bot declares, for cub channel create or update --commands, and exit")
	flag.Parse()

	// Download the kubepug deprecation database once at startup so that
	// each function invocation uses the local copy instead of downloading
	// it on every call. This avoids network latency during function execution.
	if !*printCommands {
		downloadKubepugDatabase()
	}

	exec := executor.NewEmptyExecutor()
	exec.RegisterToolchain(k8skit.NewK8sResourceProvider(), true)
	exec.RegisterFunction(workerapi.ToolchainKubernetesYAML, handler.FunctionRegistration{
		FunctionSignature: GetVetKubepugSignature(),
		Function:          VetKubepugFunction,
	})

	// The Channel is the first argument or CONFIGHUB_CHANNEL, as space/slug. With
	// CONFIGHUB_AUTH_PRIVATE_KEY set, the bot authenticates as the service account that key
	// belongs to; without it, as whoever the active cub context is logged in as.
	channelRef := flag.Arg(0)
	if channelRef == "" {
		channelRef = os.Getenv("CONFIGHUB_CHANNEL")
	}
	connector, err := channel.NewConnector(channel.ConnectorOptions{
		Channel:          channelRef,
		ConfigHubURL:     os.Getenv("CONFIGHUB_URL"),
		PrivateKey:       os.Getenv("CONFIGHUB_AUTH_PRIVATE_KEY"),
		FunctionExecutor: exec,
		UserAgent:        "kubepug-bot",
	})
	if err != nil {
		log.Fatalf("Failed to create connector: %v", err)
	}

	if *printCommands {
		if err := connector.PrintCommands(os.Stdout); err != nil {
			log.Fatalf("Failed to print commands: %v", err)
		}
		return
	}
	if channelRef == "" {
		fmt.Fprintln(os.Stderr, "usage: kubepug-bot <space>/<channel> | --print-commands")
		os.Exit(2)
	}

	log.Printf("Serving on channel %s", channelRef)
	if err := connector.Start(); err != nil {
		log.Fatalf("Connector stopped: %v", err)
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
