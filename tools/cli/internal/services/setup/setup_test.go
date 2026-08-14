// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package setup_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/tools/cli/internal/services/health"
	"github.com/thunder-id/thunderid/tools/cli/internal/services/setup"
)

// setupScript returns the platform-appropriate setup script filename.
func setupScript() string {
	if runtime.GOOS == "windows" {
		return "setup.ps1"
	}
	return "setup.sh"
}

func TestLogDir(t *testing.T) {
	base := t.TempDir()
	dir := setup.LogDir(base)
	assert.Equal(t, filepath.Join(base, "logs"), dir)
}

func TestLogFile_UnderLogDir(t *testing.T) {
	installPath := t.TempDir()
	f := setup.LogFile(installPath)
	assert.True(t, strings.HasPrefix(f, setup.LogDir(installPath)+string(os.PathSeparator)),
		"LogFile should be inside LogDir, got %q", f)
}

func TestLogFile_ContainsDate(t *testing.T) {
	f := setup.LogFile("/tmp/test")
	today := time.Now().Format("2006-01-02")
	assert.Contains(t, f, today, "log file name should contain today's date")
}

func TestLogFile_HasLogExtension(t *testing.T) {
	f := setup.LogFile("/tmp/test")
	assert.True(t, strings.HasSuffix(f, ".log"), "expected .log suffix, got %q", f)
}

// A start that crosses midnight keeps writing to the file it truncated the day
// before, so startup diagnostics must follow the newest log, not today's date.
func TestLatestLogFile_PrefersTheNewestLog(t *testing.T) {
	installPath := t.TempDir()
	logs := setup.LogDir(installPath)
	require.NoError(t, os.MkdirAll(logs, 0o755))

	yesterday := filepath.Join(logs, "thunderid-2026-08-12.log")
	require.NoError(t, os.WriteFile(yesterday, []byte("address already in use\n"), 0o644))
	stale := filepath.Join(logs, "thunderid-2026-08-11.log")
	require.NoError(t, os.WriteFile(stale, []byte("old\n"), 0o644))
	old := time.Now().Add(-48 * time.Hour)
	require.NoError(t, os.Chtimes(stale, old, old))

	assert.Equal(t, yesterday, setup.LatestLogFile(installPath))
	assert.Contains(t, setup.LogTail(installPath, 5), "address already in use")
}

func TestLatestLogFile_FallsBackToTodaysPath(t *testing.T) {
	installPath := t.TempDir()
	assert.Equal(t, setup.LogFile(installPath), setup.LatestLogFile(installPath))
}

func TestFindThunderRoot_ScriptAtRoot(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, setupScript()), []byte(""), 0o644))

	root, err := setup.FindThunderRoot(dir)
	require.NoError(t, err)
	assert.Equal(t, dir, root)
}

func TestFindThunderRoot_ScriptInSubdir(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "inner")
	require.NoError(t, os.MkdirAll(sub, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(sub, setupScript()), []byte(""), 0o644))

	root, err := setup.FindThunderRoot(dir)
	require.NoError(t, err)
	assert.Equal(t, sub, root)
}

func TestFindThunderRoot_Missing(t *testing.T) {
	dir := t.TempDir()
	_, err := setup.FindThunderRoot(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "setup script not found")
}

func TestWaitForPortFree_UnoccupiedPort(t *testing.T) {
	// Port 19999 is very unlikely to be in use; if nothing is listening, the
	// function should detect a free port on the first probe and return true.
	free := setup.WaitForPortFree(19999, 2*time.Second)
	assert.True(t, free, "expected unoccupied port to be detected as free")
}

func TestIsPortInUse_FreePort(t *testing.T) {
	assert.False(t, setup.IsPortInUse(19998), "expected unused port to not be in use")
}

func TestFindFreePort_ReturnsUnoccupied(t *testing.T) {
	port := setup.FindFreePort(19990)
	assert.False(t, setup.IsPortInUse(port), "FindFreePort should return a port that is not in use")
}

