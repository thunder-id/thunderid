// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzdetailmgt

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A capture names a type's processor credentials from the type it is handed, so the exporter must read the
// ID from it.
func TestTypeExporterReadsTheResourceID(t *testing.T) {
	exporter := newTypeExporter(nil)

	assert.Equal(t, "type-1", exporter.GetResourceID(&typeDeclarativeResource{ID: "type-1"}))
	assert.Empty(t, exporter.GetResourceID("not a type"))
	assert.Empty(t, exporter.GetResourceID(nil))
}
