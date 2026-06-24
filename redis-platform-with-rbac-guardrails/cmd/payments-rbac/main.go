// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/confighub/examples/redis-platform-with-rbac-guardrails/internal/platform"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		usage()
		return fmt.Errorf("missing command")
	}
	cmd, rest := args[0], args[1:]
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fixture := fs.String("fixture", defaultFixturePath(), "payments-platform fixture")
	queryName := fs.String("query", "who-can-get-secrets-prod-us", "query name")
	if err := fs.Parse(rest); err != nil {
		return err
	}
	if cmd == "help" || cmd == "--help" || cmd == "-h" {
		usage()
		return nil
	}

	model, err := platform.Load(*fixture)
	if err != nil {
		return err
	}

	switch cmd {
	case "explain":
		fmt.Print(platform.Explain(model, "sample-output"))
		return nil
	case "explain-json":
		return writeJSON(platform.ExplainJSON(model, *fixture))
	case "component-map":
		return writeJSON(platform.ComponentMap(model))
	case "snapshot":
		return writeJSON(platform.Snapshot(model))
	case "who-can":
		query, ok := platform.Query(model, *queryName)
		if !ok {
			return fmt.Errorf("query %q not found", *queryName)
		}
		return writeJSON(query)
	case "findings":
		return writeJSON(platform.Findings(model))
	case "plan":
		return writeJSON(platform.ProposedEditPlan(model))
	case "verify":
		return platform.Verify(model)
	default:
		usage()
		return fmt.Errorf("unknown command %q", cmd)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `Usage:
  payments-rbac explain
  payments-rbac explain-json
  payments-rbac component-map
  payments-rbac snapshot
  payments-rbac who-can [--query name]
  payments-rbac findings
  payments-rbac plan
  payments-rbac verify`)
}

func writeJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func defaultFixturePath() string {
	return filepath.Join("fixtures", "payments-platform.json")
}
