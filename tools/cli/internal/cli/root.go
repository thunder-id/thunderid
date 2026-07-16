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

// Package cli contains the default command that installs, sets up, and starts ThunderID.
package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	huhspinner "charm.land/huh/v2/spinner"
	"charm.land/lipgloss/v2"

	"github.com/thunder-id/thunderid/tools/cli/internal/commands/upgrade"
	"github.com/thunder-id/thunderid/tools/cli/internal/product"
	"github.com/thunder-id/thunderid/tools/cli/internal/services/config"
	"github.com/thunder-id/thunderid/tools/cli/internal/services/health"
	"github.com/thunder-id/thunderid/tools/cli/internal/services/release"
	"github.com/thunder-id/thunderid/tools/cli/internal/services/setup"
	"github.com/thunder-id/thunderid/tools/cli/internal/ui"
	"github.com/thunder-id/thunderid/tools/cli/internal/ui/spinner"
	"github.com/thunder-id/thunderid/tools/cli/internal/utils"
)

// nodeVersionWarning returns a non-blocking warning message when the installed
// Node.js version is below utils.MinNodeVersion, or "" if it's fine. Sample apps
// launched via the try-* commands run on Node.js, so an outdated version can
// break them even though it doesn't affect the core server started here.
func nodeVersionWarning() string {
	version, err := utils.DetectNodeVersion()
	if err != nil {
		return fmt.Sprintf("Could not detect Node.js — v%s or later is required to run sample apps (/try commands).\n%s",
			utils.MinNodeVersion, utils.NodeUpgradeHint())
	}
	if !utils.MeetsMinNodeVersion(version) {
		return fmt.Sprintf("Node.js v%s detected — v%s or later is recommended. Sample apps (/try commands) may not work correctly.\n%s",
			version, utils.MinNodeVersion, utils.NodeUpgradeHint())
	}
	return ""
}

// BaseDir is the parent directory that holds all versioned installs and samples.
func BaseDir() string {
	return filepath.Join(".", product.Slug)
}

// VersionedInstallPath returns the extracted artifact directory for the given version.
func VersionedInstallPath(version string) string {
	return filepath.Join(BaseDir(), "v"+version)
}

