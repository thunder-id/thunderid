// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package upgrade orchestrates in-place version upgrades and version switching.
package upgrade

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"charm.land/huh/v2"
	huhspinner "charm.land/huh/v2/spinner"

	"github.com/thunder-id/thunderid/tools/cli/internal/product"
	"github.com/thunder-id/thunderid/tools/cli/internal/services/config"
	"github.com/thunder-id/thunderid/tools/cli/internal/services/health"
	"github.com/thunder-id/thunderid/tools/cli/internal/services/release"
	"github.com/thunder-id/thunderid/tools/cli/internal/services/setup"
	"github.com/thunder-id/thunderid/tools/cli/internal/ui"
	"github.com/thunder-id/thunderid/tools/cli/internal/ui/spinner"
)

// startupTimeout bounds how long a freshly started instance gets to answer before the
// upgrade reports the start as failed.
const startupTimeout = 90 * time.Second

// Opts controls how the upgrade runs.
type Opts struct {
	Verbose bool
	Port    int // port the running instance serves on; 0 resolves it from config
}

// resolveLivePort returns the port the running instance serves on: the port the
// caller already resolved, else the active install's configured port (deployment.yaml
// is what the server reads), else the default.
func resolveLivePort(opts Opts, activeVersion string) int {
	if opts.Port > 0 {
		return opts.Port
	}
	return setup.ServerPort(config.ReadInstallPath(activeVersion))
}

// Run executes the upgrade workflow. baseDir is the parent thunderid directory (e.g. "./thunderid").
//
// When upgraded is false, notice (if non-empty) explains why, styled for display as the
// first message in the REPL the caller reattaches to — printing it directly here would be
// hidden the moment that REPL's alternate screen takes over the terminal again.
func Run(baseDir string, opts Opts) (upgraded bool, notice string, err error) {
	fmt.Print(ui.Dim("  Fetching latest " + product.Name + " release..."))
	latestVersion, err := release.FetchLatestVersion()
	if err != nil {
		// Not being able to check for updates is not a broken upgrade: report it and
		// leave the running instance alone, so an offline /upgrade does not end the session.
		fmt.Print("\r\033[2K")
		return false, ui.Yellow("⚠") + " Could not check for a newer " + product.Name + " release.\n" + err.Error(), nil
	}
	fmt.Printf("\r\033[2K  %s Latest %s release: v%s\n\n", ui.Green("✓"), product.Name, latestVersion)

	activeVersion := config.ReadActiveVersion()
	if activeVersion == latestVersion {
		return false, ui.Green("✓") + " " + product.Name + " v" + latestVersion + " is already the latest version.", nil
	}

	if activeVersion != "" {
		fmt.Printf("  Upgrading %s: %s → %s\n\n",
			product.Name,
			ui.Dim("v"+activeVersion),
			ui.Green("v"+latestVersion),
		)
	}

	livePort := resolveLivePort(opts, activeVersion)

	return true, "", runUpgrade(baseDir, activeVersion, latestVersion, opts.Verbose, livePort)
}

// Switch stops the running ThunderID instance and starts the selected installed version
// on the same port. It shows an interactive version picker and returns false if the user
// cancels or no other versions are installed. On success it starts the new instance and
// runs a REPL for it.
//
// When switched is false, notice (if non-empty) explains why, styled for display as the
// first message in the REPL the caller reattaches to — printing it directly here would be
// hidden the moment that REPL's alternate screen takes over the terminal again.
func Switch(baseDir, currentVersion string, livePort int, verbose bool, pendingUpgrade string) (switched bool, notice string, err error) {
	versions := config.ListInstalledVersions(currentVersion)
	if len(versions) == 0 {
		return false, ui.Yellow("⚠") + " No other installed versions found. Use " + ui.Cyan("/upgrade") + " to install a new version.", nil
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
		return false, ui.Dim("Switch cancelled."), nil
	}

	installPath := config.ReadInstallPath(selected)
	if installPath == "" {
		return false, ui.Red("✗") + " Install path not found for v" + selected + ". Re-run setup to restore it.", nil
	}

	// Validate the install is launchable before touching the running instance.
	if _, err := setup.FindThunderRoot(installPath); err != nil {
		return false, ui.Red("✗") + fmt.Sprintf(" v%s is not usable (%s). The install may have been moved or deleted.", selected, err), nil
	}

	if livePort <= 0 {
		livePort = resolveLivePort(Opts{}, currentVersion)
	}

	fmt.Print(ui.Dim("  Stopping " + product.Name + " v" + currentVersion + "..."))
	if err := setup.FreePort(livePort, 10*time.Second); err != nil {
		fmt.Println()
		ui.Fatal("Could not stop v" + currentVersion + ": " + err.Error())
		return false, "", err
	}
	fmt.Printf("\r\033[2K  %s Stopped v%s\n", ui.Green("✓"), currentVersion)

	fmt.Print(ui.Dim("\n  Starting " + product.Name + " v" + selected + "..."))
	proc, err := setup.StartBackgroundOnPort(installPath, verbose, livePort)
	if err != nil {
		fmt.Println()
		ui.Fatal("Failed to start v" + selected + ": " + err.Error())
		return false, "", err
	}
	if err := health.WaitReady(livePort, startupTimeout); err != nil {
		stopStartedProcess(proc, livePort)
		fmt.Println()
		ui.Fatal(fmt.Sprintf("Failed to start v%s: %s", selected, startupFailure(installPath, err)))
		return false, "", fmt.Errorf("failed to start v%s: %w", selected, err)
	}
	// Persist the active version only once the new instance is up, so a failed
	// start does not leave the recorded version pointing at nothing.
	if err := config.WriteActiveVersion(selected); err != nil {
		return false, "", fmt.Errorf("failed to update active version: %w", err)
	}
	fmt.Printf("\r\033[2K  %s Switched to %s v%s  %s\n", ui.Green("✓"), product.Name, selected, ui.Dim("logs: "+setup.LogDir(installPath)))

	_, _, err = ui.RunREPL(selected, proc, installPath, verbose, false, pendingUpgrade, "", livePort, nil, "")
	return true, "", err
}

