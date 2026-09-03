// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package cli contains the default command that installs, sets up, and starts ThunderID.
package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	huhspinner "charm.land/huh/v2/spinner"

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

	// Never let a non-default release source be silent: an operator has to be able to see that
	// this run is not reading the public manifest.
	if release.IsCustomSource() {
		ui.Note("Custom release source", release.SourceURL())
	}

	activeVersion := config.ReadActiveVersion()
	pinnedVersion := release.Pinned()

	fmt.Print(ui.Dim("  Fetching latest " + product.Name + " release..."))
	latestVersion, err := release.FetchLatestVersion()
	switch {
	case err == nil:
		fmt.Printf("\r\033[2K  %s Latest %s release: v%s\n\n", ui.Green("✓"), product.Name, latestVersion)
	case activeVersion != "":
		// The manifest is only needed to learn about newer releases. With a version
		// already installed, an unreachable release server must not stop the CLI from
		// starting what is on disk.
		fmt.Print("\r\033[2K")
		ui.Warn("Could not reach the " + product.Name + " release server, so this run cannot check for updates.\n" +
			"Starting the installed v" + activeVersion + ".\n" + err.Error())
		latestVersion = activeVersion
	default:
		fmt.Println()
		ui.Fatal("Could not fetch latest " + product.Name + " release: " + err.Error() +
			"\nNo local install to fall back on, so " + product.Name + " cannot start offline.")
		os.Exit(1)
	}

	// Always start the active version; only download when there's no installed version yet.
	// A pinned version overrides both: it is an explicit instruction about what to run.
	runVersion := latestVersion
	if activeVersion != "" {
		runVersion = activeVersion
	}
	if pinnedVersion != "" {
		runVersion = pinnedVersion
	}

	// Show a new-version banner inside the REPL (not a blocking prompt). Pinning suppresses it:
	// the operator already said which version they want.
	var newVersion string
	if pinnedVersion == "" && activeVersion != "" && activeVersion != latestVersion &&
		!config.IsVersionSkipped(latestVersion) {
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
	// A pinned version is judged on its own install state rather than on which version happens to
	// be active: asking for a version that is already installed should start it, not fetch it again.
	selected := activeVersion == runVersion || pinnedVersion != ""
	alreadyInstalled := selected && config.IsSetupComplete(runVersion) && installOnDisk
	isFirstRun := !config.IsOnboardingDone(runVersion)

	var port int
	var creds *setup.AdminCredentials

	if alreadyInstalled && !forceSetup {
		ui.Note("Starting "+product.Name, fmt.Sprintf("%s v%s is ready\n%s", product.Name, runVersion, path))
		// Setup already ran on an earlier invocation, so the port it seeded into the
		// console application's redirect URIs is fixed: no alternate port here.
		port = resolvePort(path, false)
	} else if selected && installOnDisk {
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
		port = resolvePort(path, true)
		creds = runSetupPhase(runVersion, path, verbose, port)
	} else {
		// If the previously-active version is no longer in the manifest we'd need
		// to download it, so fall back to the latest available version. A pinned version is
		// never swapped out: failing to find it has to be reported, not worked around.
		if pinnedVersion == "" && runVersion != latestVersion {
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
		port = resolvePort(path, true)
		creds = runSetupPhase(runVersion, path, verbose, port)
		if err := config.WriteActiveVersion(runVersion); err != nil {
			ui.Fatal("Failed to record active version: " + err.Error())
			os.Exit(1)
		}
	}

	// A pinned run switches the active version, so a later run without the pin starts what was
	// last explicitly asked for instead of silently reverting to the previous one.
	if config.ReadActiveVersion() != runVersion {
		if err := config.WriteActiveVersion(runVersion); err != nil {
			ui.Fatal("Failed to record active version: " + err.Error())
			os.Exit(1)
		}
	}

	// resolvePort has already settled any conflict on this port, so anything still
	// listening was not approved for termination: report it instead of killing it.
	if !setup.WaitForPortFree(port, 10*time.Second) {
		ui.PrintCredentialsFallback(creds)
		ui.Fatal(fmt.Sprintf("Port %d is still in use. Stop the process using it, then run again.", port))
		os.Exit(1)
	}

	fmt.Print(ui.Dim("\n  Starting " + product.Name + " in the background..."))
	proc, err := setup.StartBackgroundOnPort(path, verbose, port)
	if err != nil {
		fmt.Println()
		// The REPL normally displays generated credentials; since we exit before it,
		// print them here so a fresh password is not lost (setup won't regenerate it).
		ui.PrintCredentialsFallback(creds)
		ui.Fatal("Failed to start " + product.Name + ": " + err.Error())
		os.Exit(1)
	}

	// Launching the script only means the launcher started. Wait for the server to
	// answer before claiming success, so a rejected bind is reported as the failure it
	// is instead of a started product the REPL then finds missing.
	if err := waitForStartup(path, port, verbose); err != nil {
		fmt.Println()
		stopStartedInstance(proc, port)
		ui.PrintCredentialsFallback(creds)
		ui.FatalDetail("Failed to start " + product.Name + ": " + err.Error())
		os.Exit(1)
	}
	fmt.Printf("\r\033[2K  %s %s started on port %d  %s\n",
		ui.Green("✓"), product.Name, port, ui.Dim("logs: "+setup.LogDir(path)))

	replLoop(runVersion, path, proc, verbose, isFirstRun, newVersion, nodeWarning, port, creds)
}

// replLoop runs the REPL repeatedly, re-entering after no-op upgrades or version switches.
// An actual upgrade or normal exit breaks the loop.
func replLoop(
	version, installPath string, proc *exec.Cmd, verbose, isFirstRun bool,
	newVersion, nodeWarning string, port int, creds *setup.AdminCredentials,
) {
	notice := ""
	for {
		upgradeRequested, switchRequested, err := ui.RunREPL(
			version, proc, installPath, verbose, isFirstRun, newVersion, nodeWarning, port, creds, notice)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\nREPL error: %v\n", err)
			os.Exit(1)
		}
		isFirstRun = false

		if upgradeRequested {
			upgraded, upgradeNotice, err := upgrade.Run(BaseDir(), upgrade.Opts{Verbose: verbose, Port: port})
			if err != nil {
				fmt.Fprintf(os.Stderr, "\nUpgrade failed: %v\n", err)
				os.Exit(1)
			}
			if upgraded {
				return // upgrade ran its own REPL internally
			}
			// Already latest or cancelled — reattach to the still-running instance,
			// surfacing why inside the REPL rather than printing to a screen the
			// next alternate-screen redraw would immediately hide.
			notice = upgradeNotice
			continue
		}

		if switchRequested {
			switched, switchNotice, err := upgrade.Switch(BaseDir(), version, port, verbose, newVersion)
			if err != nil {
				fmt.Fprintf(os.Stderr, "\nSwitch failed: %v\n", err)
				os.Exit(1)
			}
			if switched {
				return // Switch ran its own REPL internally
			}
			// Cancelled or unavailable — reattach to the still-running instance,
			// surfacing why inside the REPL rather than printing to a screen the
			// next alternate-screen redraw would immediately hide.
			notice = switchNotice
			continue
		}

		return // normal exit
	}
}

