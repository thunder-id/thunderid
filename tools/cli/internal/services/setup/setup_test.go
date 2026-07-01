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