// Run executes the default (no-args) CLI command: fetch version, install if needed,
// run setup, start background, launch interactive REPL.
func Run(verbose, forceSetup bool) {
	if !verbose && runtime.GOOS != "windows" {
		fmt.Print("\033[H\033[2J")
	}

	ui.PrintBanner()

	nodeWarning := nodeVersionWarning()
	if nodeWarning != "" {
		ui.Warn(nodeWarning)
	}

	fmt.Print(ui.Dim("  Fetching latest " + product.Name + " release..."))
	latestVersion, err := release.FetchLatestVersion()
	if err != nil {
		fmt.Println()
		ui.Fatal("Could not fetch latest " + product.Name + " release: " + err.Error())
		os.Exit(1)
	}
	fmt.Printf("\r\033[2K  %s Latest %s release: v%s\n\n", ui.Green("✓"), product.Name, latestVersion)

	activeVersion := config.ReadActiveVersion()

	// Always start the active version; only download when there's no installed version yet.
	runVersion := latestVersion
	if activeVersion != "" {
		runVersion = activeVersion
	}

	// Show a new-version banner inside the REPL (not a blocking prompt).
	var newVersion string
	if activeVersion != "" && activeVersion != latestVersion && !config.IsVersionSkipped(latestVersion) {
		newVersion = latestVersion
	}

	path := VersionedInstallPath(runVersion)
	installOnDisk := false
	if stored := config.ReadInstallPath(runVersion); stored != "" {
		if _, err := os.Stat(stored); err == nil {
			path = stored
			installOnDisk = true
		}
	}
	alreadyInstalled := activeVersion == runVersion && config.IsSetupComplete(runVersion) && installOnDisk
	isFirstRun := !config.IsOnboardingDone(runVersion)

	// If the product is already responding on port 8090 and we have a valid local install,
	// skip setup and attach the REPL to the running instance without starting a new process.
	// If the install is missing or state is gone, fall through to reinstall even if something
	// is already listening on the port.
	if !forceSetup && alreadyInstalled && isRunning() {
		ui.Note("Already running",
			fmt.Sprintf("%s is already running on port %d.\nAttaching to the existing instance.",
				product.Name, health.DefaultPort))
		replLoop(runVersion, path, nil, verbose, isFirstRun, newVersion, nodeWarning, 0)
		return
	}

	port := health.DefaultPort

	if alreadyInstalled && !forceSetup {
		ui.Note("Starting "+product.Name, fmt.Sprintf("%s v%s is ready\n%s", product.Name, runVersion, path))
	} else if activeVersion == runVersion && installOnDisk {
		absPath, err := filepath.Abs(path)
		if err != nil {
			absPath = path
		}
		if err := config.WriteInstallPath(runVersion, absPath); err != nil {
			ui.Fatal("Failed to record install path: " + err.Error())
			os.Exit(1)
		}
		path = absPath
		if forceSetup {
			ui.Note("Setup requested", fmt.Sprintf("Re-running setup for %s v%s\n%s", product.Name, runVersion, path))
		} else {
			ui.Note("First-time setup", fmt.Sprintf("Setting up %s v%s\n%s", product.Name, runVersion, path))
		}
		port = resolvePort(path)
		runSetupPhase(runVersion, path, verbose)
	} else {
		// If the previously-active version is no longer in the manifest we'd need
		// to download it, so fall back to the latest available version.
		if runVersion != latestVersion {
			runVersion = latestVersion
			path = VersionedInstallPath(runVersion)
			newVersion = "" // already on latest after this download
		}
		downloadAndInstall(runVersion, path, verbose)
		absPath, err := filepath.Abs(path)
		if err != nil {
			absPath = path
		}
		if err := config.WriteInstallPath(runVersion, absPath); err != nil {
			ui.Fatal("Failed to record install path: " + err.Error())
			os.Exit(1)
		}
		path = absPath
		port = resolvePort(path)
		runSetupPhase(runVersion, path, verbose)
		if err := config.WriteActiveVersion(runVersion); err != nil {
			ui.Fatal("Failed to record active version: " + err.Error())
			os.Exit(1)
		}
	}

	if !setup.WaitForPortFree(port, 10*time.Second) {
		setup.KillPort(port)
		setup.WaitForPortFree(port, 5*time.Second)
	}

	fmt.Print(ui.Dim("\n  Starting " + product.Name + " in the background..."))
	proc, err := setup.StartBackgroundOnPort(path, verbose, port)
	if err != nil {
		fmt.Println()
		ui.Fatal("Failed to start " + product.Name + ": " + err.Error())
		os.Exit(1)
	}
	fmt.Printf("\r\033[2K  %s %s started  %s\n", ui.Green("✓"), product.Name, ui.Dim("logs: "+setup.LogDir(path)))

	replLoop(runVersion, path, proc, verbose, isFirstRun, newVersion, nodeWarning, port)
}

// replLoop runs the REPL repeatedly, re-entering after no-op upgrades or version switches.
// An actual upgrade or normal exit breaks the loop.
func replLoop(version, installPath string, proc *exec.Cmd, verbose, isFirstRun bool, newVersion, nodeWarning string, port int) {
	for {
		upgradeRequested, switchRequested, err := ui.RunREPL(version, proc, installPath, verbose, isFirstRun, newVersion, nodeWarning, port)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\nREPL error: %v\n", err)
			os.Exit(1)
		}
		isFirstRun = false
		newVersion = ""

		if upgradeRequested {
			upgraded, err := upgrade.Run(BaseDir(), upgrade.Opts{Verbose: verbose})
			if err != nil {
				os.Exit(1)
			}
			if upgraded {
				return // upgrade ran its own REPL internally
			}
			// Already latest or cancelled — Thunder is still running; reattach.
			continue
		}

		if switchRequested {
			switched, err := upgrade.Switch(BaseDir(), version, verbose)
			if err != nil {
				os.Exit(1)
			}
			if switched {
				return // Switch ran its own REPL internally
			}
			// Cancelled — reattach to the still-running instance.
			continue
		}

		return // normal exit
	}
}

