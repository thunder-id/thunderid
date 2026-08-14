// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thunder-id/thunderid/tools/cli/internal/services/health"
	"github.com/thunder-id/thunderid/tools/cli/internal/services/setup"
)

// A re-setup re-seeds the console application's redirect URIs, so it starts from the
// default port instead of inheriting the port an earlier conflict moved the install to.
func TestResolvePort_SetupStartsFromTheDefaultPort(t *testing.T) {
	if setup.IsPortInUse(health.DefaultPort) {
		t.Skipf("port %d is in use on this machine", health.DefaultPort)
	}
	dir := t.TempDir()
	writeDeploymentPort(t, dir, 8091)

	if got := resolvePort(dir, true); got != health.DefaultPort {
		t.Fatalf("expected setup to start from port %d, got %d", health.DefaultPort, got)
	}
}

// A plain start honors the configured port: that is the port the server binds and the
// port setup wrote into the console application's redirect URIs.
func TestResolvePort_StartKeepsTheConfiguredPort(t *testing.T) {
	if setup.IsPortInUse(19991) {
		t.Skip("port 19991 is in use on this machine")
	}
	dir := t.TempDir()
	writeDeploymentPort(t, dir, 19991)

	if got := resolvePort(dir, false); got != 19991 {
		t.Fatalf("expected the configured port 19991, got %d", got)
	}
}

func writeDeploymentPort(t *testing.T, dir string, port int) {
	t.Helper()
	content := fmt.Sprintf("server:\n  hostname: \"localhost\"\n  port: %d\n", port)
	if err := os.WriteFile(filepath.Join(dir, "deployment.yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write deployment.yaml: %v", err)
	}
}

func TestDefaultAdminPassword_UsesPresetEnv(t *testing.T) {
	t.Setenv("THUNDERID_ADMIN_PASSWORD", "Preset#1Pass")
	if got := defaultAdminPassword(); got != "Preset#1Pass" {
		t.Fatalf("expected pre-set password to be reused, got %q", got)
	}
}

func TestDefaultAdminPassword_GeneratesWhenUnset(t *testing.T) {
	t.Setenv("THUNDERID_ADMIN_PASSWORD", "")
	if got := defaultAdminPassword(); len(got) != 12 {
		t.Fatalf("expected a generated 12-char password, got %q (len %d)", got, len(got))
	}
}

func TestCollectAdminCredentials_SkipsWhenBothPreset(t *testing.T) {
	t.Setenv("THUNDERID_ADMIN_USERNAME", "operator")
	t.Setenv("THUNDERID_ADMIN_PASSWORD", "Preset#1Pass")
	if creds := collectAdminCredentials(); creds != nil {
		t.Fatalf("expected nil when both env vars are preset, got %+v", creds)
	}
}

func TestCollectAdminCredentials_NonInteractiveReturnsNil(t *testing.T) {
	// Under `go test` stdin is not a character device, so the interactive prompt
	// is skipped and the function falls through to setup's own defaults.
	t.Setenv("THUNDERID_ADMIN_USERNAME", "")
	t.Setenv("THUNDERID_ADMIN_PASSWORD", "")
	if creds := collectAdminCredentials(); creds != nil {
		t.Fatalf("expected nil on non-interactive stdin, got %+v", creds)
	}
}

// writeServerLog writes body as the install's current background log.
func writeServerLog(t *testing.T, installPath, body string) {
	t.Helper()
	if err := os.MkdirAll(setup.LogDir(installPath), 0o755); err != nil {
		t.Fatalf("create log dir: %v", err)
	}
	if err := os.WriteFile(setup.LogFile(installPath), []byte(body), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}
}

func TestAwaitReady_ReportsFatalLogLineWithoutWaitingOut(t *testing.T) {
	dir := t.TempDir()
	writeServerLog(t, dir, "starting up\nlisten tcp 127.0.0.1:19991: bind: address already in use\n")

	start := time.Now()
	err := awaitReady(dir, 19991, 30*time.Second)

	if err == nil {
		t.Fatal("expected a failure when the log shows the bind was rejected")
	}
	if !strings.Contains(err.Error(), "address already in use") {
		t.Fatalf("expected the server's own log line, got %q", err)
	}
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Fatalf("expected a fast failure, waited %s", elapsed)
	}
}

func TestAwaitReady_TimeoutCarriesLogTailAndPath(t *testing.T) {
	dir := t.TempDir()
	writeServerLog(t, dir, "line one\nstill booting\n")

	err := awaitReady(dir, 19992, 100*time.Millisecond)

	if err == nil {
		t.Fatal("expected a timeout failure")
	}
	for _, want := range []string{"did not start listening on port 19992", "still booting", setup.LogFile(dir)} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("expected %q in %q", want, err)
		}
	}
}

func TestFatalStartupLine_IgnoresOrdinaryOutput(t *testing.T) {
	dir := t.TempDir()
	writeServerLog(t, dir, "server starting\nlistening on 8090\n")

	if line := fatalStartupLine(dir); line != "" {
		t.Fatalf("expected no fatal line, got %q", line)
	}
}
