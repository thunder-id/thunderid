// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package upgrade

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/tools/cli/internal/services/config"
	"github.com/thunder-id/thunderid/tools/cli/internal/services/health"
)

func TestResolveLivePort_PrefersCallerPort(t *testing.T) {
	assert.Equal(t, 8095, resolveLivePort(Opts{Port: 8095}, "1.0.0"))
}

// installOnPort records an install whose deployment.yaml serves on port, with the state
// file redirected to a temp home so the developer's own install is not read.
func installOnPort(t *testing.T, version string, port int) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	installPath := filepath.Join(t.TempDir(), "v"+version)
	require.NoError(t, os.MkdirAll(installPath, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(installPath, "deployment.yaml"),
		[]byte("server:\n  hostname: \"localhost\"\n  port: "+strconv.Itoa(port)+"\n"), 0o644))
	require.NoError(t, config.WriteInstallPath(version, installPath))
}

// With no caller-resolved port the upgrade has to stop and restart on the port the
// install is configured for: stopping the default port would leave the running
// instance up and take an unrelated listener down instead.
func TestResolveLivePort_FallsBackToConfiguredPort(t *testing.T) {
	installOnPort(t, "1.0.0", 9443)
	assert.Equal(t, 9443, resolveLivePort(Opts{}, "1.0.0"))
}

// Nothing recorded for the version means nothing to read a port from, so the default
// is all that is left.
func TestResolveLivePort_FallsBackToDefaultWithoutAnInstall(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	assert.Equal(t, health.DefaultPort, resolveLivePort(Opts{}, "1.0.0"))
}

// health.WaitReady only reports a timeout, so the failure message has to carry what the
// server itself wrote or the user has nothing to act on.
func TestStartupFailure_IncludesLogTailAndPath(t *testing.T) {
	installPath := t.TempDir()
	logDir := filepath.Join(installPath, "logs")
	require.NoError(t, os.MkdirAll(logDir, 0o755))
	logPath := filepath.Join(logDir, "thunderid.log")
	require.NoError(t, os.WriteFile(logPath,
		[]byte("starting\nfailed to bind: address already in use\n"), 0o644))

	msg := startupFailure(installPath, errors.New("did not become ready"))

	assert.Contains(t, msg, "did not become ready")
	assert.Contains(t, msg, "address already in use", "the server's own reason must be shown")
	assert.Contains(t, msg, logPath, "the full log path must still be shown")
}

func TestStartupFailure_WithoutALogStillPointsAtTheLogPath(t *testing.T) {
	msg := startupFailure(t.TempDir(), errors.New("did not become ready"))

	assert.Contains(t, msg, "did not become ready")
	assert.Contains(t, msg, "logs:")
}