// resolvePort checks whether the default port is available and, if not, prompts
// the user to either kill the occupying process or switch to a free alternate port.
func resolvePort(installPath string) int {
	if !setup.IsPortInUse(health.DefaultPort) {
		return health.DefaultPort
	}
	altPort := setup.FindFreePort(health.DefaultPort + 1)
	choice, selectedPort := ui.PromptPortConflict(health.DefaultPort, altPort)
	switch choice {
	case ui.KillAndUsePort:
		setup.KillPort(health.DefaultPort)
		setup.WaitForPortFree(health.DefaultPort, 5*time.Second)
		return health.DefaultPort
	case ui.UseAlternatePort:
		if err := setup.UpdateServerPort(installPath, selectedPort); err != nil {
			ui.Warn("Could not update port configuration: " + err.Error())
			setup.KillPort(health.DefaultPort)
			setup.WaitForPortFree(health.DefaultPort, 5*time.Second)
			return health.DefaultPort
		}
		return selectedPort
	default: // ui.AbortSetup
		os.Exit(0)
		return 0
	}
}

// isRunning returns true if the product is already responding on the default port.
func isRunning() bool {
	for _, scheme := range []string{"https", "http"} {
		if health.CheckReady(fmt.Sprintf("%s://localhost:%d", scheme, health.DefaultPort)) {
			return true
		}
	}
	return false
}

// downloadAndInstall downloads and extracts the product into path.
func downloadAndInstall(version, path string, verbose bool) {
	fmt.Println()

	if verbose {
		if err := release.Download(version, path, func(pct int, msg string) {
			if pct < 0 {
				fmt.Println("  " + msg)
			} else {
				fmt.Printf("  %s  %d%%\n", msg, pct)
			}
		}); err != nil {
			ui.Fatal("Download failed: " + err.Error())
			os.Exit(1)
		}
	} else {
		if err := release.Download(version, path, func(pct int, msg string) {
			if pct < 0 {
				fmt.Printf("\r\033[2K  %s", msg)
			} else {
				fmt.Printf("\r\033[2K  %s  %s  %3d%%", spinner.Render(pct), msg, pct)
			}
		}); err != nil {
			fmt.Println()
			ui.Fatal("Download failed: " + err.Error())
			os.Exit(1)
		}
		fmt.Println()
	}

	fmt.Printf("  %s %s v%s installed to %s\n", ui.Green("✓"), product.Name, version, path)
}

// runSetupPhase runs setup.sh with a spinner (non-verbose) or raw output (verbose).
// On failure in non-verbose mode, the captured stderr is printed before the error box.
func runSetupPhase(version, installPath string, verbose bool) {
	if verbose {
		fmt.Printf("\n  Running %s setup (v%s)...\n", product.Name, version)
		if err := setup.RunSetup(installPath, true); err != nil {
			ui.Fatal("Setup failed: " + err.Error())
			os.Exit(1)
		}
	} else {
		fmt.Println()
		var setupErr error
		if err := huhspinner.New().
			WithTheme(huhspinner.ThemeFunc(func(bool) *huhspinner.Styles {
				return &huhspinner.Styles{
					Spinner: lipgloss.NewStyle().Foreground(lipgloss.Color(product.ColorElectricBlue)).PaddingLeft(2),
					Title:   lipgloss.NewStyle(),
				}
			})).
			Title("Setting up " + product.Name + "...").
			Action(func() {
				setupErr = setup.RunSetup(installPath, false)
			}).
			Run(); err != nil {
			ui.Fatal("Setup interrupted: " + err.Error())
			os.Exit(1)
		}
		if setupErr != nil {
			msg := setupErr.Error()
			if idx := strings.Index(msg, "\n\n"); idx != -1 {
				detail := strings.TrimSpace(msg[idx+2:])
				if detail != "" {
					fmt.Println()
					for _, line := range strings.Split(detail, "\n") {
						fmt.Println("  " + line)
					}
					fmt.Println()
				}
				msg = strings.TrimSpace(msg[:idx])
			}
			ui.Fatal(msg)
			os.Exit(1)
		}
	}

	if err := config.MarkSetupComplete(version); err != nil {
		ui.Fatal("Failed to mark setup complete: " + err.Error())
		os.Exit(1)
	}
	fmt.Printf("  %s Setup complete\n", ui.Green("✓"))
}
