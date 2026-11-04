// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package release_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/tools/cli/internal/services/release"
)

func TestPlatformAssetName_Format(t *testing.T) {
	name, err := release.PlatformAssetName("1.2.3")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(name, "thunderid-dp-1.2.3-"), "got %q", name)
	assert.True(t, strings.HasSuffix(name, ".zip"), "got %q", name)
}

func TestPlatformAssetName_ContainsPlatformAndArch(t *testing.T) {
	name, err := release.PlatformAssetName("0.5.0")
	require.NoError(t, err)

	// Must contain one of the known platform names.
	platforms := []string{"macos", "linux", "win"}
	found := false
	for _, p := range platforms {
		if strings.Contains(name, p) {
			found = true
			break
		}
	}
	assert.True(t, found, "expected a known platform in %q", name)

	// Must contain one of the known arch names.
	arches := []string{"x64", "arm64"}
	found = false
	for _, a := range arches {
		if strings.Contains(name, a) {
			found = true
			break
		}
	}
	assert.True(t, found, "expected a known arch in %q", name)
}

func TestSampleAssetName_Format(t *testing.T) {
	name, err := release.SampleAssetName("wayfinder", "1.2.3")
	require.NoError(t, err)
	assert.Equal(t, "sample-app-wayfinder-1.2.3.zip", name)
}

func TestSampleAssetName_DifferentSamples(t *testing.T) {
	a, errA := release.SampleAssetName("wayfinder", "1.0.0")
	b, errB := release.SampleAssetName("agentid", "1.0.0")
	require.NoError(t, errA)
	require.NoError(t, errB)
	assert.NotEqual(t, a, b, "different sample names should produce different asset names")
}
