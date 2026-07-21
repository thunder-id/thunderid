/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
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

package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/thunder-id/thunderid/internal/system/log/rollingfile"
)

const testServerHome = "/opt/thunderid"

func boolPtr(b bool) *bool        { return &b }
func intPtr(i int) *int           { return &i }
func floatPtr(f float64) *float64 { return &f }

func TestBuildOutputOptions_FileDisabled(t *testing.T) {
	home := testServerHome
	logCfg := LogConfig{}
	logCfg.Output.Console.Enabled = boolPtr(true)

	opts := logCfg.BuildOutputOptions(home)
	assert.True(t, opts.ConsoleEnabled)
	assert.False(t, opts.FileEnabled)
	assert.Empty(t, opts.File.Path)
}

func TestBuildOutputOptions_RelativePathResolvedUnderHome(t *testing.T) {
	home := testServerHome
	logCfg := LogConfig{}
	logCfg.Output.File.Enabled = boolPtr(true)
	logCfg.Output.File.Path = "logs"
	logCfg.Output.File.FileName = "thunderid.log"

	opts := logCfg.BuildOutputOptions(home)
	assert.True(t, opts.FileEnabled)
	assert.Equal(t, filepath.Join(home, "logs", "thunderid.log"), opts.File.Path)
}

func TestBuildOutputOptions_AbsolutePathUsedAsIs(t *testing.T) {
	home := testServerHome
	absDir := "/var/log/thunderid"
	logCfg := LogConfig{}
	logCfg.Output.File.Enabled = boolPtr(true)
	logCfg.Output.File.Path = absDir

	opts := logCfg.BuildOutputOptions(home)
	assert.Equal(t, filepath.Join(absDir, "thunderid.log"), opts.File.Path)
}

func TestBuildOutputOptions_DefaultsForEmptyPathAndName(t *testing.T) {
	home := testServerHome
	logCfg := LogConfig{}
	logCfg.Output.File.Enabled = boolPtr(true)

	opts := logCfg.BuildOutputOptions(home)
	assert.Equal(t, filepath.Join(home, "logs", "thunderid.log"), opts.File.Path)
}

// TestBuildOutputOptions_Rotation exercises both rotation triggers together:
// falling back to the package defaults when enabled without (or with a zero)
// value, honoring explicit values, and yielding zero when disabled.
func TestBuildOutputOptions_Rotation(t *testing.T) {
	home := testServerHome
	logCfg := LogConfig{}
	logCfg.Output.File.Enabled = boolPtr(true)
	r := &logCfg.Output.File.Rotation

	// Enabled without an explicit value falls back to the package defaults.
	r.Size.Enabled = boolPtr(true)
	r.Time.Enabled = boolPtr(true)
	opts := logCfg.BuildOutputOptions(home)
	assert.Equal(t, rollingfile.DefaultMaxSizeMB, opts.File.MaxSizeMB)
	assert.Equal(t, rollingfile.DefaultIntervalDays, opts.File.IntervalDays)

	// Explicit values are honored.
	r.Size.MaxSizeMB = floatPtr(0.02)
	r.Time.IntervalDays = intPtr(3)
	opts = logCfg.BuildOutputOptions(home)
	assert.Equal(t, 0.02, opts.File.MaxSizeMB)
	assert.Equal(t, 3, opts.File.IntervalDays)

	// An explicit zero while enabled falls back to the default.
	r.Size.MaxSizeMB = floatPtr(0)
	r.Time.IntervalDays = intPtr(0)
	opts = logCfg.BuildOutputOptions(home)
	assert.Equal(t, rollingfile.DefaultMaxSizeMB, opts.File.MaxSizeMB)
	assert.Equal(t, rollingfile.DefaultIntervalDays, opts.File.IntervalDays)

	// Disabled triggers yield zero (rotation off).
	r.Size.Enabled = boolPtr(false)
	r.Time.Enabled = boolPtr(false)
	opts = logCfg.BuildOutputOptions(home)
	assert.Equal(t, 0.0, opts.File.MaxSizeMB)
	assert.Equal(t, 0, opts.File.IntervalDays)
}

// TestBuildOutputOptions_RetentionOverrideToZero confirms a retention value can
// be overridden to 0 (which is meaningful: "keep all") now that the fields are
// presence-tracked pointers.
func TestBuildOutputOptions_RetentionOverrideToZero(t *testing.T) {
	logCfg := LogConfig{}
	logCfg.Output.File.Enabled = boolPtr(true)
	logCfg.Output.File.Rotation.MaxBackups = intPtr(0)

	opts := logCfg.BuildOutputOptions(testServerHome)
	assert.Equal(t, 0, opts.File.MaxBackups)
}
