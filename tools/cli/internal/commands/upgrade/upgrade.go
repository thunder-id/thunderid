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

// Package upgrade orchestrates in-place and side-by-side version upgrades.
package upgrade

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"charm.land/huh/v2"
	huhspinner "charm.land/huh/v2/spinner"
	"charm.land/lipgloss/v2"

	"github.com/thunder-id/thunderid/tools/cli/internal/product"
	"github.com/thunder-id/thunderid/tools/cli/internal/services/config"
	"github.com/thunder-id/thunderid/tools/cli/internal/services/health"
	"github.com/thunder-id/thunderid/tools/cli/internal/services/release"
	"github.com/thunder-id/thunderid/tools/cli/internal/services/setup"
	"github.com/thunder-id/thunderid/tools/cli/internal/ui"
	"github.com/thunder-id/thunderid/tools/cli/internal/ui/spinner"
)

const stagingPort = 8091

// Opts controls how the upgrade runs.
type Opts struct {
	Direct  bool // skip side-by-side and upgrade in-place
	Verbose bool
}

// Run executes the upgrade workflow. baseDir is the parent thunderid directory (e.g. "./thunderid").
// Returns (upgraded, err): upgraded is false when already on the latest version or the user cancelled.
func Run(baseDir string, opts Opts) (bool, error) {
	fmt.Print(ui.Dim("  Fetching latest " + product.Name + " release..."))
	latestVersion, err := release.FetchLatestVersion()
	if err != nil {
		fmt.Println()
		ui.Fatal("Could not fetch latest " + product.Name + " release: " + err.Error())
		return false, err
	}
	fmt.Printf("\r\033[2K  %s Latest %s release: v%s\n\n", ui.Green("✓"), product.Name, latestVersion)

	activeVersion := config.ReadActiveVersion()
	if activeVersion == latestVersion {
		ui.Success(product.Name + " v" + latestVersion + " is already the latest version.")
		return false, nil
	}

	if activeVersion != "" {
		fmt.Printf("  Upgrading %s: %s → %s\n\n",
			product.Name,
			ui.Dim("v"+activeVersion),
			ui.Green("v"+latestVersion),
		)
	}

	if opts.Direct || activeVersion == "" {
		return true, runDirect(baseDir, activeVersion, latestVersion, opts.Verbose)
	}

	var mode string
	if err := huh.NewSelect[string]().
		Title("How would you like to upgrade?").
		Options(
			huh.NewOption(
				fmt.Sprintf("Side-by-side — run v%s on port %d while v%s keeps serving on %d (recommended)",
					latestVersion, stagingPort, activeVersion, health.DefaultPort),
				"side-by-side",
			),
			huh.NewOption(
				fmt.Sprintf("Direct — stop v%s, upgrade, restart on port %d", activeVersion, health.DefaultPort),
				"direct",
			),
		).
		Value(&mode).
		Run(); err != nil {
		return false, nil // cancelled
	}

	if mode == "direct" {
		return true, runDirect(baseDir, activeVersion, latestVersion, opts.Verbose)
	}
	return true, runSideBySide(baseDir, activeVersion, latestVersion, opts.Verbose)
}