func TestUpdateServerPort_UpdatesDeploymentYAML(t *testing.T) {
	dir := t.TempDir()
	serverDir := filepath.Join(dir, "backend", "cmd", "server")
	require.NoError(t, os.MkdirAll(serverDir, 0o755))

	content := "server:\n  hostname: \"localhost\"\n  port: 8090\n\nother:\n  port: 9000\n"
	require.NoError(t, os.WriteFile(filepath.Join(serverDir, "deployment.yaml"), []byte(content), 0o644))

	require.NoError(t, setup.UpdateServerPort(dir, 8091))

	updated, err := os.ReadFile(filepath.Join(serverDir, "deployment.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(updated), "port: 8091")
	assert.Contains(t, string(updated), "port: 9000", "non-server port should be unchanged")
}

func TestUpdateServerPort_RootDeploymentYAMLTakesPrecedence(t *testing.T) {
	dir := t.TempDir()
	rootYAML := filepath.Join(dir, "deployment.yaml")
	require.NoError(t, os.WriteFile(rootYAML, []byte("server:\n  port: 8090\n"), 0o644))

	require.NoError(t, setup.UpdateServerPort(dir, 8092))

	data, err := os.ReadFile(rootYAML)
	require.NoError(t, err)
	assert.Contains(t, string(data), "port: 8092")
}

func TestUpdateServerPort_MissingConfig(t *testing.T) {
	dir := t.TempDir()
	err := setup.UpdateServerPort(dir, 8091)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deployment.yaml not found")
}

// writeDeployment writes a deployment.yaml at the install root and returns its path.
func writeDeployment(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "deployment.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func TestReadServerPort_ReadsConfiguredPort(t *testing.T) {
	dir := t.TempDir()
	writeDeployment(t, dir, "server:\n  hostname: \"localhost\"\n  port: 8091\n\nother:\n  port: 9000\n")

	assert.Equal(t, 8091, setup.ReadServerPort(dir))
}

// A port: belonging to a mapping nested under server: is a different setting and
// must not be mistaken for server.port, even when it comes first.
func TestReadServerPort_IgnoresPortNestedUnderServer(t *testing.T) {
	dir := t.TempDir()
	writeDeployment(t, dir, "server:\n  tls:\n    enabled: true\n    port: 9443\n  port: 8093\n")

	assert.Equal(t, 8093, setup.ReadServerPort(dir))
}

func TestUpdateServerPort_LeavesPortNestedUnderServer(t *testing.T) {
	dir := t.TempDir()
	path := writeDeployment(t, dir, "server:\n  tls:\n    port: 9443\n  port: 8090\n")

	require.NoError(t, setup.UpdateServerPort(dir, 8094))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "  port: 8094")
	assert.Contains(t, string(data), "    port: 9443", "the nested port should be unchanged")
}

func TestReadServerPort_IgnoresInlineComment(t *testing.T) {
	dir := t.TempDir()
	writeDeployment(t, dir, "server:\n  port: 8095 # moved after a conflict\n")

	assert.Equal(t, 8095, setup.ReadServerPort(dir))
}

func TestReadServerPort_NestedConfig(t *testing.T) {
	dir := t.TempDir()
	serverDir := filepath.Join(dir, "backend", "cmd", "server")
	require.NoError(t, os.MkdirAll(serverDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(serverDir, "deployment.yaml"),
		[]byte("server:\n  port: 8092\n"), 0o644))

	assert.Equal(t, 8092, setup.ReadServerPort(dir))
}

func TestReadServerPort_UnknownReturnsZero(t *testing.T) {
	assert.Equal(t, 0, setup.ReadServerPort(t.TempDir()), "no deployment.yaml")
	assert.Equal(t, 0, setup.ReadServerPort(""), "no install path")

	dir := t.TempDir()
	writeDeployment(t, dir, "server:\n  hostname: \"localhost\"\n")
	assert.Equal(t, 0, setup.ReadServerPort(dir), "no server.port key")

	other := t.TempDir()
	writeDeployment(t, other, "server:\n  port: not-a-number\n")
	assert.Equal(t, 0, setup.ReadServerPort(other), "unparseable port")
}

func TestEnsureServerPort_WritesRequestedPort(t *testing.T) {
	dir := t.TempDir()
	writeDeployment(t, dir, "server:\n  port: 8090\n")

	require.NoError(t, setup.EnsureServerPort(dir, 8091))
	assert.Equal(t, 8091, setup.ReadServerPort(dir))
}

func TestEnsureServerPort_NoOpWhenAlreadySet(t *testing.T) {
	dir := t.TempDir()
	// A comment on the port line survives only if the file is left untouched.
	path := writeDeployment(t, dir, "server:\n  port: 8090 # keep me\n")

	require.NoError(t, setup.EnsureServerPort(dir, 8090))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "# keep me")
}

func TestEnsureServerPort_ZeroKeepsConfiguredPort(t *testing.T) {
	dir := t.TempDir()
	writeDeployment(t, dir, "server:\n  port: 8093\n")

	require.NoError(t, setup.EnsureServerPort(dir, 0))
	assert.Equal(t, 8093, setup.ReadServerPort(dir))
}

func TestEnsureServerPort_MissingConfig(t *testing.T) {
	err := setup.EnsureServerPort(t.TempDir(), 8091)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deployment.yaml not found")
}

func TestLogTail_ReturnsLastLines(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(setup.LogDir(dir), 0o755))
	body := "first\nsecond\nthird\nfourth\n"
	require.NoError(t, os.WriteFile(setup.LogFile(dir), []byte(body), 0o644))

	assert.Equal(t, "third\nfourth", setup.LogTail(dir, 2), "the trailing newline must not become a blank line")
	assert.Equal(t, "first\nsecond\nthird\nfourth", setup.LogTail(dir, 10))
}

func TestTailFile_KeepsInteriorBlankLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.log")
	require.NoError(t, os.WriteFile(path, []byte("one\n\ntwo\r\n"), 0o644))

	assert.Equal(t, []string{"one", "", "two"}, setup.TailFile(path, 10))
}

func TestTailFile_EmptyOrMissing(t *testing.T) {
	assert.Empty(t, setup.TailFile(filepath.Join(t.TempDir(), "missing.log"), 5))

	empty := filepath.Join(t.TempDir(), "empty.log")
	require.NoError(t, os.WriteFile(empty, nil, 0o644))
	assert.Empty(t, setup.TailFile(empty, 5))
}

func TestTailFile_NonPositiveCount(t *testing.T) {
	assert.Nil(t, setup.TailFile(filepath.Join(t.TempDir(), "missing.log"), 0))
	assert.Nil(t, setup.TailFile(filepath.Join(t.TempDir(), "missing.log"), -1))
}

func TestLogTail_MissingLog(t *testing.T) {
	assert.Equal(t, "", setup.LogTail(t.TempDir(), 5))
}

func TestServerPort_PrefersConfiguredPort(t *testing.T) {
	dir := t.TempDir()
	writeDeployment(t, dir, "server:\n  port: 8091\n")

	assert.Equal(t, 8091, setup.ServerPort(dir))
}

func TestServerPort_FallsBackToDefault(t *testing.T) {
	assert.Equal(t, health.DefaultPort, setup.ServerPort(t.TempDir()))
	assert.Equal(t, health.DefaultPort, setup.ServerPort(""))
}
