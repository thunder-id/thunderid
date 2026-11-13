// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package importer

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// A planned unit is found by its id, or by its handle when a resource names no id; outside a dry run, and for a
// unit the import does not create, nothing is.
func TestPlannedUnitsResolveWhatTheDryRunCreates(t *testing.T) {
	ctx := withPlannedUnits(context.Background(), newPlannedUnits())
	plannedUnitsOf(ctx).add("ou-1", "team")

	id, ok := plannedUnitsOf(ctx).resolve("ou-1", "")
	assert.True(t, ok)
	assert.Equal(t, "ou-1", id)
	id, ok = plannedUnitsOf(ctx).resolve("", "team")
	assert.True(t, ok)
	assert.Equal(t, "ou-1", id)

	_, ok = plannedUnitsOf(ctx).resolve("ou-2", "team")
	assert.False(t, ok, "an id names its unit even when the handle is planned")
	_, ok = plannedUnitsOf(ctx).resolve("", "")
	assert.False(t, ok)
	_, ok = plannedUnitsOf(context.Background()).resolve("ou-1", "")
	assert.False(t, ok, "a unit was planned outside a dry run")
}

// A project is planned once its document imports in the dry run; a failed one, and one outside a dry run, is not.
func TestPlannedUnitsRecordTheProjectsTheDryRunCreates(t *testing.T) {
	ctx := withPlannedUnits(context.Background(), newPlannedUnits())
	doc := parsedDocument{ResourceType: resourceTypeProject}
	plannedUnitsOf(ctx).record(doc, ImportItemOutcome{ResourceID: "project-1", Status: statusSuccess})
	plannedUnitsOf(ctx).record(doc, ImportItemOutcome{ResourceID: "project-2", Status: statusFailed})

	assert.True(t, plannedUnitsOf(ctx).hasProject("project-1"))
	assert.False(t, plannedUnitsOf(ctx).hasProject("project-2"))
	assert.False(t, plannedUnitsOf(ctx).hasProject(""))
	_, ok := plannedUnitsOf(ctx).resolve("project-1", "")
	assert.False(t, ok, "a project is not an organization unit")
	assert.False(t, plannedUnitsOf(context.Background()).hasProject("project-1"))
}