// Switch stops the running ThunderID instance and starts the selected installed version.
// It shows an interactive version picker and returns false if the user cancels or no other
// versions are installed. On success it starts the new instance and runs a REPL for it.
func Switch(baseDir, currentVersion string, verbose bool) (bool, error) {
	versions := config.ListInstalledVersions(currentVersion)
	if len(versions) == 0 {
		ui.Warn("No other installed versions found. Use /upgrade to install a new version.")
		return false, nil
	}

	options := make([]huh.Option[string], len(versions))
	for i, v := range versions {
		options[i] = huh.NewOption("v"+v, v)
	}

	var selected string
	if err := huh.NewSelect[string]().
		Title("Switch to version:").
		Options(options...).
		Value(&selected).
		Run(); err != nil {
		return false, nil // cancelled
	}

	installPath := config.ReadInstallPath(selected)
	if installPath == "" {
		ui.Fatal("Install path not found for v" + selected + ". Re-run setup to restore it.")
		return false, nil
	}

	// Validate the install is launchable before touching the running instance.
	if _, err := setup.FindThunderRoot(installPath); err != nil {
		ui.Fatal(fmt.Sprintf("v%s is not usable (%s). The install may have been moved or deleted.", selected, err))
		return false, nil
	}

	fmt.Print(ui.Dim("  Stopping " + product.Name + " v" + currentVersion + "..."))
	setup.KillPort(health.DefaultPort)
	setup.WaitForPortFree(health.DefaultPort, 10*time.Second)
	fmt.Printf("\r\033[2K  %s Stopped v%s\n", ui.Green("✓"), currentVersion)

	if err := config.WriteActiveVersion(selected); err != nil {
		return false, fmt.Errorf("failed to update active version: %w", err)
	}

	fmt.Print(ui.Dim("\n  Starting " + product.Name + " v" + selected + "..."))
	proc, err := setup.StartBackground(installPath, verbose)
	if err != nil {
		fmt.Println()
		ui.Fatal("Failed to start v" + selected + ": " + err.Error())
		return false, err
	}
	fmt.Printf("\r\033[2K  %s Switched to %s v%s  %s\n", ui.Green("✓"), product.Name, selected, ui.Dim("logs: "+setup.LogDir(installPath)))

	_, _, err = ui.RunREPL(selected, proc, installPath, verbose, false, "", "", 0)
	return true, err
}

func runDirect(baseDir, activeVersion, newVersion string, verbose bool) error {
	label := product.Name
	if activeVersion != "" {
		label = product.Name + " v" + activeVersion
	}
	fmt.Print(ui.Dim("  Stopping " + label + "..."))
	setup.KillPort(health.DefaultPort)
	setup.KillPort(stagingPort)
	setup.WaitForPortFree(health.DefaultPort, 15*time.Second)
	if activeVersion != "" {
		fmt.Printf("\r\033[2K  %s Stopped v%s\n", ui.Green("✓"), activeVersion)
	} else {
		fmt.Printf("\r\033[2K\n")
	}

	newPath := versionedPath(baseDir, newVersion)
	if err := downloadVersion(newVersion, newPath, verbose); err != nil {
		return err
	}
	if err := runSetupWithPort(newVersion, newPath, verbose, 0); err != nil {
		return err
	}

	fmt.Print(ui.Dim("\n  Starting " + product.Name + " v" + newVersion + "..."))
	proc, err := setup.StartBackground(newPath, verbose)
	if err != nil {
		fmt.Println()
		ui.Fatal("Failed to start " + product.Name + ": " + err.Error())
		return err
	}

	// Persist both the install path and the new active version only after the
	// process has successfully started, so a failed launch doesn't corrupt state.
	if err := config.WriteInstallPath(newVersion, newPath); err != nil {
		ui.Fatal("Failed to persist install path: " + err.Error())
		return err
	}
	if err := config.WriteActiveVersion(newVersion); err != nil {
		ui.Fatal("Failed to update active version: " + err.Error())
		return err
	}
	fmt.Printf("\r\033[2K  %s %s v%s started  %s\n", ui.Green("✓"), product.Name, newVersion, ui.Dim("logs: "+setup.LogDir(newPath)))

	_, _, err = ui.RunREPL(newVersion, proc, newPath, verbose, false, "", "", 0)
	return err
}

func runSideBySide(baseDir, activeVersion, newVersion string, verbose bool) error {
	newPath := versionedPath(baseDir, newVersion)
	if err := downloadVersion(newVersion, newPath, verbose); err != nil {
		return err
	}
	if err := runSetupWithPort(newVersion, newPath, verbose, stagingPort); err != nil {
		return err
	}

	fmt.Print(ui.Dim(fmt.Sprintf("\n  Starting %s v%s on port %d (staging)...", product.Name, newVersion, stagingPort)))
	proc, err := setup.StartBackgroundOnPort(newPath, verbose, stagingPort)
	if err != nil {
		fmt.Println()
		ui.Fatal("Failed to start staging instance: " + err.Error())
		return err
	}
	fmt.Printf("\r\033[2K  %s %s v%s staging on port %d\n", ui.Green("✓"), product.Name, newVersion, stagingPort)
	fmt.Printf("  %s v%s still serving on port %d\n\n", ui.Dim("→  Current"), activeVersion, health.DefaultPort)
	fmt.Printf("  Type %s in the REPL to cut over to v%s and restart on port %d.\n\n",
		ui.Cyan("/cutover"), newVersion, health.DefaultPort)

	cutoverRequested, err := ui.RunStagingREPL(newVersion, proc, newPath, verbose, stagingPort)
	if err != nil {
		return err
	}
	if !cutoverRequested {
		return nil // user exited without cutting over
	}
	return performCutover(baseDir, activeVersion, newVersion, newPath, proc, verbose)
}

