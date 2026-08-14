// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package main is the entry point for the ThunderID CLI.
package main

import (
	"fmt"
	"os"

	"github.com/thunder-id/thunderid/tools/cli/internal/cli"
	"github.com/thunder-id/thunderid/tools/cli/internal/commands/sample"
	"github.com/thunder-id/thunderid/tools/cli/internal/commands/upgrade"
	"github.com/thunder-id/thunderid/tools/cli/internal/product"
	"github.com/thunder-id/thunderid/tools/cli/internal/services/config"
	"github.com/thunder-id/thunderid/tools/cli/internal/ui"
)

func main() {
	args := os.Args[1:]

	// upgrade — stop the running version, install the latest, restart on the same port.
	if len(args) > 0 && args[0] == "upgrade" {
		verbose, _ := parseFlags(args[1:])
		if _, err := upgrade.Run(cli.BaseDir(), upgrade.Opts{Verbose: verbose}); err != nil {
			os.Exit(1)
		}
		return
	}

	// try <usecase> — download and launch a use-case sample app.
	if len(args) >= 2 && args[0] == "try" {
		usecase := args[1]
		verbose, _ := parseFlags(args[2:])
		activeVersion := config.ReadActiveVersion()
		if activeVersion == "" {
			ui.Fatal(fmt.Sprintf("No active %s install found. Run `npx %s` first.", product.Name, product.Slug))
			os.Exit(1)
		}
		path := cli.VersionedInstallPath(activeVersion)
		if err := sample.Run(usecase, path, verbose, sample.Options{
			ConfirmPorts: ui.ConfirmStopPortHolders,
		}); err != nil {
			ui.Fatal(err.Error())
			os.Exit(1)
		}
		return
	}

	// integrate <technology> — configure a technology integration (future).
	if len(args) >= 2 && args[0] == "integrate" {
		ui.Fatal(fmt.Sprintf("`integrate %s` is not yet implemented.", args[1]))
		os.Exit(1)
	}

	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		printUsage()
		return
	}

	verbose, forceSetup := parseFlags(args)
	cli.Run(verbose, forceSetup)
}

func printUsage() {
	fmt.Printf(`Usage: %s [command] [flags]

Commands:
  (none)               Install and start %s
  upgrade              Upgrade to the latest release
  try <usecase>        Download and launch a use-case sample app

Flags:
  --verbose, -v        Show detailed output
  --setup              Force re-run setup
  --help, -h           Show this help message
`, product.Slug, product.Name)
}

func parseFlags(args []string) (verbose, forceSetup bool) {
	for _, a := range args {
		switch a {
		case "--verbose", "-v":
			verbose = true
		case "--setup":
			forceSetup = true
		}
	}
	return
}
