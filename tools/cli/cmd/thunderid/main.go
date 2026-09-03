// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package main is the entry point for the ThunderID CLI.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/thunder-id/thunderid/tools/cli/internal/cli"
	"github.com/thunder-id/thunderid/tools/cli/internal/commands/sample"
	"github.com/thunder-id/thunderid/tools/cli/internal/commands/upgrade"
	"github.com/thunder-id/thunderid/tools/cli/internal/product"
	"github.com/thunder-id/thunderid/tools/cli/internal/services/config"
	"github.com/thunder-id/thunderid/tools/cli/internal/services/release"
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

	// --product-version selects which release to install and where to fetch it from, so it has to
	// be applied before any command reaches the release service.
	if err := configureReleaseSource(args); err != nil {
		ui.Fatal(err.Error())
		os.Exit(1)
	}

	// --help can follow a flag, so it is matched anywhere rather than only in first position.
	if hasFlag(args, "--help", "-h") {
		printUsage()
		return
	}

	// upgrade — stop the running version, install the latest, restart on the same port.
	if len(args) > 0 && args[0] == "upgrade" {
		// Upgrading to the latest while pinned to a version contradicts itself, and silently
		// dropping the pin would move an operator off the release they asked for.
		if release.Pinned() != "" {
			ui.Fatal("`upgrade` cannot run with a pinned --product-version. " +
				"Drop the pin to upgrade, or pass the version you want directly.")
			os.Exit(1)
		}
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
  --product-version    Release to install, or the manifest to read releases from.
                       Takes a version (1.0.1) or a URL (https://example.com/releases.json).
                       Pass it twice to combine the two. Also read from
                       THUNDERID_PRODUCT_VERSION.
  --help, -h           Show this help message

Examples:
  %s --product-version 1.0.1
  %s --product-version https://releases.example.com/releases.json
  %s --product-version https://releases.example.com/releases.json --product-version 1.0.1
`, product.Slug, product.Name, product.Slug, product.Slug, product.Slug)
}

// configureReleaseSource applies --product-version, falling back to THUNDERID_PRODUCT_VERSION so
// scripted runs can set it without rewriting their command lines. Flags win over the environment.
func configureReleaseSource(args []string) error {
	values := productVersionFlags(args)
	if len(values) == 0 {
		if env := strings.TrimSpace(os.Getenv("THUNDERID_PRODUCT_VERSION")); env != "" {
			values = strings.Fields(env)
		}
	}
	if len(values) == 0 {
		return nil
	}
	client, err := release.Configure(values)
	if err != nil {
		return err
	}
	release.Default = client
	return nil
}

// productVersionFlags collects every --product-version value, accepting both "--flag value" and
// "--flag=value".
func productVersionFlags(args []string) []string {
	var values []string
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == productVersionFlag:
			if i+1 < len(args) {
				values = append(values, args[i+1])
				i++
			}
		case strings.HasPrefix(args[i], productVersionFlag+"="):
			values = append(values, strings.TrimPrefix(args[i], productVersionFlag+"="))
		}
	}
	return values
}

const productVersionFlag = "--product-version"

// hasFlag reports whether any of names appears in args.
func hasFlag(args []string, names ...string) bool {
	for _, a := range args {
		for _, name := range names {
			if a == name {
				return true
			}
		}
	}
	return false
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
