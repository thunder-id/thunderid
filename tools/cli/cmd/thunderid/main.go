/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

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

	// upgrade [--direct] — explicit upgrade with optional blue/green staging.
	if len(args) > 0 && args[0] == "upgrade" {
		verbose, direct := parseUpgradeFlags(args[1:])
		if err := upgrade.Run(cli.BaseDir(), upgrade.Opts{Direct: direct, Verbose: verbose}); err != nil {
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
		if err := sample.Run(usecase, path, verbose, sample.Options{}); err != nil {
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
  upgrade              Upgrade to the latest release (side-by-side by default)
  try <usecase>        Download and launch a use-case sample app
  integrate <tech>     Configure a technology integration (coming soon)

Flags:
  --verbose, -v        Show detailed output
  --setup              Force re-run setup
  --help, -h           Show this help message

Upgrade flags:
  --direct             Upgrade in-place (stop current, upgrade, restart)
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

func parseUpgradeFlags(args []string) (verbose, direct bool) {
	for _, a := range args {
		switch a {
		case "--verbose", "-v":
			verbose = true
		case "--direct":
			direct = true
		}
	}
	return
}
