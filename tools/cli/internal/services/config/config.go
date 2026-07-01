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

// Package config manages persistent CLI state stored in ~/.thunderid/state.json.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

	"github.com/thunder-id/thunderid/tools/cli/internal/product"
)

// StateDir returns the hidden state directory under the user's home.
// Falls back to the OS temp directory when the home directory cannot be determined.
func StateDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.TempDir()
	}
	return filepath.Join(home, "."+product.Slug)
}

func statePath() string {
	return filepath.Join(StateDir(), "state.json")
}

type versionState struct {
	InstallPath    string `json:"installPath,omitempty"`
	SetupComplete  bool   `json:"setupComplete,omitempty"`
	OnboardingDone bool   `json:"onboardingDone,omitempty"`
}

type stateFile struct {
	Active          string                  `json:"active,omitempty"`
	Versions        map[string]versionState `json:"versions,omitempty"`
	SkippedUpgrades []string                `json:"skippedUpgrades,omitempty"`
}

func load() stateFile {
	data, err := os.ReadFile(statePath())
	if err != nil {
		return stateFile{}
	}
	var s stateFile
	if err := json.Unmarshal(data, &s); err != nil {
		return stateFile{}
	}
	return s
}

func save(s stateFile) error {
	if err := os.MkdirAll(StateDir(), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(statePath(), data, 0o644)
}

// ReadActiveVersion returns the active version, or "" if none is recorded.
func ReadActiveVersion() string {
	return load().Active
}

// WriteActiveVersion records version as the active version.
func WriteActiveVersion(version string) error {
	s := load()
	s.Active = version
	return save(s)
}

// IsSetupComplete reports whether setup has been completed for version.
func IsSetupComplete(version string) bool {
	s := load()
	return s.Versions[version].SetupComplete
}

// MarkSetupComplete records that setup has been completed for version.
func MarkSetupComplete(version string) error {
	s := load()
	if s.Versions == nil {
		s.Versions = make(map[string]versionState)
	}
	v := s.Versions[version]
	v.SetupComplete = true
	s.Versions[version] = v
	return save(s)
}

// ReadInstallPath returns the recorded absolute install path for version, or "" if none is stored.
func ReadInstallPath(version string) string {
	return load().Versions[version].InstallPath
}

// WriteInstallPath records the absolute install path for version.
func WriteInstallPath(version, installPath string) error {
	s := load()
	if s.Versions == nil {
		s.Versions = make(map[string]versionState)
	}
	v := s.Versions[version]
	v.InstallPath = installPath
	s.Versions[version] = v
	return save(s)
}

// IsOnboardingDone reports whether the first-run onboarding has been shown for version.
func IsOnboardingDone(version string) bool {
	s := load()
	return s.Versions[version].OnboardingDone
}

// MarkOnboardingDone records that onboarding has been completed for version.
func MarkOnboardingDone(version string) error {
	s := load()
	if s.Versions == nil {
		s.Versions = make(map[string]versionState)
	}
	v := s.Versions[version]
	v.OnboardingDone = true
	s.Versions[version] = v
	return save(s)
}

// IsVersionSkipped reports whether the user has chosen to skip upgrading to version.
func IsVersionSkipped(version string) bool {
	for _, v := range load().SkippedUpgrades {
		if v == version {
			return true
		}
	}
	return false
}

// ListInstalledVersions returns all versions that have completed setup and a recorded
// install path, sorted in ascending order. Pass exceptVersion to exclude one entry
// (e.g. the currently active version).
func ListInstalledVersions(exceptVersion string) []string {
	s := load()
	var versions []string
	for v, state := range s.Versions {
		if v == exceptVersion {
			continue
		}
		if state.SetupComplete && state.InstallPath != "" {
			versions = append(versions, v)
		}
	}
	sort.Strings(versions)
	return versions
}

// MarkVersionSkipped records that the user skipped upgrading to version.
func MarkVersionSkipped(version string) error {
	s := load()
	for _, v := range s.SkippedUpgrades {
		if v == version {
			return nil
		}
	}
	s.SkippedUpgrades = append(s.SkippedUpgrades, version)
	return save(s)
}
