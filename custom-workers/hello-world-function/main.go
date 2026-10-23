// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// This example shows how to register a custom function with ConfigHub and serve it from a
// function bot: a process that claims a Channel and runs the functions invoked through it.
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
)

func main() {
	printCommands := flag.Bool("print-commands", false, "print the commands the bot declares, for cub channel create or update --commands, and exit")
	flag.Parse()

	exec := executor.NewEmptyExecutor()
	exec.RegisterToolchain(k8skit.NewK8sResourceProvider(), true)
	exec.RegisterFunction(workerapi.ToolchainKubernetesYAML, handler.FunctionRegistration{
		FunctionSignature: GetHelloWorldFunctionSignature(),
		Function:          HelloWorldFunction,
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
		UserAgent:        "hello-world-function",
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
		fmt.Fprintln(os.Stderr, "usage: hello-world-function <space>/<channel> | --print-commands")
		os.Exit(2)
	}

	log.Printf("Serving hello-world on channel %s", channelRef)
	if err := connector.Start(); err != nil {
		log.Fatalf("Connector stopped: %v", err)
	}
}
