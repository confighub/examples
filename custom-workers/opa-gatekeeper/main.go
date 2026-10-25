// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// This example shows how to validate Kubernetes resources against OPA Gatekeeper
// constraints running in a cluster by sending AdmissionReview requests to the
// Gatekeeper webhook.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/confighub/sdk/configkit/k8skit"
	"github.com/confighub/sdk/core/channel"
	"github.com/confighub/sdk/core/function/executor"
	"github.com/confighub/sdk/core/function/handler"
	"github.com/confighub/sdk/core/workerapi"
	opagatekeeper "github.com/confighub/sdk/worker-function-impl/opa-gatekeeper"
)

func main() {
	printCommands := flag.Bool("print-commands", false, "print the commands the bot declares, for cub channel create or update --commands, and exit")
	flag.Parse()

	exec := executor.NewEmptyExecutor()
	exec.RegisterToolchain(k8skit.NewK8sResourceProvider(), true)
	registration := handler.FunctionRegistration{
		FunctionSignature: opagatekeeper.GetVetOPAGatekeeperSignature(),
		Function:          opagatekeeper.VetOPAGatekeeperFunction,
	}
	// The function watches the cluster's webhook configuration from the start, so it is
	// started only when the bot is to serve it.
	if !*printCommands {
		registration.FunctionInit = opagatekeeper.InitOPAGatekeeper
	}
	if err := exec.RegisterFunction(workerapi.ToolchainKubernetesYAML, registration); err != nil {
		log.Fatalf("Failed to register function: %v", err)
	}

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
		UserAgent:        "opa-gatekeeper",
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
		fmt.Fprintln(os.Stderr, "usage: opa-gatekeeper <space>/<channel> | --print-commands")
		os.Exit(2)
	}

	log.Printf("Serving on channel %s", channelRef)
	if err := connector.Start(); err != nil {
		log.Fatalf("Connector stopped: %v", err)
	}
}