func runUpgrade(baseDir, activeVersion, newVersion string, verbose bool, livePort int) error {
	label := product.Name
	if activeVersion != "" {
		label = product.Name + " v" + activeVersion
	}
	// With no active version there is nothing of ours to stop: the resolved live port is
	// only the default, and whatever listens on it belongs to something else.
	if activeVersion != "" {
		fmt.Print(ui.Dim("  Stopping " + label + "..."))
		if err := setup.FreePort(livePort, 15*time.Second); err != nil {
			fmt.Println()
			ui.Fatal("Could not stop " + label + ": " + err.Error())
			return err
		}
		fmt.Printf("\r\033[2K  %s Stopped v%s\n", ui.Green("✓"), activeVersion)
	}

	newPath := versionedPath(baseDir, newVersion)
	if err := downloadVersion(newVersion, newPath, verbose); err != nil {
		return err
	}
	creds, err := runSetupWithPort(newVersion, newPath, verbose, livePort)
	if err != nil {
		return err
	}

	fmt.Print(ui.Dim("\n  Starting " + product.Name + " v" + newVersion + "..."))
	proc, err := setup.StartBackgroundOnPort(newPath, verbose, livePort)
	if err != nil {
		fmt.Println()
		ui.PrintCredentialsFallback(creds)
		ui.Fatal("Failed to start " + product.Name + ": " + err.Error())
		return err
	}
	if err := health.WaitReady(livePort, startupTimeout); err != nil {
		stopStartedProcess(proc, livePort)
		fmt.Println()
		ui.PrintCredentialsFallback(creds)
		ui.Fatal(fmt.Sprintf("Failed to start %s: %s", product.Name, startupFailure(newPath, err)))
		return fmt.Errorf("failed to start %s: %w", product.Name, err)
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

	_, _, err = ui.RunREPL(newVersion, proc, newPath, verbose, false, "", "", livePort, creds, "")
	return err
}

// startupFailure describes a failed start: the readiness error, what the server itself
// last wrote, and where the full log is. health.WaitReady only reports a timeout, so
// without the tail the user is left with a log path to open by hand.
func startupFailure(installPath string, err error) string {
	logPath := setup.LatestLogFile(installPath)
	if tail := setup.LogTail(installPath, 8); tail != "" {
		return fmt.Sprintf("%s\n\n%s\n\nlogs: %s", err, tail, logPath)
	}
	return fmt.Sprintf("%s\nlogs: %s", err, logPath)
}

// stopStartedProcess stops what this command launched on port after it failed to become
// ready. start.sh traps SIGTERM and stops the server it backgrounded, but the trap does
// not run when the launcher is killed outright on Windows, and it does not wait for the
// port to be released either; freeing the port makes the stop definite. Nothing else can
// be on that port here: it was ours to start on.
func stopStartedProcess(proc *exec.Cmd, port int) {
	if proc != nil && proc.Process != nil {
		if runtime.GOOS == "windows" {
			_ = proc.Process.Kill()
		} else {
			_ = proc.Process.Signal(syscall.SIGTERM)
		}
	}
	if setup.IsPortInUse(port) {
		_ = setup.FreePort(port, 5*time.Second)
	}
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

func runSetupWithPort(version, installPath string, verbose bool, port int) (*setup.AdminCredentials, error) {
	var creds *setup.AdminCredentials
	if verbose {
		fmt.Printf("\n  Running %s setup (v%s)...\n", product.Name, version)
		c, err := setup.RunSetupOnPort(installPath, true, port)
		if err != nil {
			ui.Fatal("Setup failed: " + err.Error())
			return nil, err
		}
		creds = c
	} else {
		fmt.Println()
		var setupErr error
		if err := huhspinner.New().
			WithTheme(ui.SpinnerTheme()).
			Title("Setting up " + product.Name + " v" + version + "...").
			Action(func() {
				creds, setupErr = setup.RunSetupOnPort(installPath, false, port)
			}).
			Run(); err != nil {
			ui.Fatal("Setup interrupted: " + err.Error())
			return nil, err
		}
		if setupErr != nil {
			ui.FatalDetail(setupErr.Error())
			return nil, setupErr
		}
	}
	if err := config.MarkSetupComplete(version); err != nil {
		ui.PrintCredentialsFallback(creds)
		ui.Fatal("Failed to mark setup complete: " + err.Error())
		return nil, err
	}
	fmt.Printf("  %s Setup complete\n", ui.Green("✓"))
	return creds, nil
}

func versionedPath(baseDir, version string) string {
	return filepath.Join(baseDir, "v"+version)
}
