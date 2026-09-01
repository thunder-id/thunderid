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

// version is the CLI version. It is injected at build time via
// -ldflags "-X main.version=<v>" and keeps the placeholder below for local builds.
var version = "0.0.0-semantically-released"

func main() {
	args := os.Args[1:]

	// --version is resolved before command dispatch so it always prints and exits,
	// rather than falling through to a command or to install and setup.
	if hasVersionFlag(args) {
		fmt.Println(versionLine())
		return
	}

	// upgrade — stop the running version, install the latest, restart on the same port.
	if len(args) > 0 && args[0] == "upgrade" {
		verbose, _ := parseFlags(args[1:])
		_, notice, err := upgrade.Run(cli.BaseDir(), upgrade.Opts{Verbose: verbose})
		if err != nil {
			os.Exit(1)
		}
		if notice != "" {
			fmt.Println("  " + notice)
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

// hasVersionFlag reports whether args request the CLI version, in any position.
func hasVersionFlag(args []string) bool {
	for _, a := range args {
		if a == "--version" || a == "-V" {
			return true
		}
	}
	return false
}

// versionLine renders the version string printed by --version.
func versionLine() string {
	return fmt.Sprintf("%s %s", product.Slug, version)
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
  --version, -V        Show the CLI version
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