// resolvePort returns the port ThunderID should run on.
//
// Without setup in this invocation the baseline is the port configured in the install's
// deployment.yaml — the only port the server itself honors — so a previous run that moved
// the port is carried forward instead of health-checking a port nothing listens on. A
// conflict there is answered by freeing the port or aborting: setup seeded the console
// application with redirect URIs carrying its port, so a run on a different port would
// serve a console that cannot complete a login.
//
// With setup in this invocation (setupWillRun) the baseline is the default port, since
// setup re-seeds those redirect URIs from the port it runs on: a re-setup returns to the
// default instead of inheriting a port an earlier conflict moved it to. A conflict here
// can also be answered by moving to a free port.
//
// Whatever holds the port is left running until the user says otherwise. A process
// answering on it is not necessarily this install: it may be another install, another
// version, or an unrelated app.
func resolvePort(installPath string, setupWillRun bool) int {
	port := setup.ServerPort(installPath)
	if setupWillRun {
		port = health.DefaultPort
	}
	if !setup.IsPortInUse(port) {
		return port
	}
	holders := setup.PortHolders(port)
	if !ui.Interactive() {
		// A scripted or CI run cannot answer the prompt, so take the port as before
		// rather than stalling, and say what is being stopped.
		warning := fmt.Sprintf("Port %d is in use, so the process holding it is being stopped.", port)
		if summary := holderSummary(holders); summary != "" {
			warning += "\n" + summary
		}
		ui.Warn(warning)
		freePortOrExit(port)
		return port
	}
	altPort := 0
	if setupWillRun {
		altPort = setup.FindFreePort(port + 1)
	}
	choice, selectedPort := ui.PromptPortConflict(port, altPort, holders)
	switch choice {
	case ui.KillAndUsePort:
		freePortOrExit(port)
		return port
	case ui.UseAlternatePort:
		return selectedPort
	default: // ui.AbortSetup
		os.Exit(0)
		return 0
	}
}