func performCutover(baseDir, activeVersion, newVersion, newPath string, stagingProc *exec.Cmd, verbose bool) error {
	fmt.Printf("\n  %s Cutting over from v%s to v%s...\n\n", ui.Cyan("→"), activeVersion, newVersion)

	if stagingProc != nil && stagingProc.Process != nil {
		if runtime.GOOS == "windows" {
			stagingProc.Process.Kill() //nolint:errcheck
		} else {
			stagingProc.Process.Signal(syscall.SIGTERM) //nolint:errcheck
		}
		time.Sleep(time.Second)
	}

	fmt.Print(ui.Dim("  Stopping v" + activeVersion + "..."))
	setup.KillPort(health.DefaultPort)
	setup.WaitForPortFree(health.DefaultPort, 15*time.Second)
	fmt.Printf("\r\033[2K  %s v%s stopped\n", ui.Green("✓"), activeVersion)

	fmt.Print(ui.Dim(fmt.Sprintf("  Starting %s v%s on port %d...", product.Name, newVersion, health.DefaultPort)))
	proc, err := setup.StartBackground(newPath, verbose)
	if err != nil {
		fmt.Println()
		return fmt.Errorf("failed to start %s: %w", product.Name, err)
	}

	// Persist state only after the new instance is confirmed running.
	if err := config.WriteInstallPath(newVersion, newPath); err != nil {
		return fmt.Errorf("failed to persist install path: %w", err)
	}
	if err := config.WriteActiveVersion(newVersion); err != nil {
		return fmt.Errorf("failed to update active version: %w", err)
	}
	fmt.Printf("\r\033[2K  %s %s v%s is now live on port %d\n", ui.Green("✓"), product.Name, newVersion, health.DefaultPort)

	_, _, err = ui.RunREPL(newVersion, proc, newPath, verbose, false, "", "", 0)
	return err
}

func downloadVersion(version, destDir string, verbose bool) error {
	fmt.Println()
	if verbose {
		if err := release.Download(version, destDir, func(pct int, msg string) {
			if pct < 0 {
				fmt.Println("  " + msg)
			} else {
				fmt.Printf("  %s  %d%%\n", msg, pct)
			}
		}); err != nil {
			ui.Fatal("Download failed: " + err.Error())
			return err
		}
	} else {
		if err := release.Download(version, destDir, func(pct int, msg string) {
			if pct < 0 {
				fmt.Printf("\r\033[2K  %s", msg)
			} else {
				fmt.Printf("\r\033[2K  %s  %s  %3d%%", spinner.Render(pct), msg, pct)
			}
		}); err != nil {
			fmt.Println()
			ui.Fatal("Download failed: " + err.Error())
			return err
		}
		fmt.Println()
	}
	fmt.Printf("  %s %s v%s installed to %s\n", ui.Green("✓"), product.Name, version, destDir)
	return nil
}

func runSetupWithPort(version, installPath string, verbose bool, port int) error {
	if verbose {
		fmt.Printf("\n  Running %s setup (v%s)...\n", product.Name, version)
		if err := setup.RunSetupOnPort(installPath, true, port); err != nil {
			ui.Fatal("Setup failed: " + err.Error())
			return err
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
			Title("Setting up " + product.Name + " v" + version + "...").
			Action(func() {
				setupErr = setup.RunSetupOnPort(installPath, false, port)
			}).
			Run(); err != nil {
			ui.Fatal("Setup interrupted: " + err.Error())
			return err
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
			return setupErr
		}
	}
	if err := config.MarkSetupComplete(version); err != nil {
		ui.Fatal("Failed to mark setup complete: " + err.Error())
		return err
	}
	fmt.Printf("  %s Setup complete\n", ui.Green("✓"))
	return nil
}

func versionedPath(baseDir, version string) string {
	return filepath.Join(baseDir, "v"+version)
}
