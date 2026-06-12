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

package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/tools/cli/internal/services/config"
)

// redirectState points the state file to a temp directory for test isolation.
func redirectState(t *testing.T) {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
}

func TestReadActiveVersion_EmptyOnFirstRun(t *testing.T) {
	redirectState(t)
	assert.Empty(t, config.ReadActiveVersion())
}

func TestWriteAndReadActiveVersion(t *testing.T) {
	redirectState(t)
	require.NoError(t, config.WriteActiveVersion("1.2.3"))
	assert.Equal(t, "1.2.3", config.ReadActiveVersion())
}

func TestIsSetupComplete_FalseByDefault(t *testing.T) {
	redirectState(t)
	assert.False(t, config.IsSetupComplete("1.2.3"))
}

func TestMarkSetupComplete_RoundTrip(t *testing.T) {
	redirectState(t)
	require.NoError(t, config.MarkSetupComplete("1.2.3"))
	assert.True(t, config.IsSetupComplete("1.2.3"))
	assert.False(t, config.IsSetupComplete("9.9.9")) // unrelated version unchanged
}

func TestIsOnboardingDone_FalseByDefault(t *testing.T) {
	redirectState(t)
	assert.False(t, config.IsOnboardingDone("1.2.3"))
}

func TestMarkOnboardingDone_RoundTrip(t *testing.T) {
	redirectState(t)
	require.NoError(t, config.MarkOnboardingDone("1.2.3"))
	assert.True(t, config.IsOnboardingDone("1.2.3"))
}

func TestIsVersionSkipped_FalseByDefault(t *testing.T) {
	redirectState(t)
	assert.False(t, config.IsVersionSkipped("2.0.0"))
}

func TestMarkVersionSkipped_RoundTrip(t *testing.T) {
	redirectState(t)
	require.NoError(t, config.MarkVersionSkipped("2.0.0"))
	assert.True(t, config.IsVersionSkipped("2.0.0"))
	assert.False(t, config.IsVersionSkipped("3.0.0"))
}

func TestMarkVersionSkipped_Idempotent(t *testing.T) {
	redirectState(t)
	require.NoError(t, config.MarkVersionSkipped("2.0.0"))
	require.NoError(t, config.MarkVersionSkipped("2.0.0")) // second call must not error or duplicate
	assert.True(t, config.IsVersionSkipped("2.0.0"))
}

func TestStateDir_UnderHome(t *testing.T) {
	redirectState(t)
	dir := config.StateDir()
	home, _ := os.UserHomeDir()
	assert.Equal(t, filepath.Join(home, ".thunderid"), dir)
}