// freePortOrExit stops whatever holds port and gives up when it cannot: starting
// anyway would fail on the bind with a less useful message.
func freePortOrExit(port int) {
	if err := setup.FreePort(port, 10*time.Second); err != nil {
		ui.Fatal(fmt.Sprintf("Could not free port %d: %s\nStop the process manually, then run again.", port, err))
		os.Exit(1)
	}
}

// holderSummary lists the processes holding a port, one per line, for the warning
// shown where the prompt with its own list cannot run.
func holderSummary(holders []setup.PortHolder) string {
	lines := make([]string, 0, len(holders))
	for _, h := range holders {
		lines = append(lines, h.String())
	}
	return strings.Join(lines, "\n")
}

// startupTimeout bounds how long the CLI waits for a freshly started server to answer
// before it reports the start as failed.
const startupTimeout = 90 * time.Second

// fatalStartupPatterns are log lines that mean the server will not come up, so the CLI
// can report the real cause immediately instead of waiting out startupTimeout.
var fatalStartupPatterns = []string{
	"address already in use",
	"is already in use",
	"panic:",
	"failed to start",
}

// stopStartedInstance stops what this run launched after startup verification failed, so
// the launcher and the server it spawned do not outlive the CLI and hold the port. The
// port check before the launch proved nothing else held it, so a listener on it now is
// this run's own.
func stopStartedInstance(proc *exec.Cmd, port int) {
	if proc != nil && proc.Process != nil {
		if runtime.GOOS == "windows" {
			proc.Process.Kill() //nolint:errcheck
		} else {
			proc.Process.Signal(syscall.SIGTERM) //nolint:errcheck
		}
	}
	if setup.IsPortInUse(port) {
		_ = setup.FreePort(port, 5*time.Second)
	}
}

// waitForStartup waits for the server to answer, showing a spinner in non-verbose mode
// so a slow boot does not look like a hang. Verbose runs keep the raw log output.
func waitForStartup(installPath string, port int, verbose bool) error {
	if verbose {
		return awaitReady(installPath, port, startupTimeout)
	}
	fmt.Println()
	var readyErr error
	if err := huhspinner.New().
		WithTheme(ui.SpinnerTheme()).
		Title("Waiting for " + product.Name + " to become ready...").
		Action(func() {
			readyErr = awaitReady(installPath, port, startupTimeout)
		}).
		Run(); err != nil {
		return fmt.Errorf("interrupted while waiting for %s: %w", product.Name, err)
	}
	return readyErr
}

