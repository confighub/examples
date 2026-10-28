// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// This example shows how to validate Kubernetes resources against the policies of an
// admission controller running in a cluster, Kyverno or OPA Gatekeeper, by sending
// AdmissionReview requests to its webhook.
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

	"github.com/confighub/examples/custom-workers/admission-webhook/gatekeeper"
	"github.com/confighub/examples/custom-workers/admission-webhook/kyverno"
)

func main() {
	printCommands := flag.Bool("print-commands", false, "print the commands the bot declares, for cub channel create or update --commands, and exit")
	flag.Parse()

	// The bot declares both functions, and serves each whose admission controller it is
	// told the URL of. A function it does not serve fails with a message saying which
	// variable is missing.
	kyvernoURL := os.Getenv("KYVERNO_URL")
	gatekeeperURL := os.Getenv("GATEKEEPER_URL")

	exec := executor.NewEmptyExecutor()
	exec.RegisterToolchain(k8skit.NewK8sResourceProvider(), true)
	kyvernoRegistration := handler.FunctionRegistration{
		FunctionSignature: kyverno.GetVetKyvernoServerSignature(),
		Function:          kyverno.VetKyvernoServerFunction,
	}
	gatekeeperRegistration := handler.FunctionRegistration{
		FunctionSignature: gatekeeper.GetVetOPAGatekeeperSignature(),
		Function:          gatekeeper.VetOPAGatekeeperFunction,
	}
	// Each function watches the cluster's webhook configuration from the start, so it is
	// started only when the bot is to serve it.
	if !*printCommands {
		if kyvernoURL != "" {
			kyvernoRegistration.FunctionInit = kyverno.InitKyvernoServer
		}
		if gatekeeperURL != "" {
			gatekeeperRegistration.FunctionInit = gatekeeper.InitOPAGatekeeper
		}
	}
	for _, registration := range []handler.FunctionRegistration{kyvernoRegistration, gatekeeperRegistration} {
		if err := exec.RegisterFunction(workerapi.ToolchainKubernetesYAML, registration); err != nil {
			log.Fatalf("Failed to register function: %v", err)
		}
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
		UserAgent:        "admission-webhook",
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
		fmt.Fprintln(os.Stderr, "usage: admission-webhook <space>/<channel> | --print-commands")
		os.Exit(2)
	}
	if kyvernoURL == "" && gatekeeperURL == "" {
		fmt.Fprintln(os.Stderr, "set KYVERNO_URL, GATEKEEPER_URL or both to the webhook the bot is to call")
		os.Exit(2)
	}

	log.Printf("Serving on channel %s", channelRef)
	if err := connector.Start(); err != nil {
		log.Fatalf("Connector stopped: %v", err)
	}
}
