// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executormeta

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// pickExecutorWith returns the name of a catalog entry whose metadata has a non-empty slice of the
// kind the caller cares about, so each test mutates a field that actually has an element to mutate.
func pickExecutorWith(t *testing.T, has func(name string) bool) string {
	t.Helper()
	for _, name := range Names() {
		if has(name) {
			return name
		}
	}
	t.Fatal("no catalog entry carries the field this test needs")
	return ""
}

func TestMetaForDoesNotAliasTheCatalog(t *testing.T) {
	name := pickExecutorWith(t, func(n string) bool {
		meta, _ := MetaFor(n)
		return len(meta.SupportedModes) > 0
	})

	meta, ok := MetaFor(name)
	require.True(t, ok)
	original := meta.SupportedModes[0]

	meta.SupportedModes[0] = "mutated-by-caller"

	again, ok := MetaFor(name)
	require.True(t, ok)
	assert.Equal(t, original, again.SupportedModes[0],
		"a caller writing through returned metadata must not change the catalog")
}

func TestGetExecutorMetaDoesNotAliasTheCatalog(t *testing.T) {
	name := pickExecutorWith(t, func(n string) bool {
		meta, _ := MetaFor(n)
		return len(meta.SupportedFlowTypes) > 0
	})

	reg, err := NewRegistry(nil)
	require.NoError(t, err)

	meta, err := reg.GetExecutorMeta(name)
	require.NoError(t, err)
	original := meta.SupportedFlowTypes[0]

	meta.SupportedFlowTypes[0] = "MUTATED"

	again, err := reg.GetExecutorMeta(name)
	require.NoError(t, err)
	assert.Equal(t, original, again.SupportedFlowTypes[0],
		"a caller writing through returned metadata must not change the catalog")
}

func TestReturnedMetadataDoesNotAliasNestedApplicableModes(t *testing.T) {
	name := pickExecutorWith(t, func(n string) bool {
		meta, _ := MetaFor(n)
		for _, p := range meta.SupportedProperties {
			if len(p.ApplicableModes) > 0 {
				return true
			}
		}
		return false
	})

	meta, ok := MetaFor(name)
	require.True(t, ok)

	var idx int
	for i, p := range meta.SupportedProperties {
		if len(p.ApplicableModes) > 0 {
			idx = i
			break
		}
	}
	original := meta.SupportedProperties[idx].ApplicableModes[0]

	meta.SupportedProperties[idx].ApplicableModes[0] = "mutated-by-caller"

	again, ok := MetaFor(name)
	require.True(t, ok)
	assert.Equal(t, original, again.SupportedProperties[idx].ApplicableModes[0],
		"the nested ApplicableModes slice must not alias the catalog either")
}

func TestMetaForReportsAnUnknownExecutor(t *testing.T) {
	meta, ok := MetaFor("NoSuchExecutor")

	assert.False(t, ok)
	assert.Equal(t, providers.ExecutorMeta{}, meta, "an unknown name yields the zero value")
}

// An empty list enables the whole catalog, which is what a deployment that names no executors gets.
func TestNewRegistryWithNoNamesEnablesEverything(t *testing.T) {
	reg, err := NewRegistry(nil)
	require.NoError(t, err)

	for _, name := range Names() {
		assert.True(t, reg.IsRegistered(name), "%s should be enabled", name)
	}
	assert.False(t, reg.IsRegistered("NoSuchExecutor"))
}

// A named list enables those and nothing else, so a deployment can run a subset.
func TestNewRegistryWithNamesEnablesOnlyThose(t *testing.T) {
	enabled := []string{ExecutorNameCredentialsAuth, ExecutorNameProvisioning}

	reg, err := NewRegistry(enabled)
	require.NoError(t, err)

	for _, name := range enabled {
		assert.True(t, reg.IsRegistered(name), "%s was named and should be enabled", name)
	}
	assert.False(t, reg.IsRegistered(ExecutorNameOTPExecutor),
		"an executor that was not named must not be enabled")

	meta, err := reg.GetExecutorMeta(ExecutorNameCredentialsAuth)
	require.NoError(t, err)
	assert.NotNil(t, meta)
}

// An unknown name fails at startup rather than becoming a flow that cannot run. This is the
// behavior the constructor exists to provide, so it is worth pinning.
func TestNewRegistryRejectsAnUnknownExecutor(t *testing.T) {
	reg, err := NewRegistry([]string{ExecutorNameCredentialsAuth, "NoSuchExecutor"})

	require.Error(t, err)
	assert.Nil(t, reg)
	assert.Contains(t, err.Error(), "NoSuchExecutor",
		"the error should name the executor that could not be resolved")
}

// Metadata for an executor the deployment did not enable is an error, not empty metadata, so a
// flow naming a disabled executor is rejected rather than silently validated against nothing.
func TestGetExecutorMetaRefusesAnExecutorThatIsNotEnabled(t *testing.T) {
	reg, err := NewRegistry([]string{ExecutorNameCredentialsAuth})
	require.NoError(t, err)

	meta, err := reg.GetExecutorMeta(ExecutorNameOTPExecutor)

	require.Error(t, err)
	assert.Nil(t, meta)
	assert.Contains(t, err.Error(), ExecutorNameOTPExecutor)
}

// Names is sorted and covers the catalog, which is what makes it usable as a stable listing.
func TestNamesIsSortedAndComplete(t *testing.T) {
	names := Names()

	assert.Len(t, names, len(catalog))
	assert.True(t, sort.StringsAreSorted(names), "Names must be sorted")
	assert.Contains(t, names, ExecutorNameCredentialsAuth)
}