// awaitReady blocks until the server answers on port, and otherwise returns the real
// reason it did not: the server's own log line when it names one, or a timeout that
// still carries the tail of that log.
func awaitReady(installPath string, port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if health.IsReady(port) {
			return nil
		}
		if line := fatalStartupLine(installPath); line != "" {
			return withLogTail(installPath, errors.New(line))
		}
		if !time.Now().Before(deadline) {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if setup.IsPortInUse(port) {
		return withLogTail(installPath,
			fmt.Errorf("port %d is in use but %s never became ready", port, product.Name))
	}
	return withLogTail(installPath,
		fmt.Errorf("%s did not start listening on port %d within %s", product.Name, port, timeout))
}

// fatalStartupLine returns the log line that shows the server gave up, or "".
func fatalStartupLine(installPath string) string {
	tail := setup.LogTail(installPath, 40)
	for _, line := range strings.Split(tail, "\n") {
		lower := strings.ToLower(line)
		for _, pattern := range fatalStartupPatterns {
			if strings.Contains(lower, pattern) {
				return strings.TrimSpace(line)
			}
		}
	}
	return ""
}

// withLogTail appends the tail of the server log to err, so the failure box shows what
// the server itself reported.
func withLogTail(installPath string, err error) error {
	tail := setup.LogTail(installPath, 8)
	if tail == "" {
		return fmt.Errorf("%w\n\nlogs: %s", err, setup.LatestLogFile(installPath))
	}
	return fmt.Errorf("%w\n\n%s\n\nlogs: %s", err, tail, setup.LatestLogFile(installPath))
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

// collectAdminCredentials prompts for the admin username and password before setup,
// showing the default username and a freshly generated password as the values that
// will be used if nothing is entered (mirroring setup.sh). It exports the results via
// the THUNDERID_ADMIN_* env vars that setup consumes.
//
// It is skipped when both values are already provided via the environment (scripted
// runs) or when stdin is not interactive; in those cases setup.sh generates and
// surfaces the password itself. When the operator accepts the generated password it
// is returned so the REPL can display it; a typed password returns nil (the operator
// already knows it), matching setup.sh's notice behavior.
func collectAdminCredentials() *setup.AdminCredentials {
	if os.Getenv("THUNDERID_ADMIN_USERNAME") != "" && os.Getenv("THUNDERID_ADMIN_PASSWORD") != "" {
		return nil
	}
	defaultUser := os.Getenv("THUNDERID_ADMIN_USERNAME")
	if defaultUser == "" {
		defaultUser = "admin"
	}
	username, password, useGenerated, ok := ui.PromptAdminCredentials(defaultUser, defaultAdminPassword())
	if !ok {
		return nil
	}
	if err := os.Setenv("THUNDERID_ADMIN_USERNAME", username); err != nil {
		ui.Fatal("Failed to set admin username: " + err.Error())
		os.Exit(1)
	}
	if err := os.Setenv("THUNDERID_ADMIN_PASSWORD", password); err != nil {
		ui.Fatal("Failed to set admin password: " + err.Error())
		os.Exit(1)
	}
	if useGenerated {
		return &setup.AdminCredentials{Username: username, Password: password}
	}
	return nil
}

// defaultAdminPassword returns the password to offer as the prompt default: a
// pre-configured THUNDERID_ADMIN_PASSWORD if set (so accepting the prompt keeps it
// instead of overwriting it with a fresh one), otherwise a freshly generated one.
func defaultAdminPassword() string {
	if p := os.Getenv("THUNDERID_ADMIN_PASSWORD"); p != "" {
		return p
	}
	return setup.GenerateAdminPassword()
}

// runSetupPhase runs setup.sh on the resolved port with a spinner (non-verbose) or
// raw output (verbose). On failure in non-verbose mode, the captured stderr is
// printed before the error box.
// It returns the generated admin credentials when setup produced them, or nil.
func runSetupPhase(version, installPath string, verbose bool, port int) *setup.AdminCredentials {
	promptCreds := collectAdminCredentials()
	var setupCreds *setup.AdminCredentials
	if verbose {
		fmt.Printf("\n  Running %s setup (v%s)...\n", product.Name, version)
		c, err := setup.RunSetupOnPort(installPath, true, port)
		if err != nil {
			ui.Fatal("Setup failed: " + err.Error())
			os.Exit(1)
		}
		setupCreds = c
	} else {
		fmt.Println()
		var setupErr error
		if err := huhspinner.New().
			WithTheme(ui.SpinnerTheme()).
			Title("Setting up " + product.Name + "...").
			Action(func() {
				setupCreds, setupErr = setup.RunSetupOnPort(installPath, false, port)
			}).
			Run(); err != nil {
			ui.Fatal("Setup interrupted: " + err.Error())
			os.Exit(1)
		}
		if setupErr != nil {
			ui.FatalDetail(setupErr.Error())
			os.Exit(1)
		}
	}

	if err := config.MarkSetupComplete(version); err != nil {
		creds := setupCreds
		if promptCreds != nil {
			creds = promptCreds
		}
		ui.PrintCredentialsFallback(creds)
		ui.Fatal("Failed to mark setup complete: " + err.Error())
		os.Exit(1)
	}
	fmt.Printf("  %s Setup complete\n", ui.Green("✓"))
	// Prefer the interactively collected credentials; fall back to whatever setup.sh
	// generated and printed (non-interactive path).
	if promptCreds != nil {
		return promptCreds
	}
	return setupCreds
}
