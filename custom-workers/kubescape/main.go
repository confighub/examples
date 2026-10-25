// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// This example shows how to register and use a kubescape validation function with ConfigHub.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	osexec "os/exec"
	"time"

	"github.com/confighub/sdk/configkit/k8skit"
	"github.com/confighub/sdk/core/channel"
	"github.com/confighub/sdk/core/function/executor"
	"github.com/confighub/sdk/core/function/handler"
	"github.com/confighub/sdk/core/workerapi"
)

func main() {
	printCommands := flag.Bool("print-commands", false, "print the commands the bot declares, for cub channel create or update --commands, and exit")
	flag.Parse()

	// Pre-download kubescape artifacts (frameworks, controls, etc.) into the
	// default cache (~/.kubescape) so that scans don't need to fetch them.
	if !*printCommands {
		downloadKubescapeArtifacts()
	}

	exec := executor.NewEmptyExecutor()
	exec.RegisterToolchain(k8skit.NewK8sResourceProvider(), true)
	exec.RegisterFunction(workerapi.ToolchainKubernetesYAML, handler.FunctionRegistration{
		FunctionSignature: GetVetKubescapeSignature(),
		Function:          VetKubescapeFunction,
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
		UserAgent:        "kubescape-bot",
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
		fmt.Fprintln(os.Stderr, "usage: kubescape-bot <space>/<channel> | --print-commands")
		os.Exit(2)
	}

	log.Printf("Serving on channel %s", channelRef)
	if err := connector.Start(); err != nil {
		log.Fatalf("Connector stopped: %v", err)
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
